package crypto

import (
	"encoding/base64"
	"fmt"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "basepod"
	keyringAccount = "aes-key"
)

// LoadOrCreateKey returns the AES key from macOS Keychain, generating one on
// first use.
func LoadOrCreateKey() ([]byte, error) {
	enc, err := keyring.Get(keyringService, keyringAccount)
	if err == nil && enc != "" {
		raw, err := base64.StdEncoding.DecodeString(enc)
		if err != nil {
			return nil, fmt.Errorf("decode key: %w", err)
		}
		if len(raw) == KeySize {
			return raw, nil
		}
	}
	key, err := NewKey()
	if err != nil {
		return nil, err
	}
	if err := keyring.Set(keyringService, keyringAccount, base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, fmt.Errorf("save key to keychain: %w", err)
	}
	return key, nil
}
