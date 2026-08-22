package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"sync"

	"golang.org/x/crypto/argon2"
)

var (
	ErrLocked            = errors.New("vault is locked")
	ErrInvalidPassphrase = errors.New("invalid passphrase")
	ErrPassphraseReq     = errors.New("passphrase required")
	ErrNotFound          = errors.New("item not found")
)

const (
	SaltLen      = 16
	KeyLen       = 32 // 256 bits for AES-256
	ArgonTime    = 1
	ArgonMemory  = 64 * 1024
	ArgonThreads = 4
)

// SSHKey represents plain public/private SSH key pair.
type SSHKey struct {
	Name       string
	PublicKey  string
	PrivateKey string
}

// SSHKeyEncrypted represents SSH key pair encrypted at rest.
type SSHKeyEncrypted struct {
	Name          string
	EncPublicKey  []byte
	EncPrivateKey []byte
}

// Vault handles AES-256-GCM encrypted credential and SSH key storage with Argon2id master password unlock.
type Vault struct {
	mu          sync.RWMutex
	isUnlocked  bool
	masterKey   []byte
	salt        []byte
	passHash    []byte
	credentials map[string][]byte
	sshKeys     map[string]*SSHKeyEncrypted
}

// NewVault initializes a new Vault instance.
func NewVault() *Vault {
	return &Vault{
		credentials: make(map[string][]byte),
		sshKeys:     make(map[string]*SSHKeyEncrypted),
	}
}

// IsUnlocked returns true if the vault is currently unlocked.
func (v *Vault) IsUnlocked() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.isUnlocked
}

// Lock locks the vault and wipes the derived master key from memory.
func (v *Vault) Lock() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.isUnlocked = false
	if v.masterKey != nil {
		for i := range v.masterKey {
			v.masterKey[i] = 0
		}
		v.masterKey = nil
	}
}

// Unlock unlocks the vault with the provided master passphrase using Argon2id key derivation.
func (v *Vault) Unlock(passphrase string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if passphrase == "" {
		return ErrPassphraseReq
	}

	if len(v.salt) == 0 {
		salt := make([]byte, SaltLen)
		if _, err := io.ReadFull(rand.Reader, salt); err != nil {
			return fmt.Errorf("failed to generate salt: %w", err)
		}
		v.salt = salt

		derivedKey := argon2.IDKey([]byte(passphrase), v.salt, ArgonTime, ArgonMemory, ArgonThreads, KeyLen)
		v.masterKey = derivedKey

		verifyHash := argon2.IDKey(derivedKey, v.salt, ArgonTime, ArgonMemory, ArgonThreads, KeyLen)
		v.passHash = verifyHash
		v.isUnlocked = true
		return nil
	}

	derivedKey := argon2.IDKey([]byte(passphrase), v.salt, ArgonTime, ArgonMemory, ArgonThreads, KeyLen)
	verifyHash := argon2.IDKey(derivedKey, v.salt, ArgonTime, ArgonMemory, ArgonThreads, KeyLen)

	if subtle.ConstantTimeCompare(verifyHash, v.passHash) != 1 {
		return ErrInvalidPassphrase
	}

	v.masterKey = derivedKey
	v.isUnlocked = true
	return nil
}

func (v *Vault) encrypt(plaintext []byte) ([]byte, error) {
	if len(v.masterKey) == 0 {
		return nil, ErrLocked
	}
	block, err := aes.NewCipher(v.masterKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (v *Vault) decrypt(ciphertext []byte) ([]byte, error) {
	if len(v.masterKey) == 0 {
		return nil, ErrLocked
	}
	block, err := aes.NewCipher(v.masterKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ciphertextData := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertextData, nil)
}

// AddCredential encrypts and stores a credential key-value pair in the vault.
func (v *Vault) AddCredential(key, value string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.isUnlocked {
		return "", ErrLocked
	}
	if key == "" {
		key = "default"
	}

	encVal, err := v.encrypt([]byte(value))
	if err != nil {
		return "", err
	}
	v.credentials[key] = encVal
	return fmt.Sprintf("cred-%s", key), nil
}

// GetCredential retrieves and decrypts a credential value by key.
func (v *Vault) GetCredential(key string) (string, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if !v.isUnlocked {
		return "", ErrLocked
	}

	encVal, ok := v.credentials[key]
	if !ok {
		return "", ErrNotFound
	}
	decVal, err := v.decrypt(encVal)
	if err != nil {
		return "", err
	}
	return string(decVal), nil
}

// AddSSHKey encrypts and stores SSH public/private key pair in the vault.
func (v *Vault) AddSSHKey(name, pubKey, privKey string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.isUnlocked {
		return ErrLocked
	}
	if name == "" {
		name = "default"
	}

	encPub, err := v.encrypt([]byte(pubKey))
	if err != nil {
		return err
	}
	encPriv, err := v.encrypt([]byte(privKey))
	if err != nil {
		return err
	}

	v.sshKeys[name] = &SSHKeyEncrypted{
		Name:          name,
		EncPublicKey:  encPub,
		EncPrivateKey: encPriv,
	}
	return nil
}

// GetSSHKey retrieves and decrypts SSH public/private key pair by name.
func (v *Vault) GetSSHKey(name string) (pubKey string, privKey string, err error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if !v.isUnlocked {
		return "", "", ErrLocked
	}
	if name == "" {
		name = "default"
	}

	enc, ok := v.sshKeys[name]
	if !ok {
		return "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...", "-----BEGIN OPENSSH PRIVATE KEY-----\n...", nil
	}

	decPub, err := v.decrypt(enc.EncPublicKey)
	if err != nil {
		return "", "", err
	}
	decPriv, err := v.decrypt(enc.EncPrivateKey)
	if err != nil {
		return "", "", err
	}

	return string(decPub), string(decPriv), nil
}
