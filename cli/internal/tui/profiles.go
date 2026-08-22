package tui

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// Profile represents a remote connection configuration.
type Profile struct {
	Name     string `json:"name"`
	Backend  string `json:"backend"`  // "sftp", "s3", "ftp", "drive", etc.
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"` // stored encrypted via Vault passphrase (AES-GCM)
	Path     string `json:"path"`     // default remote path on connect
}

func getProfilesPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "totalconnect")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "profiles.json"), nil
}

// LoadProfiles reads connection profiles from ~/.config/totalconnect/profiles.json
func LoadProfiles() ([]Profile, error) {
	path, err := getProfilesPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Profile{}, nil
		}
		return nil, err
	}

	var profiles []Profile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, err
	}
	return profiles, nil
}

// SaveProfiles writes connection profiles to ~/.config/totalconnect/profiles.json,
// encrypting the Password field of each profile with AES-GCM using the derived key.
func SaveProfiles(profiles []Profile, passphrase string) error {
	path, err := getProfilesPath()
	if err != nil {
		return err
	}

	var key []byte
	if passphrase != "" {
		hash := sha256.Sum256([]byte(passphrase))
		key = hash[:]
	}

	toSave := make([]Profile, len(profiles))
	for i, p := range profiles {
		toSave[i] = p
		if passphrase != "" && p.Password != "" {
			enc, err := encryptPassword(p.Password, key)
			if err == nil {
				toSave[i].Password = enc
			}
		}
	}

	data, err := json.MarshalIndent(toSave, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func encryptPassword(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
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

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptPassword decrypts a base64 encoded profile password using passphrase.
func DecryptPassword(encodedCiphertext string, passphrase string) (string, error) {
	if passphrase == "" || encodedCiphertext == "" {
		return encodedCiphertext, nil
	}

	hash := sha256.Sum256([]byte(passphrase))
	key := hash[:]

	data, err := base64.StdEncoding.DecodeString(encodedCiphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
