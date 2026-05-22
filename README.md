# sncf

Personal CLI for [SNCF Connect](https://www.sncf-connect.com) — search trains, book tickets, manage your account from the terminal.

## Install

**Homebrew (macOS):**

```bash
brew install --cask ThomasMarcelin754/tap/sncf
```

**From release (Linux/macOS):**

```bash
curl -sL https://github.com/ThomasMarcelin754/clisncf/releases/latest/download/sncf_$(uname -s | tr A-Z a-z)_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz | tar xz -C /usr/local/bin sncf
```

**From source:**

```bash
go install github.com/thomasmarcelin/sncf-cli/cmd/sncf@latest
```

## Usage

```bash
# Authenticate
sncf auth login          # headless OIDC (email + password + OTP)
sncf auth status         # HOT / COLD / expired

# Search & book
sncf search --from "Paris" --to "Lyon" --date 2026-07-10
sncf book --from "Paris" --to "Lyon" --date 2026-07-10 --select 0 -y
sncf cart                # view basket

# Trips & proofs
sncf trips list --past
sncf proofs list
sncf proofs generate --email you@example.com --trips <id1>,<id2>

# Real-time
sncf boards "Paris Gare de Lyon"
sncf status 6231 --date 2026-07-10
sncf traffic

# Account
sncf account display
sncf account companions list
sncf alerting list
```

All commands output tables by default. Use `--json` for structured output.

## Auth

Session is stored in `~/.config/clisncf/session.json`. Tokens auto-refresh on each command. If refresh fails, re-login with `sncf auth login`.

## Building

```bash
make build     # binary with version/commit/date
make install   # copy to /usr/local/bin
make check     # vet + lint + test + govulncheck + build
```

## License

Personal project. No warranty.
