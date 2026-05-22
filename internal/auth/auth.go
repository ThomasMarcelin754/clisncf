package auth

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/99designs/keyring"
)

const (
	serviceName = "sncf-cli"
	tokenKey    = "auth-token"
)

// Token represents stored OAuth credentials.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Email        string    `json:"email"`
}

// UserInfo represents the authenticated user's profile.
type UserInfo struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

func openKeyring() (keyring.Keyring, error) {
	return keyring.Open(keyring.Config{
		ServiceName: serviceName,
		// macOS Keychain by default, falls back to file-based on Linux
		AllowedBackends: []keyring.BackendType{
			keyring.KeychainBackend,
			keyring.FileBackend,
		},
		FileDir:          "~/.sncf-cli/keys",
		FilePasswordFunc: keyring.FixedStringPrompt("sncf-cli-keyring"),
	})
}

// StoreToken saves the auth token to the system keyring.
func StoreToken(token *Token) error {
	ring, err := openKeyring()
	if err != nil {
		return fmt.Errorf("keyring open: %w", err)
	}

	data, err := json.Marshal(token)
	if err != nil {
		return err
	}

	return ring.Set(keyring.Item{
		Key:  tokenKey,
		Data: data,
	})
}

// LoadToken retrieves the auth token from the system keyring.
func LoadToken() (*Token, error) {
	ring, err := openKeyring()
	if err != nil {
		return nil, fmt.Errorf("keyring open: %w", err)
	}

	item, err := ring.Get(tokenKey)
	if err != nil {
		return nil, fmt.Errorf("no stored token: %w", err)
	}

	var token Token
	if err := json.Unmarshal(item.Data, &token); err != nil {
		return nil, fmt.Errorf("corrupt token: %w", err)
	}

	if time.Now().After(token.ExpiresAt) {
		return nil, fmt.Errorf("token expired, please login again")
	}

	return &token, nil
}

// ClearToken removes the stored auth token.
func ClearToken() error {
	ring, err := openKeyring()
	if err != nil {
		return fmt.Errorf("keyring open: %w", err)
	}

	return ring.Remove(tokenKey)
}
