# Subscription Tracker

A self-contained web app for tracking recurring subscriptions. Runs as a single binary, stores data in a local JSON file, and serves a simple HTMX-powered web UI.

## Features

- **Dashboard** — spending summary cards (monthly/yearly AUD), breakdown by tag, top 5 most expensive subscriptions
- **Subscription list** — sortable table with real-time search and tag filtering
- **Tags** — replace the single "Category" field from the spreadsheet; multiple tags per subscription, multi-select filtering; rename or delete tags globally via **Actions → Manage Tags**
- **Currency conversion** — all costs shown in AUD regardless of original currency (USD→AUD via [Frankfurter](https://www.frankfurter.app/), cached for 6 hours)
- **Active / Cancelled status** — explicit status field; cancelled subscriptions are hidden by default with a toggle to show them
- **Add / Edit / Delete** — inline modal forms, no page reloads
- **Import** — import directly from the original Excel spreadsheet (`.xlsx`)
- **Export** — download your data as CSV or JSON at any time
- **Dark mode** — toggle in the nav bar, preference saved to `localStorage`
- **Single binary** — all assets (HTML, CSS, JS) are embedded; works fully offline after first run
- **Docker** — included scripts to package into a Docker image and deploy on another machine


## Requirements

- [Go 1.22+](https://go.dev/dl/) to build from source
- Docker (optional, for containerised deployment)


## Quick start

```bash
# Clone / download the repo, then:
go run .
```

Open [http://localhost:8080](http://localhost:8080) in your browser.

On first run a fresh `subscriptions.json` data file is created in the current directory.


## Importing your existing spreadsheet

1. Open the app and click **Actions → Import XLSX**
2. Select your `.xlsx` file
3. Check **Replace all existing data** if you want a clean import
4. Click **Import**

The importer maps the original spreadsheet columns by header name and:

- Converts the single **Category** column to a **Tag** (e.g. `Entertainment - Podcast` → `entertainment-podcast`)
- Detects cancelled subscriptions: rows where cost is `0` and the Notes field contains "cancelled" are imported as `status: cancelled`
- Handles Excel's `MM-DD-YY` date format automatically


## Configuration

Configuration is via environment variables. All have sensible defaults.

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Port the HTTP server listens on |
| `DATA_FILE` | `./subscriptions.json` | Path to the JSON data file |
| `CURRENCY_TTL_MINUTES` | `360` | How long to cache the exchange rate (minutes) |

Example:

```bash
PORT=9000 DATA_FILE=/home/me/data/subs.json go run .
```


## Building a binary

```bash
go build -o subtracker .
./subtracker
```


## Docker deployment

### 1. Build and package on your Mac

```bash
./package.sh
```

This will:
1. Cross-compile the binary for Linux (amd64)
2. Build a Docker image (`subscription-tracker:latest`)
3. Save it to a `.tar.gz` file (e.g. `subscription-tracker-20260302.tar.gz`)

Transfer the `.tar.gz` file and `docker-compose.yml` to your Windows server (put them in the same folder).

### 2. Configure port and data path

Edit `docker-compose.yml` to set the host port and data directory before deploying:

```yaml
services:
  subscription-tracker:
    image: subscription-tracker:latest
    container_name: subscription-tracker
    ports:
      - "5011:8080"          # change the left side to your preferred host port
    environment:
      DATA_FILE: /data/subscriptions.json
    volumes:
      - ./data:/data         # host folder (next to docker-compose.yml) mapped into the container
    restart: unless-stopped
```

Data is stored in a `data/` folder alongside `docker-compose.yml` on the host, so it persists across container restarts and image upgrades.

### 3. Deploy on Windows

Copy the `.tar.gz` tarball into the same folder as `docker-compose.yml`, then run:

```powershell
.\deploy.ps1
```

The script will:
1. Find the newest `subscription-tracker-*.tar.gz` in the current directory
2. Load the Docker image
3. Start (or recreate) the container via `docker-compose`

To deploy a specific tarball:

```powershell
.\deploy.ps1 -ImageFile "subscription-tracker-20260302.tar.gz"
```

Port and data path are configured in `docker-compose.yml`, not in `deploy.ps1`.


## Backing up your data

`backup.ps1` creates compressed backups of `subscriptions.json` with automatic daily/weekly/monthly rotation. Run it from the same folder as `docker-compose.yml`.

```powershell
.\backup.ps1
```

By default, backups are written to a `backups\` folder next to the script:

```
backups\
  daily\    subscription-tracker-2026-03-03_120000.zip   # kept for 7 days
  weekly\   subscription-tracker-2026-03-02_120000.zip   # kept for 4 weeks (Sundays)
  monthly\  subscription-tracker-2026-03-01_120000.zip   # kept for 12 months (1st of month)
```

To send backups to a different location (e.g. a NAS):

```powershell
.\backup.ps1 -BackupDir "Z:\Backups\SubscriptionTracker"
.\backup.ps1 -BackupDir "\\nas\backups\subscriptiontracker" -DailyKeep 14
```

| Parameter | Default | Description |
|---|---|---|
| `-BackupDir` | `.\backups` | Destination for backup archives |
| `-DataDir` | `.\data` | Folder containing `subscriptions.json` |
| `-DailyKeep` | `7` | Number of daily backups to retain |
| `-WeeklyKeep` | `4` | Number of weekly backups to retain |
| `-MonthlyKeep` | `12` | Number of monthly backups to retain |

To run automatically, add a Windows Task Scheduler entry pointing to `backup.ps1`.


## Project structure

```
SubscriptionTracker/
├── main.go                     # Entry point, config, routing
├── Dockerfile                  # Minimal alpine image (binary only)
├── docker-compose.yml          # Container config: port, data path, restart policy
├── package.sh                  # Mac: build + package to Docker .tar.gz
├── deploy.ps1                  # Windows: load image + docker-compose up
├── backup.ps1                  # Windows: backup subscriptions.json with rotation
├── internal/
│   ├── model/subscription.go   # Core types and cost calculation methods
│   ├── store/json_store.go     # Atomic JSON file persistence
│   ├── currency/converter.go   # Frankfurter API client with cache
│   ├── handler/                # HTTP handlers and view models
│   └── importer/xlsx.go        # Excel import logic
└── web/
    ├── templates/               # HTML templates (embedded in binary)
    └── static/                  # htmx, Pico CSS, custom CSS (embedded)
```


## Data file format

The data is stored as a plain JSON file you can inspect or back up with any text editor.

```json
{
  "version": 1,
  "tags": ["entertainment-podcast", "productivity", "gaming"],
  "subscriptions": [
    {
      "id": "a1b2c3d4-...",
      "name": "1Password",
      "description": "Family plan",
      "start_date": "2017-01-29T00:00:00Z",
      "cost": 71.88,
      "currency": "USD",
      "cycle": "yearly",
      "tags": ["productivity"],
      "notes": "",
      "status": "active",
      "created_at": "2026-03-02T...",
      "updated_at": "2026-03-02T..."
    }
  ]
}
```

Valid values:

| Field | Values |
|---|---|
| `currency` | `AUD`, `USD` |
| `cycle` | `monthly`, `yearly`, `every2years` |
| `status` | `active`, `cancelled` |


## Tech stack

| Layer | Choice |
|---|---|
| Backend | Go 1.22, `net/http` |
| Frontend | [HTMX](https://htmx.org/) + [Pico CSS](https://picocss.com/) |
| Templating | `html/template` (built-in) |
| Storage | JSON file |
| xlsx parsing | [excelize](https://github.com/xuri/excelize) |
| Currency | [Frankfurter API](https://www.frankfurter.app/) (free, no key required) |
