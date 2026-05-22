package api

import (
	"os"
	"testing"
)

// TestSurfTransport verifies the surf Chrome transport passes Datadome on all
// SNCF BFF endpoint tiers (public + protected).
// Run: SNCF_LIVE=1 go test ./internal/api -run TestSurf -v
func TestSurfTransportGET(t *testing.T) {
	if os.Getenv("SNCF_LIVE") == "" {
		t.Skip("SNCF_LIVE not set")
	}
	c, _ := NewConnectClient("")
	defer c.Close()

	resp, err := c.get("/api/v1/trafficinfo")
	if err != nil {
		t.Fatalf("trafficinfo: %v", err)
	}
	t.Logf("GET trafficinfo → HTTP %d (%d bytes)", int(resp.StatusCode), len(resp.Body.Bytes().Ok()))
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		t.Fatal("expected 200")
	}
}

func TestSurfTransportTrips(t *testing.T) {
	if os.Getenv("SNCF_LIVE") == "" {
		t.Skip("SNCF_LIVE not set")
	}
	c, _ := NewConnectClient("")
	defer c.Close()

	trips, err := c.GetTrips(true)
	if err != nil {
		t.Fatalf("trips: %v", err)
	}
	t.Logf("POST trips → %d past trips (anon=0 expected)", len(trips))
}

func TestSurfTransportAutocomplete(t *testing.T) {
	if os.Getenv("SNCF_LIVE") == "" {
		t.Skip("SNCF_LIVE not set")
	}
	c, _ := NewConnectClient("")
	defer c.Close()

	id, label, err := c.AutocompletePlace("Paris")
	if err != nil {
		t.Fatalf("autocomplete: %v", err)
	}
	t.Logf("POST autocomplete → id=%s label=%s", id, label)
}

func TestSurfTransportRefresh(t *testing.T) {
	if os.Getenv("SNCF_LIVE") == "" {
		t.Skip("SNCF_LIVE not set")
	}
	c, _ := NewConnectClient("")
	defer c.Close()

	_, _, _, _, err := c.Refresh("dummy")
	if err == nil {
		t.Fatal("expected error with dummy token")
	}
	// 400 = auth error (Datadome passed), 403 = Datadome block
	if containsStr(err.Error(), "captcha") {
		t.Fatalf("DATADOME BLOCK: %v", err)
	}
	t.Logf("refresh → %v (Datadome passed, auth error expected)", err)
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findStr(s, sub))
}

func findStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
