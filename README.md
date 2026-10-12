# Tempo

A local Vue + Go application for preparing your weekly Zoho Projects time entries.
Keep task-based drafts with work notes, review the week, copy a project report, and explicitly
submit selected drafts to Zoho. Existing Zoho time logs are available in a separate reference view.

## Run the built application
The Windows executable is generated at `dist/timetracker.exe`. Start it in PowerShell:

```powershell
./dist/timetracker.exe
```

Open [http://127.0.0.1:8080](http://127.0.0.1:8080). The first run offers Zoho setup.
For a fully usable sample workspace with simulated submissions:

```powershell
./dist/timetracker.exe -demo
```

Demo data is stored separately and no Zoho requests are made in demo mode.
The server runs until you press Ctrl+C. Use `-port 8081` to choose another loopback port;
the OAuth redirect URI then uses that port as well. Use `-data-dir C:/path/to/data` to choose storage.

## Zoho setup
1. In your regional [Zoho API console](https://api-console.zoho.com/), create a **server-based application**.
2. Register `http://127.0.0.1:8080/api/connection/callback` as its authorized redirect URI.
   If using a different port, copy the exact URI displayed by the app.
3. Open **Zoho settings** in Tempo. Select the data center where your account/client lives,
   enter the client ID and secret, and save them.
4. Click **Connect to Zoho**, grant access, and choose your work portal.
5. Refresh to load projects, tasks, and your time logs. The selected account's portal user is
   identified through the account profile and Zoho Projects user API.

Zoho Projects v3 is a separate product/API from the ERP Projects module.
See [integration details and required scopes](docs/ZOHO.md). If your company restricts API access,
ask its Zoho administrator for an allowed client and the necessary project/time-log permissions.

## Daily workflow
- Add an entry with its date, project, task, duration, billable status, and optional notes.
- Enter `1:30` or `1.5` for 90 minutes. Durations must represent whole minutes.
- Drafts save on this computer. Use the project filter or daily bars to focus the week.
- Generate and copy project reports **before submission**; reports contain current drafts only.
- Click **Review entries**, select drafts, and confirm to submit. Successes move to Submitted time.
- Failed entries can be edited. An uncertain submission stays locked: use **Check**, inspect the
  matching Zoho log, and either link it or explicitly release it after confirming no entry exists.
- Zoho time logs are a reference view and are excluded from all draft totals and reports.

## Build and develop
Requirements: Go **1.26 or newer**, Node **22.12 or newer**, and npm. A downloaded Go toolchain
may be available at `.tools/go/bin/go.exe` in this workspace. The delivered code was verified with
Go 1.27.2 and Node 24.13.1. Dependencies are pinned by `go.sum` and `web/package-lock.json`.

```powershell
npm ci --prefix web
npm run build --prefix web
go build -o dist/timetracker.exe ./cmd/timetracker
```

Build Vue before Go so the executable embeds the latest assets. No Node runtime is needed to run
the built executable. On Windows, `./scripts/build.ps1` performs the frontend build and Go build,
using the workspace toolchain when Go is not on PATH.

For development, run the backend and frontend in separate terminals:

```powershell
go run ./cmd/timetracker -demo
```

```powershell
npm run dev --prefix web
```

Open [http://127.0.0.1:5173](http://127.0.0.1:5173); Vite proxies API requests to port 8080.
OAuth callbacks return to the Go server on port 8080. Use the production URL for initial connection.

## Verify
```powershell
go test ./...
go vet ./...
npm test --prefix web
npm run build --prefix web
```

For browser tests, first build the executable, then install Playwright Chromium once:

```powershell
cd web
npx playwright install chromium
npx playwright test
```

Browser tests use isolated normal/demo databases and ports 8090/8091. See [validation details](docs/VALIDATION.md)
for acceptance scenarios and the distinction between tested fixtures and real Zoho verification.

## Storage and backups
Data defaults to the user's configuration directory (`%APPDATA%/Tempo` on Windows).
Demo uses its `demo/` subdirectory. SQLite `app.db` holds drafts, settings, and cached reference data.
Windows protects `credentials.bin` with the current user's DPAPI; other systems use a private 0600 file.
Tokens and client secrets never enter SQLite or the browser's local storage.

Use **Back up local data** to download a consistent SQLite snapshot. It includes work notes and cached
project/user metadata, and excludes OAuth credentials. To restore, stop the server and replace `app.db`
in a fresh data directory with the backup, then reconnect the same Zoho account and portal.
Never copy over a running SQLite database. OAuth credentials are tied to the Windows user and computer.

## Project documentation
- [Agent guide](AGENTS.md)
- [Product requirements](docs/REQUIREMENTS.md)
- [Architecture and implementation plan](docs/ARCHITECTURE.md)
- [Architecture diagrams](docs/ARCHITECTURE_DIAGRAMS.md)
- [Verification and validation](docs/VALIDATION.md)
- [Zoho API integration](docs/ZOHO.md)
