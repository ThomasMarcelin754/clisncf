# sncfcli

Personal CLI for [SNCF Connect](https://www.sncf-connect.com) — search trains, book tickets, manage your account from the terminal.

## Install

**Homebrew (macOS):**

```bash
brew install --cask ThomasMarcelin754/tap/sncfcli
```

**From release (Linux/macOS):**

```bash
curl -sL https://github.com/ThomasMarcelin754/clisncf/releases/latest/download/sncfcli_$(uname -s | tr A-Z a-z)_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz | tar xz -C /usr/local/bin sncfcli
```

**From source:**

```bash
go install github.com/thomasmarcelin/sncf-cli/cmd/sncf@latest
```

## Usage

```bash
# Authenticate
sncfcli auth login          # headless OIDC (email + password + OTP)
sncfcli auth status         # HOT / COLD / expired

# Search & book
sncfcli search --from "Paris" --to "Lyon" --date 2026-07-10
sncfcli book --from "Paris" --to "Lyon" --date 2026-07-10 --select 0 -y
sncfcli cart                # view basket

# Trips & proofs
sncfcli trips list --past
sncfcli proofs list
sncfcli proofs generate --email you@example.com --trips <id1>,<id2>

# Real-time
sncfcli boards "Paris Gare de Lyon"
sncfcli status 6231 --date 2026-07-10
sncfcli traffic

# Account
sncfcli account display
sncfcli account companions list
sncfcli alerting list
```

All commands output tables by default. Use `--json` for structured output.

## Auth

Session is stored in `~/.config/sncfcli/session.json`. Tokens auto-refresh on each command. If refresh fails, re-login with `sncfcli auth login`.

## Building

```bash
make build     # binary with version/commit/date
make install   # copy to /usr/local/bin/sncfcli
make check     # vet + lint + test + govulncheck + build
```

## License

Personal project. No warranty.
