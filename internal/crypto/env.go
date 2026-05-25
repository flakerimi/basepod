package crypto

import "sync"

// EnvCipher wraps Seal/Open with a single cached key.
type EnvCipher struct {
	mu  sync.Mutex
	key []byte
}

func NewEnvCipher(key []byte) *EnvCipher {
	return &EnvCipher{key: key}
}

func (e *EnvCipher) Encrypt(plain string) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Seal(e.key, []byte(plain))
}

func (e *EnvCipher) Decrypt(ct []byte) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	pt, err := Open(e.key, ct)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
