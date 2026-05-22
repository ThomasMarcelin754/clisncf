package auth

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"encoding/base64"
	"encoding/json"
)

// cmsAccount is the decoded __Secure-cms-account marker cookie.
type cmsAccount struct {
	IsLoggedIn bool   `json:"isLoggedIn"`
	FirstName  string `json:"firstName"`
	Initials   string `json:"initials"`
}

func decodeCMS(cookieHeader string) (cmsAccount, bool) {
	for _, kv := range strings.Split(cookieHeader, "; ") {
		name, val, ok := strings.Cut(kv, "=")
		if !ok || name != "__Secure-cms-account" {
			continue
		}
		if m := len(val) % 4; m != 0 {
			val += strings.Repeat("=", 4-m)
		}
		raw, err := base64.StdEncoding.DecodeString(val)
		if err != nil {
			return cmsAccount{}, false
		}
		var cms cmsAccount
		if json.Unmarshal(raw, &cms) != nil {
			return cmsAccount{}, false
		}
		return cms, true
	}
	return cmsAccount{}, false
}

// AccountName returns the logged-in holder's first name (from the cms cookie).
func AccountName(cookieHeader string) string {
	cms, _ := decodeCMS(cookieHeader)
	return cms.FirstName
}

// Session states (verified against a real account). The COLD vs ABSENT
// distinction matters: a logged-in user whose SNCF
// tab has been idle >~30 min has COLD (Chrome evicted the expired access token
// but kept refresh+cms) — that is NOT "not signed in", and there is NO headless
// rescue (the out-of-window refresh token is dead, measured 2026-05-19).
const (
	SessionHot    = "HOT"    // access token + cms isLoggedIn → account calls work now
	SessionCold   = "COLD"   // cms isLoggedIn, no access token → re-open sncf-connect.com
	SessionAbsent = "ABSENT" // no SNCF session in this Chrome profile
)

// SessionState classifies the extracted jar and returns the account holder
// first name when known.
func SessionState(cookieHeader string) (state, holder string) {
	cms, ok := decodeCMS(cookieHeader)
	switch {
	case ok && cms.IsLoggedIn && strings.Contains(cookieHeader, "__Host-access-account-token="):
		return SessionHot, cms.FirstName
	case ok && cms.IsLoggedIn:
		return SessionCold, cms.FirstName
	default:
		return SessionAbsent, ""
	}
}

// LoggedIn reports whether the jar can make account calls right now (HOT).
// Passing Datadome is NOT the same as a usable session; this guard turns the
// BFF's silent anonymous-empty response into a clear message.
func LoggedIn(cookieHeader string) bool {
	s, _ := SessionState(cookieHeader)
	return s == SessionHot
}

// SNCF Connect lives on www.sncf-connect.com; the whole jar must be replayed
// (don't cherry-pick — partial replay causes session mismatch).
const cookieHostLike = "%sncf-connect.com%"

// ChromeCookieJar reads the SNCF Connect cookies from the user's Chrome
// profile (default "Default"), decrypts them (macOS Chrome "v10" scheme:
// AES-128-CBC, key = PBKDF2-HMAC-SHA1(Keychain "Chrome Safe Storage",
// "saltysalt", 1003, 16)) and returns them as a single "name=value; ..."
// Cookie header.
//
// The account must be "hot" (user logged into sncf-connect.com in that
// Chrome profile). Reading the Keychain triggers a one-time allow prompt the
// account owner must approve.
//
// NOTE: a valid jar is necessary but not sufficient — the HTTP client that
// replays it MUST present a Chrome TLS/JA3 fingerprint (uTLS), otherwise
// Datadome answers 403 + a captcha redirect.
func ChromeCookieJar(profile string) (string, error) {
	if profile == "" {
		profile = "Default"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dbPath := filepath.Join(home, "Library", "Application Support",
		"Google", "Chrome", profile, "Cookies")
	if _, err := os.Stat(dbPath); err != nil {
		return "", fmt.Errorf("Chrome cookie DB not found for profile %q: %w", profile, err)
	}

	// Chrome locks the live DB — work on a copy (+ WAL/SHM if present).
	tmpDir, err := os.MkdirTemp("", "sncf-cookies-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	tmpDB := filepath.Join(tmpDir, "Cookies")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if data, err := os.ReadFile(dbPath + suffix); err == nil {
			_ = os.WriteFile(tmpDB+suffix, data, 0o600)
		}
	}

	key, err := chromeSafeStorageKey()
	if err != nil {
		return "", err
	}

	// macOS ships /usr/bin/sqlite3 — avoids a CGO sqlite driver dependency.
	out, err := exec.Command("sqlite3", tmpDB,
		"SELECT name || '\t' || hex(encrypted_value) FROM cookies WHERE host_key LIKE '"+
			cookieHostLike+"';").Output()
	if err != nil {
		return "", fmt.Errorf("sqlite3 read failed: %w", err)
	}

	var pairs []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name, hexVal, ok := strings.Cut(line, "\t")
		if !ok || hexVal == "" {
			continue
		}
		enc, err := hex.DecodeString(hexVal)
		if err != nil {
			continue
		}
		val, err := decryptV10(enc, key)
		if err != nil {
			continue
		}
		pairs = append(pairs, name+"="+val)
	}
	if len(pairs) == 0 {
		return "", fmt.Errorf("no sncf-connect cookies in Chrome profile %q (logged in there?)", profile)
	}
	return strings.Join(pairs, "; "), nil
}

// chromeSafeStorageKey derives the 16-byte AES key from the Keychain secret.
func chromeSafeStorageKey() ([]byte, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-w", "-s", "Chrome Safe Storage", "-a", "Chrome").Output()
	if err != nil {
		return nil, fmt.Errorf("read 'Chrome Safe Storage' from Keychain: %w", err)
	}
	pw := bytes.TrimRight(out, "\n")
	return pbkdf2SHA1(pw, []byte("saltysalt"), 1003, 16), nil
}

// decryptV10 decrypts a Chrome macOS "v10" cookie value.
func decryptV10(enc, key []byte) (string, error) {
	if len(enc) < 3 || string(enc[:3]) != "v10" {
		return "", fmt.Errorf("not a v10 value")
	}
	ct := enc[3:]
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	if len(ct) == 0 || len(ct)%aes.BlockSize != 0 {
		return "", fmt.Errorf("bad ciphertext length")
	}
	iv := bytes.Repeat([]byte{' '}, aes.BlockSize)
	pt := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(pt, ct)

	// strip PKCS7
	if n := int(pt[len(pt)-1]); n > 0 && n <= aes.BlockSize && n <= len(pt) {
		pt = pt[:len(pt)-n]
	}
	// Chrome >= v130 prepends 32-byte SHA256(host) to the plaintext.
	if !isUTF8Printable(pt) && len(pt) > 32 {
		pt = pt[32:]
	}
	return string(pt), nil
}

func isUTF8Printable(b []byte) bool {
	for _, c := range b {
		if c < 0x09 || (c > 0x0d && c < 0x20) {
			return false
		}
	}
	return true
}

// pbkdf2SHA1 is RFC 2898 PBKDF2 with HMAC-SHA1 (only variant Chrome uses),
// inlined to avoid an x/crypto dependency.
func pbkdf2SHA1(password, salt []byte, iter, keyLen int) []byte {
	prf := func(b []byte) []byte {
		m := hmac.New(sha1.New, password)
		m.Write(b)
		return m.Sum(nil)
	}
	hashLen := sha1.Size
	numBlocks := (keyLen + hashLen - 1) / hashLen
	var dk []byte
	for block := 1; block <= numBlocks; block++ {
		u := prf(append(salt, byte(block>>24), byte(block>>16), byte(block>>8), byte(block)))
		t := make([]byte, len(u))
		copy(t, u)
		for n := 2; n <= iter; n++ {
			u = prf(u)
			for i := range t {
				t[i] ^= u[i]
			}
		}
		dk = append(dk, t...)
	}
	return dk[:keyLen]
}
