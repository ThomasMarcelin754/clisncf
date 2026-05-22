package auth

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ChromeCredentials extracts email + password for monidentifiant.sncf
// from Chrome's Login Data SQLite. Reuses the AES decryption from
// chrome_cookies.go (same v10 format). Uses /usr/bin/sqlite3 (no CGO).
func ChromeCredentials(profile string) (email, password string, err error) {
	if profile == "" {
		profile = "Default"
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	dbPath := filepath.Join(home, "Library", "Application Support", "Google", "Chrome", profile, "Login Data")

	// Get Chrome AES key (same as chrome_cookies.go)
	keyPass, err := exec.Command("security", "find-generic-password",
		"-w", "-s", "Chrome Safe Storage", "-a", "Chrome").Output()
	if err != nil {
		return "", "", fmt.Errorf("keychain access denied: %w", err)
	}
	key := pbkdf2SHA1([]byte(strings.TrimSpace(string(keyPass))), []byte("saltysalt"), 1003, 16)

	// Copy DB to avoid Chrome lock
	tmpFile := dbPath + ".clisncf-pw-tmp"
	data, err := os.ReadFile(dbPath)
	if err != nil {
		return "", "", fmt.Errorf("read Login Data: %w", err)
	}
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return "", "", err
	}
	defer func() { _ = os.Remove(tmpFile) }()
	for _, ext := range []string{"-wal", "-shm"} {
		if d, err := os.ReadFile(dbPath + ext); err == nil {
			_ = os.WriteFile(tmpFile+ext, d, 0600)
			defer func(f string) { _ = os.Remove(f) }(tmpFile + ext)
		}
	}

	// Query email
	emailOut, err := exec.Command("sqlite3", tmpFile,
		`SELECT username_value FROM logins WHERE origin_url LIKE '%monidentifiant%' LIMIT 1`).Output()
	if err != nil {
		return "", "", fmt.Errorf("sqlite3 query: %w", err)
	}
	email = strings.TrimSpace(string(emailOut))
	if email == "" {
		return "", "", fmt.Errorf("no SNCF credentials in Chrome profile %q", profile)
	}

	// Query encrypted password as hex
	hexOut, err := exec.Command("sqlite3", tmpFile,
		`SELECT hex(password_value) FROM logins WHERE origin_url LIKE '%monidentifiant%' LIMIT 1`).Output()
	if err != nil {
		return "", "", fmt.Errorf("sqlite3 password query: %w", err)
	}
	encrypted, err := hex.DecodeString(strings.TrimSpace(string(hexOut)))
	if err != nil {
		return "", "", fmt.Errorf("hex decode: %w", err)
	}

	// Reuse decryptV10 from chrome_cookies.go
	password, err = decryptV10(encrypted, key)
	if err != nil {
		return "", "", fmt.Errorf("decrypt password: %w", err)
	}

	return email, password, nil
}
