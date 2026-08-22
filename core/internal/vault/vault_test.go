package vault_test

import (
	"testing"

	"github.com/totalconnect/core/internal/vault"
)

func TestVaultUnlockLock(t *testing.T) {
	v := vault.NewVault()

	if v.IsUnlocked() {
		t.Fatal("new vault should be locked")
	}

	// Empty passphrase fails
	err := v.Unlock("")
	if err != vault.ErrPassphraseReq {
		t.Fatalf("expected ErrPassphraseReq, got %v", err)
	}

	// Initial unlock sets password
	pass := "my-secret-passphrase"
	err = v.Unlock(pass)
	if err != nil {
		t.Fatalf("failed to unlock vault: %v", err)
	}
	if !v.IsUnlocked() {
		t.Fatal("vault should be unlocked")
	}

	// Lock vault
	v.Lock()
	if v.IsUnlocked() {
		t.Fatal("vault should be locked after Lock()")
	}

	// Wrong password fails
	err = v.Unlock("wrong-password")
	if err != vault.ErrInvalidPassphrase {
		t.Fatalf("expected ErrInvalidPassphrase, got %v", err)
	}
	if v.IsUnlocked() {
		t.Fatal("vault should remain locked on invalid passphrase")
	}

	// Correct password succeeds
	err = v.Unlock(pass)
	if err != nil {
		t.Fatalf("unlock failed with correct password: %v", err)
	}
	if !v.IsUnlocked() {
		t.Fatal("vault should be unlocked")
	}
}

func TestVaultCredentials(t *testing.T) {
	v := vault.NewVault()

	// Locked operations fail
	_, err := v.AddCredential("db_user", "admin")
	if err != vault.ErrLocked {
		t.Fatalf("expected ErrLocked, got %v", err)
	}
	_, err = v.GetCredential("db_user")
	if err != vault.ErrLocked {
		t.Fatalf("expected ErrLocked, got %v", err)
	}

	// Unlock and add credential
	pass := "super-secure"
	if err := v.Unlock(pass); err != nil {
		t.Fatalf("unlock failed: %v", err)
	}

	id, err := v.AddCredential("db_user", "admin")
	if err != nil {
		t.Fatalf("failed to add credential: %v", err)
	}
	if id != "cred-db_user" {
		t.Fatalf("expected cred-db_user, got %s", id)
	}

	// Retrieve credential
	val, err := v.GetCredential("db_user")
	if err != nil {
		t.Fatalf("failed to get credential: %v", err)
	}
	if val != "admin" {
		t.Fatalf("expected 'admin', got '%s'", val)
	}

	// Non-existent key returns ErrNotFound
	_, err = v.GetCredential("nonexistent")
	if err != vault.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Lock and attempt retrieval
	v.Lock()
	_, err = v.GetCredential("db_user")
	if err != vault.ErrLocked {
		t.Fatalf("expected ErrLocked when locked, got %v", err)
	}

	// Re-unlock and retrieve
	if err := v.Unlock(pass); err != nil {
		t.Fatalf("re-unlock failed: %v", err)
	}
	val, err = v.GetCredential("db_user")
	if err != nil {
		t.Fatalf("failed to get credential after re-unlock: %v", err)
	}
	if val != "admin" {
		t.Fatalf("expected 'admin', got '%s'", val)
	}
}

func TestVaultSSHKeys(t *testing.T) {
	v := vault.NewVault()
	pass := "ssh-master-key"

	// Locked operations fail
	err := v.AddSSHKey("id_ed25519", "pubkey", "privkey")
	if err != vault.ErrLocked {
		t.Fatalf("expected ErrLocked, got %v", err)
	}
	_, _, err = v.GetSSHKey("id_ed25519")
	if err != vault.ErrLocked {
		t.Fatalf("expected ErrLocked, got %v", err)
	}

	if err := v.Unlock(pass); err != nil {
		t.Fatalf("unlock failed: %v", err)
	}

	// Default fallback if key not explicitly added
	pub, priv, err := v.GetSSHKey("default")
	if err != nil {
		t.Fatalf("unexpected error on default key: %v", err)
	}
	if pub == "" || priv == "" {
		t.Fatal("expected non-empty default ssh keys")
	}

	// Store custom SSH key
	pubInput := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI..."
	privInput := "-----BEGIN OPENSSH PRIVATE KEY-----\n..."
	err = v.AddSSHKey("prod_server", pubInput, privInput)
	if err != nil {
		t.Fatalf("failed to add ssh key: %v", err)
	}

	pub, priv, err = v.GetSSHKey("prod_server")
	if err != nil {
		t.Fatalf("failed to get ssh key: %v", err)
	}
	if pub != pubInput || priv != privInput {
		t.Fatalf("decrypted ssh keys do not match input: got pub=%s, priv=%s", pub, priv)
	}
}
