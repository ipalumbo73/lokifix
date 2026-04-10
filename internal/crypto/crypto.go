package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// Cipher provides AES-256-GCM encryption and decryption.
type Cipher struct {
	gcm cipher.AEAD
}

// DeriveKey derives a 32-byte AES key from a shared secret using HKDF-SHA256.
func DeriveKey(secret string) ([]byte, error) {
	if secret == "" {
		return nil, fmt.Errorf("empty secret for key derivation")
	}

	salt := []byte("lokifix-e2e-v1")
	info := []byte("aes-256-gcm")

	hkdfReader := hkdf.New(sha256.New, []byte(secret), salt, info)

	key := make([]byte, 32) // AES-256
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return nil, fmt.Errorf("hkdf derive: %w", err)
	}
	return key, nil
}

// NewCipher creates a new AES-256-GCM cipher from a raw 32-byte key.
func NewCipher(key []byte) (*Cipher, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm new: %w", err)
	}

	return &Cipher{gcm: gcm}, nil
}

// NewCipherFromSecret derives a key from a shared secret and creates a cipher.
func NewCipherFromSecret(secret string) (*Cipher, error) {
	key, err := DeriveKey(secret)
	if err != nil {
		return nil, err
	}
	return NewCipher(key)
}

// Encrypt encrypts plaintext using AES-256-GCM.
// Returns: nonce (12 bytes) || ciphertext (with GCM tag appended).
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := c.gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts data encrypted by Encrypt.
// Expects: nonce (12 bytes) || ciphertext (with GCM tag).
func (c *Cipher) Decrypt(data []byte) ([]byte, error) {
	nonceSize := c.gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]

	plaintext, err := c.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}
