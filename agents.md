# agents.md — AI Agent Guide for CLISNCF

Reference for any AI agent (or human) with shell access to operate this CLI.
CLISNCF is a personal CLI for SNCF Connect, authenticated against a real account.
Binary: `./sncfcli` (or `sncfcli` if installed via `make install`).

## Quick start

```bash
sncfcli auth status          # check session state (HOT/COLD/expired)
sncfcli search --from "Paris" --to "Lyon" --date 2026-07-10
sncfcli book --from "Paris" --to "Lyon" --date 2026-07-10 --select 0 -y
sncfcli cart                 # see what's in the basket
sncfcli trips list --past    # past trips
```

All commands output to stdout (tables by default, `--json` for structured).
Diagnostics go to stderr. `--yes` / `-y` skips confirmation prompts.

## Authentication

Session lives in `~/.config/sncfcli/session.json` (access + ID + refresh tokens).
Auto-refreshes on every command when expired. If refresh fails, re-login:

```bash
sncfcli auth login --chrome   # headless OIDC, reads creds from Chrome, user types OTP
sncfcli auth whoami           # verify identity
sncfcli auth status           # HOT (valid) / COLD (needs refresh) / expired
```

## Searching trains

```bash
sncfcli search --from "Paris" --to "Marseille" --date 2026-08-01 --time 09:00
```

- Returns first page (6 results). Paginate with:
  ```bash
  sncfcli search more <itineraryID>              # next page
  sncfcli search more --previous <itineraryID>   # previous page
  ```
- The itinerary ID is printed to stderr after each search.
- Prices include the account's discount cards (Avantage Jeune, etc.) automatically.
- `--json` gives full offer details (offer IDs, fare names, segment IDs).
- `--raw` dumps the full BFF `/api/v1/itineraries` response (debug).

## Booking a train

```bash
# Step 1: search (implicit in book)
sncfcli book --from "Paris" --to "Toulon" --date 2026-07-10 --time 08:00 --select 2 -y

# With seat preference (1st class recommended for SOLO):
sncfcli book --from "Paris" --to "Toulon" --date 2026-07-10 --select 2 --offer 1 \
  --seat SOLO --deck BAS -y
```

**Flags:**
- `--select N` — index of the train in search results (0-based)
- `--offer N` — index of the fare within that train (0 = cheapest, usually 2nd class)
- `--seat CODE` — seat preference (best-effort, see table below)
- `--deck CODE` — deck preference: `BAS`, `HAUT`, or `ANY` (default)

**The CLI does NOT pay.** It stops after adding to cart. Payment must be completed
on sncf-connect.com or the app (3D Secure required).

### Seat preference codes

| Code | Description | 2nd class | 1st class |
|------|-------------|-----------|-----------|
| `FENETRE` | Window | available but often ignored | honored |
| `COULOIR` | Aisle | honored | honored |
| `DUO_COTE_A_COTE` | Side-by-side duo | honored | honored |
| `CLUB_QUATRE` | Four-seat square | honored | honored |
| `SOLO` | Isolated seat | NOT available | honored |
| `DUO_VIS_A_VIS` | Face-to-face duo | NOT available | honored |

Preferences are "best effort" — the SNCF server may assign a different seat.
In 2nd class, `FENETRE` is systematically ignored on tested trains. `SOLO` only
exists in 1st class. To guarantee an isolated seat, book 1st class with `--seat SOLO`.

## Cart management

```bash
sncfcli cart                 # show basket (table)
sncfcli cart --json          # structured output
sncfcli cart --raw           # full BFF JSON (includes seat assignment details)
sncfcli cart clear           # remove all items from cart
```

`cart --raw` is useful to verify the exact seat assigned (coach, seat number,
description, icon). The parsed `cart` command only shows route/price.

## Trips & proofs

```bash
sncfcli trips list           # upcoming trips
sncfcli trips list --past    # past trips
sncfcli trips show <tripID>  # single trip details
sncfcli proofs list          # trips eligible for justificatif
sncfcli proofs generate --email user@example.com --name "Name" --trips <id1>,<id2>
```

## Account

```bash
sncfcli account display      # profile, discount cards, loyalty number
sncfcli account companions list
sncfcli account payment-cards
```

## Alerts

```bash
sncfcli alerting list
sncfcli alerting calendar --from "Paris" --to "Lyon" --month 2026-07
sncfcli alerting create-low-price --from "Paris" --to "Lyon" --date 2026-08-01
```

## Real-time info

```bash
sncfcli boards "Paris Gare de Lyon"          # departures
sncfcli boards "Paris Gare de Lyon" --arrivals
sncfcli status <trainNumber> --date 2026-07-10
sncfcli vehicle <trainNumber> --date 2026-07-10 --from "Paris" --to "Toulon"
sncfcli traffic                              # disruptions (no auth needed)
```

## Gotchas

1. **Datadome rate-limiting**: making many rapid requests from the same IP will
   trigger a 403 block. Space out requests. If blocked, use `SNCF_PROXY` env var
   with a residential/ISP proxy (e.g. Decodo: `SNCF_PROXY=http://user:pass@isp.decodo.com:10000`).
   Different ports = different IPs.

2. **Search pagination**: `sncfcli search` returns 6 results per page. To see all
   trains for a day, you need to paginate 3-4 times via `sncfcli search more <id>`.

3. **Book replaces vs. appends**: `sncfcli book` does NOT clear the cart first.
   If you book twice, you get two items in the cart. Always `sncfcli cart clear`
   before re-booking if you want to replace.

4. **Select index depends on --time**: the `--select` index is relative to the
   search results page, which depends on `--time`. To find a specific train,
   do a dry-run first with `--select 99` (will error with the list + valid range).

5. **Discount cards are automatic**: the CLI injects all discount cards from the
   account into every search. Displayed prices already include reductions. No flag needed.

6. **Age is hardcoded to 30**: the search passenger age is 30 regardless of the
   account holder's actual age. This doesn't affect pricing (the card code determines
   the discount), but the `ageRank` label will show "30-59 ans".

7. **Payment is a structural wall**: the CLI cannot complete payment. It uses
   SIPS tokenization + 3D Secure which requires a browser. The CLI stops at
   `finalization/create`. Redirect the user to sncf-connect.com to pay.

8. **Seat map (1st class)**: 1st class offers `FRONT_SEAT_MAP` as default placement
   mode, meaning you could pick an exact seat from the seat map. The CLI currently
   only supports preference mode (`FRONT_PREFERENCES`), not exact seat picking.
   The `GetSeatMap` API method exists but is not wired to a CLI command.

## Environment variables

| Variable | Purpose |
|----------|---------|
| `SNCF_PROXY` | HTTP proxy for all requests (e.g. `http://user:pass@host:port`) |
| `SNCF_LIVE` | Set to `1` to run live E2E tests |
| `SNCF_LIVE_COOKIE_TEST` | Set to `1` for Chrome cookie extraction tests |
