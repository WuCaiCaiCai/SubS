package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Box encrypts small secrets (subscription URLs, tokens) at rest with AES-256-GCM.
// The key lives in <data-dir>/master.key (0600), never in the database, so a
// stolen .db file alone cannot reveal stored credentials.
type Box struct {
	key  []byte
	aead cipher.AEAD
}

const masterKeyFile = "master.key"

// LoadOrCreateBox reads the master key from dataDir, generating it on first run.
func LoadOrCreateBox(dataDir string) (*Box, error) {
	path := filepath.Join(dataDir, masterKeyFile)
	key, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("读取主密钥失败: %w", err)
		}
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, key, 0o600); err != nil {
			return nil, fmt.Errorf("写入主密钥失败: %w", err)
		}
		return newBox(key)
	}
	if len(key) != 32 {
		return nil, errors.New("主密钥长度异常")
	}
	return newBox(key)
}

func newBox(key []byte) (*Box, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{key: append([]byte(nil), key...), aead: aead}, nil
}

// Encrypt returns base64(nonce || ciphertext).
func (b *Box) Encrypt(plain string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := b.aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.RawStdEncoding.EncodeToString(out), nil
}

// Decrypt reverses Encrypt.
func (b *Box) Decrypt(enc string) (string, error) {
	raw, err := base64.RawStdEncoding.DecodeString(enc)
	if err != nil {
		return "", errors.New("密文格式错误")
	}
	ns := b.aead.NonceSize()
	if len(raw) < ns {
		return "", errors.New("密文长度异常")
	}
	plain, err := b.aead.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", errors.New("解密失败")
	}
	return string(plain), nil
}

// Subkey derives a purpose-bound 32-byte key so one master key can serve
// multiple independent uses (e.g. backup encryption).
func (b *Box) Subkey(purpose string) []byte {
	mac := hmac.New(sha256.New, b.key)
	mac.Write([]byte(purpose))
	return mac.Sum(nil)
}
