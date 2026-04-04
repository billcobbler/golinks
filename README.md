# golinks

[![CI](https://github.com/billcobbler/golinks/actions/workflows/ci.yml/badge.svg)](https://github.com/billcobbler/golinks/actions/workflows/ci.yml)

A self-hosted short-link service. Type `go/something` in your browser and get redirected. Scales from a personal home-lab setup to a small team.

**Included:**

- **Go server** — redirect handler + REST API + embedded web dashboard
- **CLI tool** — manage links from the terminal (`golinks add`, `ls`, `edit`, …)
- **Browser extension** — Chrome & Firefox; intercepts `go/` URLs + quick-add popup
- **Pattern links** — `go/gh/myorg/myrepo` → `https://github.com/myorg/myrepo`
- **Analytics** — click counting with configurable retention
- **Export / Import** — JSON and CSV
- **Auth** — none (personal), local accounts, or OAuth (Google / GitHub)


---

## Quick start

### Option A: Build from source

Requires Go 1.25+.

```bash
git clone https://github.com/billcobbler/golinks
cd golinks
make tidy   # download dependencies
make build  # builds bin/golinks-server
./bin/golinks-server
```

The server starts on `:8080` and creates `golinks.db` in the current directory.
Open `http://localhost:8080/-/` for the web dashboard.

### Option B: Docker Compose

```bash
git clone https://github.com/billcobbler/golinks
cd golinks
docker compose up -d
```

### Option C: Homebrew (macOS / Linux)

```bash
brew tap golinks/tap
brew install golinks-server   # server binary + brew services integration
brew install golinks           # CLI tool
brew services start golinks-server
```

---

## DNS setup

For `go/shortname` to work in the browser address bar, the hostname `go` must resolve to your server. Pick the method that fits your setup:

### Hosts file (single machine)

**macOS / Linux** — add to `/etc/hosts`:
```
127.0.0.1   go
```

**Windows** — add to `C:\Windows\System32\drivers\etc\hosts` (run Notepad as Administrator):
```
127.0.0.1   go
```

Flush DNS after editing:
```bash
# macOS
sudo dscacheutil -flushcache; sudo killall -HUP mDNSResponder
# Linux
sudo systemd-resolve --flush-caches
# Windows (run as Administrator)
ipconfig /flushdns
```

### Local network DNS (dnsmasq / Pi-hole)

Add to `/etc/dnsmasq.d/golinks.conf`:
```
address=/go/192.168.1.50
```
Replace `192.168.1.50` with the server's LAN IP, then `sudo systemctl restart dnsmasq`.

---

## Browser extension

The extension makes `go/` links work natively in Chrome and Firefox, provides an omnibox shortcut, and adds a popup for creating links from any page.

### Install from a release

1. Download the latest release from the [Releases page](https://github.com/billcobbler/golinks/releases):
   - **Chrome**: `golinks-extension-chrome-<version>.zip`
   - **Firefox**: `golinks-extension-firefox-<version>.zip`
2. Follow the browser-specific steps below.

### Chrome

**From the zip (unsigned, development/self-hosted use):**

1. Unzip `golinks-extension-chrome-<version>.zip` to a permanent folder (e.g. `~/golinks-extension`).
2. Open `chrome://extensions`.
3. Enable **Developer mode** (top-right toggle).
4. Click **Load unpacked** and select the unzipped folder.

**From the Chrome Web Store** *(once published)*: Search for "Golinks" and click **Add to Chrome**.

### Firefox

**From the zip (temporary, survives until browser restart):**

1. Open `about:debugging#/runtime/this-firefox`.
2. Click **Load Temporary Add-on…** and select the `.zip` file directly.

**Permanent install (self-signed):**

1. Go to `about:config` and set `xpinstall.signatures.required` to `false`.
2. Rename the zip to `.xpi` and open it in Firefox, or drag it onto `about:addons`.

**From Firefox Add-ons** *(once published)*: Search for "Golinks" at [addons.mozilla.org](https://addons.mozilla.org).

### Configure the extension

After installing, open the extension's **Options** page:

- **Chrome**: right-click the extension icon → *Options*, or visit `chrome://extensions` → *Details* → *Extension options*.
- **Firefox**: `about:addons` → Golinks → *Preferences*.

Set:

| Field | Value |
|---|---|
| Server URL | The base URL of your golinks server, e.g. `http://localhost:8080` or `http://go` |
| API token | Optional. Required when `GOLINKS_AUTH` is not `none`. Generate one from the dashboard's **Settings** page. |

Click **Save**, then **Test connection** to verify the extension can reach your server.

### Using the extension

| How | What happens |
|---|---|
| Type `go/standup` in the address bar | **Arc**: works automatically — Arc treats single-word paths as URLs, the extension intercepts and redirects. No DNS setup needed. **Chrome/Firefox**: doesn't work — those browsers send single-word hostnames to the search engine before the extension can intercept. Use the omnibox instead. |
| Type `http://go/standup` in the address bar | Works in Chrome and Firefox — explicit scheme forces navigation, extension intercepts and redirects. (Not needed in Arc, which handles bare `go/` already.) |
| Type `go ` in the address bar (note the space) | **Chrome/Firefox only** — activates the omnibox keyword. Type a shortname and press Enter to navigate, or select a live suggestion from your link list. Not supported in Arc (Arc doesn't implement the Chrome extension omnibox API). |
| Click the extension icon | Opens a popup pre-filled with the current page's URL and a suggested shortname — click **Save link** to create a golink |

> **Arc users:** You get the best experience — just type `go/standup` directly and press Enter. The extension intercepts the request before it hits DNS, so no hosts file or DNS changes are needed. The omnibox keyword (`go ` + space) is not available in Arc.
>
> **Chrome/Firefox users:** Those browsers decide whether to search or navigate *before* the extension is consulted. Since `go` is a single-word hostname with no dot, they default to searching. Use `go ` + Space (omnibox) or `http://go/shortname` as alternatives. No DNS or hosts-file entry is required for either.

### Build the extension locally

```bash
# Generate icons (requires Docker; override with NODE=node if Node ≥ 18 is installed)
make extension-icons

# Run unit tests
make extension-test

# Package for both browsers
make extension-pack
# Produces: bin/golinks-extension-chrome-<version>.zip
#           bin/golinks-extension-firefox-<version>.zip
```

---

## CLI tool

```bash
# Install
make build-cli            # builds bin/golinks
# or
brew install golinks      # macOS / Linux via Homebrew

# Point the CLI at your server
golinks config set server http://localhost:8080
golinks config set token  <api-token>   # omit when GOLINKS_AUTH=none

# Common commands
golinks add standup https://meet.example.com/daily
golinks ls
golinks ls --search meet
golinks info standup
golinks edit standup --url https://meet.example.com/new-link
golinks rm standup
golinks open standup      # opens in the default browser
golinks stats

# Export / import
golinks export --format json --output links.json
golinks import links.json
```

Config is stored in `~/.config/golinks/config.yaml`.

---

## Authentication

Set `GOLINKS_AUTH` to control who can access the server:

| Mode | Behaviour |
|---|---|
| `none` (default) | No auth. All routes public. Suitable for personal home-lab use. |
| `local` | Username/password login. Run the server, visit `/-/auth/setup` to create the first account. |
| `oauth` | OAuth only (Google or GitHub). No local passwords. |
| `local+oauth` | Both methods available on the login page. |

### Local auth setup

```bash
GOLINKS_AUTH=local ./bin/golinks-server
# Open http://localhost:8080/-/auth/setup and create the first account.
```

### OAuth setup

Register an OAuth app with your provider:

**Google** — [console.cloud.google.com](https://console.cloud.google.com)
- Create a project → *APIs & Services* → *Credentials* → *OAuth 2.0 Client ID*
- Authorised redirect URI: `<GOLINKS_BASE_URL>/-/auth/callback`

**GitHub** — [github.com/settings/developers](https://github.com/settings/developers)
- *New OAuth App*
- Authorization callback URL: `<GOLINKS_BASE_URL>/-/auth/callback`

Then run:
```bash
GOLINKS_AUTH=oauth \
GOLINKS_OAUTH_PROVIDER=google \
GOLINKS_OAUTH_CLIENT_ID=<client-id> \
GOLINKS_OAUTH_CLIENT_SECRET=<client-secret> \
GOLINKS_BASE_URL=https://go.example.com \
./bin/golinks-server
```

### API tokens (for CLI and extension)

When auth is enabled, the CLI and browser extension need a token.

1. Log in to the dashboard.
2. Go to **Settings** (top-right nav).
3. Enter a label (e.g. "laptop CLI") and click **Generate token**.
4. Copy the token — it is shown only once.

```bash
golinks config set token <token>          # CLI
# Or enter it in the extension's Options page
```

### Non-TLS environments

Session cookies have the `Secure` flag set by default, which prevents them from being sent over plain HTTP. For home-lab setups without TLS, set:

```bash
GOLINKS_INSECURE_COOKIES=true
```

---

## Configuration reference

| Variable | Default | Description |
|---|---|---|
| `GOLINKS_PORT` | `8080` | HTTP listen port |
| `GOLINKS_DB` | `./golinks.db` | Database path. Use `postgres://user:pass@host/db` for Postgres. |
| `GOLINKS_AUTH` | `none` | Auth mode: `none`, `local`, `oauth`, `local+oauth` |
| `GOLINKS_OAUTH_PROVIDER` | — | `google` or `github` |
| `GOLINKS_OAUTH_CLIENT_ID` | — | OAuth app client ID |
| `GOLINKS_OAUTH_CLIENT_SECRET` | — | OAuth app client secret |
| `GOLINKS_BASE_URL` | — | Public base URL (required for OAuth callback) |
| `GOLINKS_INSECURE_COOKIES` | `false` | Set `true` to allow session cookies over plain HTTP |
| `GOLINKS_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `GOLINKS_ANALYTICS_RETENTION` | `0` | Days to retain click events. `0` = keep forever. |

---

## Postgres

Pass a Postgres DSN as `GOLINKS_DB` and the server switches drivers automatically:

```bash
GOLINKS_DB=postgres://golinks:secret@localhost:5432/golinks?sslmode=disable \
./bin/golinks-server
```

A minimal setup:
```sql
CREATE USER golinks WITH PASSWORD 'secret';
CREATE DATABASE golinks OWNER golinks;
```

Migrations run automatically on startup. SQLite is the default and requires no setup.

---

## REST API

All endpoints are under `/-/api/` and return JSON.
When `GOLINKS_AUTH` is not `none`, include `Authorization: Bearer <token>` on all API requests.
The health endpoint is always public.

### Links

| Method | Path | Description |
|---|---|---|
| `GET` | `/-/api/links` | List links. Query: `q` (search), `offset`, `limit` |
| `POST` | `/-/api/links` | Create a link |
| `GET` | `/-/api/links/{shortname}` | Get a link (supports namespaced paths) |
| `PUT` | `/-/api/links/{shortname}` | Update a link |
| `DELETE` | `/-/api/links/{shortname}` | Delete a link |

**Create / Update body:**
```json
{
  "shortname":   "gh",
  "target_url":  "https://github.com/{*}",
  "description": "GitHub — append any path",
  "is_pattern":  true
}
```

### Stats & utilities

| Method | Path | Description |
|---|---|---|
| `GET` | `/-/api/stats` | Total links, total clicks, top links, recent activity |
| `GET` | `/-/api/export?format=json` | Export all links (`format=json` or `format=csv`) |
| `POST` | `/-/api/import?overwrite=false` | Import links from JSON array or CSV |
| `GET` | `/-/api/health` | Health check (always public) |

---

## Pattern links

A single shortname can handle dynamic paths using placeholders.

| Placeholder | Matches |
|---|---|
| `{*}` | The entire remaining path |
| `{1}`, `{2}`, … | Individual `/`-separated path segments |

**Examples:**

| Shortname | Target URL | Request | Resolves to |
|---|---|---|---|
| `gh` | `https://github.com/{*}` | `go/gh/org/repo` | `https://github.com/org/repo` |
| `jira` | `https://jira.example.com/browse/{1}` | `go/jira/PROJ-123` | `https://jira.example.com/browse/PROJ-123` |
| `meet` | `https://meet.google.com/{1}` | `go/meet/abc-xyz` | `https://meet.google.com/abc-xyz` |

Longest-prefix matching is used when multiple pattern links could match.

---

## Linux service (systemd)

```bash
sudo cp bin/golinks-server /usr/local/bin/
sudo useradd -r -s /bin/false golinks
sudo mkdir -p /var/lib/golinks /etc/golinks
sudo chown golinks /var/lib/golinks

# Create the environment file
sudo tee /etc/golinks/env <<'EOF'
GOLINKS_DB=/var/lib/golinks/golinks.db
GOLINKS_AUTH=none
EOF

sudo cp scripts/golinks.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now golinks
```

## macOS service (launchd)

```bash
sudo cp bin/golinks-server /usr/local/bin/
mkdir -p /usr/local/var/golinks
cp scripts/com.golinks.server.plist ~/Library/LaunchAgents/
launchctl load ~/Library/LaunchAgents/com.golinks.server.plist
```

---

## Development

```bash
make test            # run all Go tests
make test-race       # with race detector
make lint            # golangci-lint
make build-all       # cross-compile for linux/darwin/windows (amd64 + arm64)
make extension-test  # run browser extension unit tests (Docker)
make download-assets # re-download Pico CSS + htmx.min.js
```

---

## Project structure

```
cmd/
  server/          Server entrypoint
  cli/             CLI entrypoint + subcommands
internal/
  config/          Config loading from environment
  models/          Data types (Link, User, Session, …)
  store/           SQLite + Postgres persistence layer
  api/             HTTP router, REST handlers, middleware
  auth/            Session + token middleware, OAuth helpers, bcrypt
  redirect/        Redirect logic and pattern substitution
  web/             Dashboard handlers + embedded templates/static assets
extension/
  manifest.json    Manifest V3 (Chrome + Firefox)
  background/      Service worker (omnibox, declarativeNetRequest)
  popup/           Quick-add popup
  options/         Settings page
  tests/           Unit tests (Node.js built-in runner)
scripts/
  golinks.service         systemd unit file
  com.golinks.server.plist  macOS LaunchAgent
homebrew-tap/
  Formula/         Homebrew formula templates (updated by release workflow)
```
