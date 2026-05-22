package api

import (
	"os"

	"github.com/enetx/g"
	"github.com/enetx/surf"
)

// newSurfClient returns a surf HTTP client impersonating Chrome with full
// JA3 + HTTP/2 fingerprint. enetx/surf passes Datadome on ALL SNCF endpoints
// (unlike bogdanfinn/tls-client which gets 403 on protected endpoints).
// Set SNCF_PROXY=http://user:pass@host:port to route through a proxy.
func newSurfClient() *surf.Client {
	b := surf.NewClient().
		Builder().
		Session().
		Impersonate().
		Chrome()

	if proxy := os.Getenv("SNCF_PROXY"); proxy != "" {
		b = b.Proxy(g.String(proxy))
	}

	return b.Build().Unwrap()
}
