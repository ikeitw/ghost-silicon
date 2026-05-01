// pkg/storage/encrypted_store.go
// Package storage — encrypted key-value store.
// Wraps a JSON file with AES-256-GCM encryption so sensitive data
// (tokens, saved credentials) is protected at rest.
// The encryption key is derived from DPAPI on Windows or the system keyring
// on Linux and is never written to disk in plaintext.
package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// EncryptedStore is a thread-safe encrypted key-value store backed by a file.
type EncryptedStore struct {
	mu   sync.RWMutex
	path string
	key  []byte // 32-byte AES-256 key
	data map[string]string
}

// NewEncryptedStore creates or opens an EncryptedStore at path using key.
// key must be exactly 32 bytes (AES-256).
func NewEncryptedStore(path string, key []byte) (*EncryptedStore, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("storage/encrypted: key must be 32 bytes, got %d", len(key))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("storage/encrypted: mkdir: %w", err)
	}
	s := &EncryptedStore{path: path, key: key, data: make(map[string]string)}
	if err := s.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("storage/encrypted: load: %w", err)
	}
	return s, nil
}

// Set stores value under key and flushes to disk.
func (s *EncryptedStore) Set(k, v string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[k] = v
	return s.flush()
}

// Get retrieves the value for k. Returns ("", false) if not found.
func (s *EncryptedStore) Get(k string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[k]
	return v, ok
}

// Delete removes k and flushes to disk.
func (s *EncryptedStore) Delete(k string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, k)
	return s.flush()
}

// load decrypts and parses the store file.
func (s *EncryptedStore) load() error {
	ciphertext, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	plaintext, err := aesgcmDecrypt(s.key, ciphertext)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}
	return json.Unmarshal(plaintext, &s.data)
}

// flush serialises and encrypts the store to disk atomically.
func (s *EncryptedStore) flush() error {
	plaintext, err := json.Marshal(s.data)
	if err != nil {
		return fmt.Errorf("storage/encrypted: marshal: %w", err)
	}
	ciphertext, err := aesgcmEncrypt(s.key, plaintext)
	if err != nil {
		return fmt.Errorf("storage/encrypted: encrypt: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, ciphertext, 0o600); err != nil {
		return fmt.Errorf("storage/encrypted: write: %w", err)
	}
	return os.Rename(tmp, s.path)
}

// aesgcmEncrypt encrypts plaintext with AES-256-GCM.
// Output format: [12-byte nonce][ciphertext+tag].
func aesgcmEncrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
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

// aesgcmDecrypt decrypts ciphertext produced by aesgcmEncrypt.
func aesgcmDecrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return gcm.Open(nil, ciphertext[:ns], ciphertext[ns:], nil)
}
