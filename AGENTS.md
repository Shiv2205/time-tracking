# Agent guide

## Read first
- [Product requirements](docs/REQUIREMENTS.md) are the source of truth for behavior.
- [Architecture and implementation plan](docs/ARCHITECTURE.md) explain the design and data flow.
- [Validation](docs/VALIDATION.md) defines checks, acceptance criteria, and recorded results.
- [Setup and operation](README.md) explain local development and Zoho configuration.
- [Zoho integration](docs/ZOHO.md) records the external API contract and limitations.

## Project conventions
- Build a local, single-user app: Vue 3 + TypeScript, Go, and SQLite.
- Use Vue Composition API and `<script setup lang="ts">`.
- Keep UI code in `web/`, server code in `internal/`, and the executable in `cmd/timetracker/`.
- Use Go `net/http`, explicit JSON types, context cancellation, and bounded HTTP requests.
- Store durations as integer minutes, calendar dates as `YYYY-MM-DD`, and Zoho IDs as strings.
- Preserve draft data on remote failures. Never submit time on save, refresh, startup, or a schedule.
- Confirm submission through the review screen. Record each remote result independently.
- Keep uncertain submissions locked until reconciled; never blindly retry an uncertain POST.
- Keep credentials out of JSON API responses, browser storage, logs, fixtures, and database backups.
- Bind to loopback; enforce Host, Origin, and a request token on browser writes.
- Keep demo storage separate from real storage and identify demo mode visibly.
- Update the linked documents whenever behavior, API contracts, or validation changes.
- Do not add real credentials or company data to the repository.

## Commands
- Install frontend dependencies: `npm ci --prefix web`.
- Develop frontend: `npm run dev --prefix web` (proxies `/api` to Go).
- Develop backend: `go run ./cmd/timetracker`.
- Demo backend: `go run ./cmd/timetracker -demo`.
- Validate backend: `go test ./...` and `go vet ./...`.
- Validate frontend: `npm test --prefix web` and `npm run build --prefix web`.
- Keep frontend source readable: `npm run format --prefix web`; check with `npm run format:check --prefix web`.
- Validate browser workflow: `npm run test:e2e --prefix web` (see validation prerequisites).
- Build release after frontend build: `go build -o dist/timetracker.exe ./cmd/timetracker`.
- If Go is not on PATH, the workspace toolchain is `.tools/go/bin/go.exe`.
- Use `gofmt` on changed Go source. Commit dependency lockfiles; do not commit generated assets.

## Official documentation
These links were checked through Context7 on 2026-10-11. Re-query Context7 for unfamiliar APIs
or dependency upgrades; use the official source documentation it returns.
- [Go documentation](https://go.dev/doc/) and [language specification](https://go.dev/ref/spec).
- [Go HTTP server/client](https://pkg.go.dev/net/http), [HTTP testing](https://pkg.go.dev/net/http/httptest),
  and [embedded files](https://pkg.go.dev/embed).
- [Vue quick start](https://vuejs.org/guide/quick-start.html),
  [TypeScript](https://vuejs.org/guide/typescript/overview.html),
  [Composition API](https://vuejs.org/guide/extras/composition-api-faq.html),
  and [forms](https://vuejs.org/guide/essentials/forms.html).
- [Vite guide](https://vite.dev/guide/) and [Zoho Projects v3](https://projects.zoho.com/api-docs).
- Context7 library IDs: `/websites/go_dev_doc` and `/websites/vuejs`.
