// pkg/security/dpapi_windows.go
//go:build windows

// Package security — Windows DPAPI secret store.
// Uses the Windows Data Protection API to encrypt/decrypt secrets so they
// are bound to the current Windows user account and machine.
package security

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// DPAPIStore is a SecretStore backed by the Windows DPAPI.
// Each secret is stored as a separate encrypted file under dir.
type DPAPIStore struct {
	dir string
}

// NewDPAPIStore creates a DPAPIStore that persists secrets in dir.
func NewDPAPIStore(dir string) (*DPAPIStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("security/dpapi: mkdir %q: %w", dir, err)
	}
	return &DPAPIStore{dir: dir}, nil
}

// Set encrypts value with DPAPI and writes it to a file named key.
func (s *DPAPIStore) Set(key, value string) error {
	ciphertext, err := dpapEncrypt([]byte(value))
	if err != nil {
		return fmt.Errorf("security/dpapi: encrypt %q: %w", key, err)
	}
	path := filepath.Join(s.dir, sanitiseKey(key))
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, ciphertext, 0o600); err != nil {
		return fmt.Errorf("security/dpapi: write: %w", err)
	}
	return os.Rename(tmp, path)
}

// Get reads and decrypts the secret for key.
func (s *DPAPIStore) Get(key string) (string, error) {
	path := filepath.Join(s.dir, sanitiseKey(key))
	ciphertext, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrSecretNotFound
		}
		return "", fmt.Errorf("security/dpapi: read %q: %w", key, err)
	}
	plaintext, err := dpapDecrypt(ciphertext)
	if err != nil {
		return "", fmt.Errorf("security/dpapi: decrypt %q: %w", key, err)
	}
	return string(plaintext), nil
}

// Delete removes the secret file for key.
func (s *DPAPIStore) Delete(key string) error {
	err := os.Remove(filepath.Join(s.dir, sanitiseKey(key)))
	if os.IsNotExist(err) {
		return ErrSecretNotFound
	}
	return err
}

// ── DPAPI syscall wrappers ───────────────────────────────────────────────────

var (
	modCrypt32             = windows.NewLazySystemDLL("crypt32.dll")
	procCryptProtectData   = modCrypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = modCrypt32.NewProc("CryptUnprotectData")
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func newBlob(data []byte) *dataBlob {
	if len(data) == 0 {
		return &dataBlob{}
	}
	return &dataBlob{cbData: uint32(len(data)), pbData: &data[0]}
}

func (b *dataBlob) bytes() []byte {
	if b.cbData == 0 {
		return nil
	}
	return (*[1 << 28]byte)(unsafe.Pointer(b.pbData))[:b.cbData:b.cbData]
}

func dpapEncrypt(plaintext []byte) ([]byte, error) {
	in := newBlob(plaintext)
	var out dataBlob
	ret, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(in)),
		0, 0, 0, 0,
		0, // CRYPTPROTECT_LOCAL_MACHINE not set → user-scope
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("CryptProtectData: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.pbData))) //nolint:errcheck
	result := make([]byte, out.cbData)
	copy(result, out.bytes())
	return result, nil
}

func dpapDecrypt(ciphertext []byte) ([]byte, error) {
	in := newBlob(ciphertext)
	var out dataBlob
	ret, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(in)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("CryptUnprotectData: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.pbData))) //nolint:errcheck
	result := make([]byte, out.cbData)
	copy(result, out.bytes())
	return result, nil
}

// sanitiseKey makes a secret key safe to use as a file name.
func sanitiseKey(k string) string {
	safe := make([]byte, 0, len(k))
	for i := 0; i < len(k); i++ {
		c := k[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' {
			safe = append(safe, c)
		} else {
			safe = append(safe, '_')
		}
	}
	return string(safe)
}
