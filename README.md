# meta-ads

A fast, zero-dependency CLI for the [Meta Marketing API](https://developers.facebook.com/docs/marketing-apis/) (Facebook & Instagram Ads).

- **Agent-friendly** — outputs JSON automatically when piped, human tables in a terminal
- **Single static binary** — no runtime, no dependencies to install
- **Two auth modes** — browser OAuth or paste a token directly
- **Full read + write** — list, get, create, pause, update campaigns / ad sets / ads

---

## Installation

### Build from source

```bash
git clone https://github.com/the20100/meta-ads-cli
cd meta-ads-cli
go build -o meta-ads .
# optionally move to PATH
mv meta-ads /usr/local/bin/meta-ads
```

> Requires Go 1.22+

---

## Authentication

You need a Meta app with the **Marketing API** product enabled. Create one at [developers.facebook.com](https://developers.facebook.com) (Business type).

### Option 1 — Browser OAuth (recommended)

```bash
export META_APP_ID=<your_app_id>
export META_APP_SECRET=<your_app_secret>
meta-ads auth login
```

Opens your browser, completes the OAuth flow, and saves a long-lived token (~60 days) to `~/.config/meta-ads/config.json`.

Your app's **Redirect URI** must include `http://127.0.0.1` (any port) in the Meta Developer console.

### Option 2 — Paste a token directly

Get a token from the [Graph API Explorer](https://developers.facebook.com/tools/explorer/) and paste it:

```bash
meta-ads auth set-token EAABsbCS...
```

If `META_APP_ID` and `META_APP_SECRET` are available, the token is automatically upgraded to a long-lived one. Pass `--no-extend` to skip this.

### Option 3 — Extend a short-lived token

```bash
# Print the long-lived token
meta-ads auth extend-token EAABsbCS...

# Extend and save to config
meta-ads auth extend-token EAABsbCS... --save
```

### Check status / Logout

```bash
meta-ads auth status
meta-ads auth logout
```

---

## Usage

### Global flags

| Flag | Description |
|------|-------------|
| `-a, --account <id>` | Ad account ID (`act_` prefix optional) |
| `--json` | Force JSON output |
| `--pretty` | Force pretty-printed JSON |

**Tip:** Set `META_ADS_ACCOUNT=act_123456789` in your environment to avoid passing `--account` on every command.

---

### Accounts

```bash
# List all ad accounts you have access to
meta-ads accounts list
```

---

### Campaigns

```bash
# List campaigns
meta-ads campaigns list -a act_123456789
meta-ads campaigns list -a act_123456789 --status ACTIVE
meta-ads campaigns list -a act_123456789 --limit 20

# Get details
meta-ads campaigns get <campaign_id>

# Create
meta-ads campaigns create -a act_123456789 \
  --name "My Campaign" \
  --objective OUTCOME_SALES \
  --daily-budget 5000          # in cents → $50.00
  --status PAUSED

# Pause
meta-ads campaigns pause <campaign_id>

# Update
meta-ads campaigns update <campaign_id> --status ACTIVE
meta-ads campaigns update <campaign_id> --daily-budget 10000
meta-ads campaigns update <campaign_id> --name "New Name" --status PAUSED
```

**Objectives:** `OUTCOME_SALES` · `OUTCOME_AWARENESS` · `OUTCOME_TRAFFIC` · `OUTCOME_LEADS` · `OUTCOME_ENGAGEMENT` · `OUTCOME_APP_PROMOTION`

---

### Ad Sets

```bash
# List ad sets
meta-ads adsets list -a act_123456789
meta-ads adsets list -a act_123456789 --campaign <campaign_id>
meta-ads adsets list -a act_123456789 --status ACTIVE

# Get details
meta-ads adsets get <adset_id>

# Pause
meta-ads adsets pause <adset_id>

# Update budget
meta-ads adsets update-budget <adset_id> --daily-budget 2000
meta-ads adsets update-budget <adset_id> --lifetime-budget 50000
```

---

### Ads

```bash
# List ads
meta-ads ads list -a act_123456789
meta-ads ads list -a act_123456789 --adset <adset_id>
meta-ads ads list -a act_123456789 --status PAUSED

# Get details
meta-ads ads get <ad_id>

# Pause
meta-ads ads pause <ad_id>
```

---

### Insights

```bash
# Account-level insights
meta-ads insights get -a act_123456789 \
  --since 2026-01-01 --until 2026-01-31

# Campaign-level breakdown
meta-ads insights get -a act_123456789 \
  --level campaign \
  --since 2026-01-01 --until 2026-01-31

# Insights for a specific campaign
meta-ads insights get <campaign_id> \
  --since 2026-01-01 --until 2026-01-31

# Custom fields and breakdowns
meta-ads insights get -a act_123456789 \
  --level ad \
  --fields "impressions,clicks,spend,ctr,cpc,actions" \
  --breakdowns age,gender \
  --since 2026-01-01 --until 2026-01-31
```

**Levels:** `account` · `campaign` · `adset` · `ad`

**Common fields:** `impressions` · `clicks` · `spend` · `reach` · `ctr` · `cpc` · `cpm` · `cpp` · `actions` · `conversions` · `frequency` · `unique_clicks`

**Breakdowns:** `age` · `gender` · `country` · `device_platform` · `publisher_platform` · `impression_device`

---

### Audiences

```bash
meta-ads audiences list -a act_123456789
```

---

### Pixels

```bash
meta-ads pixels list -a act_123456789
```

---

### Update — Self-update

Pull the latest source from GitHub, rebuild, and replace the current binary.

```bash
meta-ads update
```

Requires `git` and `go` to be installed.

---

## JSON output & agent use

All commands output JSON automatically when stdout is not a TTY (e.g. when piped):

```bash
# Pipe to jq
meta-ads campaigns list -a act_123456789 | jq '.[].name'

# Save to file
meta-ads insights get -a act_123456789 --level campaign \
  --since 2026-01-01 --until 2026-01-31 > insights.json

# Force pretty JSON in terminal
meta-ads campaigns list -a act_123456789 --pretty
```

---

## Config file

Credentials are stored at:

| OS | Path |
|----|------|
| macOS | `~/Library/Application Support/meta-ads/config.json` |
| Linux | `~/.config/meta-ads/config.json` |
| Windows | `%AppData%\meta-ads\config.json` |

The file is created with `0600` permissions. It stores the access token, user info, optional app credentials, and a default account ID. **Never commit this file.**

---

## Notes on budgets

Meta returns and accepts budgets as **integer cents** (minor currency units). The CLI formats them for display (e.g. `5000` → `50.00`) but the raw value is used when creating or updating.

```bash
# $50.00/day
meta-ads campaigns create ... --daily-budget 5000

# €100.00 lifetime
meta-ads campaigns create ... --lifetime-budget 10000
```

---

## License

MIT
