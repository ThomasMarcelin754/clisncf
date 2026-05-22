# CLAUDE.md — CLISNCF

Personal CLI for SNCF Connect. Go + cobra + enetx/surf.

## Build & check

```bash
make check    # vet → lint (gosec) → test -race → govulncheck → build
make build    # binary with ldflags (version/commit/date)
make install  # → /usr/local/bin/sncf
```

## Project layout

- `cmd/sncf/` — cobra commands (glue layer, no unit tests by design)
- `internal/api/` — SNCF BFF client, surf transport, Datadome bypass
- `internal/auth/` — headless OIDC login, Chrome cookie extraction, session persistence
- `internal/output/` — JSON/table output helpers

## Test philosophy

Per-package coverage floors, not a gamed aggregate total.

**Unit-tested** (CI-enforced floors):
- `internal/auth` (>=8%) — SessionState machine, PBKDF2-SHA1, cookie parsing

**Live E2E only** (require `SNCF_LIVE=1` or `SNCF_LIVE_COOKIE_TEST=1`):
- `internal/api` — surf transport, Datadome pass, BFF endpoint tiers
- Chrome cookie extraction (touches Keychain)

**No unit tests by design:**
- `cmd/sncf/` — cobra command glue, tested via live backtest
- `internal/output/` — trivial encoder wrappers

Coverage floors will be raised as unit-testable logic is added to each package.

## Secrets

Never in the repo, tests, fixtures, or commit messages. Runtime session lives
in `~/.config/clisncf/session.json` (0600, gitignored). Use obvious
placeholders in code (`"eyJ.payload.sig"`, `"supersecretjwt"`).
