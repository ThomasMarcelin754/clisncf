package auth

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

// TestSessionState is the deterministic backtest of the HOT/COLD/ABSENT state
// machine. COLD's input mirrors a real measured inactive jar (cms isLoggedIn
// but Chrome evicted __Host-access-account-token).
func TestSessionState(t *testing.T) {
	cms := func(j string) string {
		return "__Secure-cms-account=" + base64.StdEncoding.EncodeToString([]byte(j))
	}
	in := cms(`{"isLoggedIn":true,"firstName":"Thomas"}`)
	out := cms(`{"isLoggedIn":false}`)
	const acc = "__Host-access-account-token=eyJ.payload.sig"
	const ref = "__Secure-refresh-account-token=v1.xyz"

	cases := []struct {
		name, jar, want, holder string
	}{
		{"HOT", acc + "; " + in + "; " + ref, SessionHot, "Thomas"},
		{"COLD measured inactive", in + "; " + ref + "; datadome=z", SessionCold, "Thomas"},
		{"COLD no refresh", in, SessionCold, "Thomas"},
		{"ABSENT not-logged", out + "; " + ref, SessionAbsent, ""},
		{"ABSENT no cms", acc + "; datadome=z", SessionAbsent, ""},
		{"ABSENT empty", "", SessionAbsent, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, holder := SessionState(c.jar)
			if got != c.want || holder != c.holder {
				t.Fatalf("SessionState = (%q,%q), want (%q,%q)", got, holder, c.want, c.holder)
			}
			// LoggedIn must stay exactly "state == HOT".
			if LoggedIn(c.jar) != (c.want == SessionHot) {
				t.Fatalf("LoggedIn = %v, want %v", LoggedIn(c.jar), c.want == SessionHot)
			}
		})
	}
}

// TestChromeCookieJarLive validates extraction against the real local Chrome.
// Requires a hot SNCF Connect session in the Default profile and a Keychain
// allow. Skipped unless SNCF_LIVE_COOKIE_TEST=1 (it pops a Keychain prompt).
func TestChromeCookieJarLive(t *testing.T) {
	if os.Getenv("SNCF_LIVE_COOKIE_TEST") != "1" {
		t.Skip("set SNCF_LIVE_COOKIE_TEST=1 to run (touches Keychain)")
	}
	jar, err := ChromeCookieJar("Default")
	if err != nil {
		t.Fatalf("ChromeCookieJar: %v", err)
	}
	n := strings.Count(jar, "; ") + 1
	if !strings.Contains(jar, "__Secure-cms-account=") {
		t.Fatalf("jar missing __Secure-cms-account (not logged in?) — %d cookies", n)
	}
	if !strings.Contains(jar, "__Secure-refresh-account-token=") {
		t.Fatalf("jar missing refresh token — %d cookies", n)
	}
	t.Logf("OK: %d sncf-connect cookies, logged-in markers present", n)
}

func TestPBKDF2SHA1(t *testing.T) {
	// RFC 6070 vector: P="password", S="salt", c=2, dkLen=20
	got := pbkdf2SHA1([]byte("password"), []byte("salt"), 2, 20)
	want := []byte{
		0xea, 0x6c, 0x01, 0x4d, 0xc7, 0x2d, 0x6f, 0x8c, 0xcd, 0x1e,
		0xd9, 0x2a, 0xce, 0x1d, 0x41, 0xf0, 0xd8, 0xde, 0x89, 0x57,
	}
	if string(got) != string(want) {
		t.Fatalf("pbkdf2SHA1 mismatch:\n got  %x\n want %x", got, want)
	}
}
