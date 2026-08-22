package tui

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Profile struct {
	Name     string `json:"name"`
	Backend  string `json:"backend"`  // "sftp", "s3", "ftp", "drive", etc.
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"` // AES-GCM encrypted with Vault passphrase
	Path     string `json:"path"`     // default remote path on connect
}

func getProfilesPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "totalconnect", "profiles.json"), nil
}

// LoadProfiles loads profiles from ~/.config/totalconnect/profiles.json
func LoadProfiles() ([]Profile, error) {
	path, err := getProfilesPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var profiles []Profile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, err
	}

	return profiles, nil
}

// SaveProfiles saves profiles to ~/.config/totalconnect/profiles.json
func SaveProfiles(profiles []Profile, passphrase string) error {
	path, err := getProfilesPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// ExportEncryptPassword exports encryptPassword for internal package tests.
func ExportEncryptPassword(plain, passphrase string) (string, error) {
	return encryptPassword(plain, passphrase)
}

// ExportDecryptPassword exports decryptPassword for internal package tests.
func ExportDecryptPassword(cipherHex, passphrase string) (string, error) {
	return decryptPassword(cipherHex, passphrase)
}

func encryptPassword(plain, passphrase string) (string, error) {
	if plain == "" {
		return "", nil
	}
	key := sha256.Sum256([]byte(passphrase))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return hex.EncodeToString(ciphertext), nil
}

func decryptPassword(cipherHex, passphrase string) (string, error) {
	if cipherHex == "" {
		return "", nil
	}
	ciphertext, err := hex.DecodeString(cipherHex)
	if err != nil {
		return "", err
	}

	key := sha256.Sum256([]byte(passphrase))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
