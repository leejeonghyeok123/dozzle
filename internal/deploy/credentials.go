package deploy

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Credential struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

type CredentialStore struct {
	path string
	key  []byte
}

func NewCredentialStore(path, secret string) *CredentialStore {
	sum := sha256.Sum256([]byte(secret))
	return &CredentialStore{path: path, key: sum[:]}
}

func (s *CredentialStore) Save(c Credential) error {
	all, _ := s.readAll()
	all[c.Host] = c
	return s.writeAll(all)
}

func (s *CredentialStore) Get(host string) (Credential, bool) {
	all, err := s.readAll()
	if err != nil {
		return Credential{}, false
	}
	c, ok := all[host]
	return c, ok
}

func (s *CredentialStore) readAll() (map[string]Credential, error) {
	out := map[string]Credential{}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return out, nil
	}
	plain, err := s.decrypt(data)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(plain, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *CredentialStore) writeAll(values map[string]Credential) error {
	raw, err := json.Marshal(values)
	if err != nil {
		return err
	}
	enc, err := s.encrypt(raw)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	return os.WriteFile(s.path, enc, 0600)
}

func (s *CredentialStore) encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
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
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	out := base64.StdEncoding.EncodeToString(ciphertext)
	return []byte(out), nil
}

func (s *CredentialStore) decrypt(data []byte) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("invalid encrypted payload")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

