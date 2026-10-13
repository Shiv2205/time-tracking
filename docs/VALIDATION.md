# Verification and validation

## Automated gates
- `go test ./...`: domain duration/date validation, calendar week boundaries including DST,
  draft/report totals, SQLite persistence, revisions, catalog membership, and submission recovery.
- `go vet ./...`: Go static checks.
- `npm test --prefix web`: duration and date helpers and report-facing frontend behavior.
  Comment tests cover section formatting, reopening, appending after freeform notes, and validation.
- `npm run build --prefix web`: TypeScript checking and production Vue build.
- `npm run test:e2e --prefix web`: start isolated normal and demo servers and test first-run setup, draft CRUD,
  weekly review selection, copyable report, explicit submission, reference logs, and mobile layout.
- Check `AGENTS.md` has fewer than 100 lines and referenced local Markdown files exist.
- `npm run format:check --prefix web`: keep Vue, TypeScript, and CSS source consistently formatted.

## External integration scenarios
Use an HTTP test server with Zoho-shaped fixtures to verify OAuth exchange/refresh,
data-center host selection, current-user mapping, pagination, date/duration serialization,
read filtering, 401 handling, rate-limit handling, and sanitized provider errors.
Verify partial success never resubmits confirmed entries. Interrupt an attempted POST and ensure
the entry stays locked as uncertain across restart. Test zero, one, and multiple reconciliation matches.

## Browser acceptance
1. A fresh normal run shows connection setup; a demo run is visibly labeled.
2. Add a dated task entry with notes, edit it, duplicate it, and delete the duplicate.
3. Reload and restart; remaining drafts and minute totals persist.
4. Switch week and project filters; excluded drafts do not affect visible review totals.
5. Generate a project report; dates, task names, notes, and total match current drafts only.
6. Saving, refreshing, and report generation perform no Zoho writes.
7. Review selected drafts, explicitly submit, and see each result with its remote ID.
8. Confirm submitted entries disappear from draft totals/reports and remain visible in history.
9. Inspect existing Zoho logs separately; they never inflate local draft totals.
10. Keyboard navigation, accessible labels, clear feedback, and a usable narrow viewport work.
11. Add a formatted comment, edit its brief after reopening the draft, append another comment,
    and confirm the plain-text layout survives reload and appears in the project report.

## Live acceptance prerequisite
Real OAuth and write acceptance require the user's client credentials and a permitted Zoho Projects
portal. Do not claim live integration validation from mocked or demo results. Start with one deliberate
test entry and verify its user, task, date, duration, billing flag, and notes directly in Zoho.

## Results
Verified on 2026-10-11 using Windows, Go 1.27.2, Node 24.13.1, and Playwright Chromium.

| Check | Result |
| --- | --- |
| Go tests | 25 passing before the portal-wide task change; its targeted catalog test passed afterward. A full rerun is pending because local HTTP test servers cannot accept connections in the restricted sandbox. |
| Go static checks | `go vet ./...` passed |
| Frontend tests | 4 Vitest tests passed |
| TypeScript and Vue production build | Passed; compiled assets embedded in the Windows executable |
| Browser acceptance | 3 scenarios passed: normal setup/secret redaction, complete draft/report/submission flow, and populated mobile/week navigation |
| Source formatting | Prettier and gofmt applied; formatting check passed |
| Dependencies | Go module graph tidied; npm installation audit reported zero known vulnerabilities |
| Documentation | `AGENTS.md` has 49 lines; all its referenced local documents exist |
| Visual inspection | Desktop (1440px) and mobile (390px) screenshots inspected; mobile has no horizontal overflow |

The browser scenario verifies report contents and clipboard copy, persistence after reload,
explicit submission confirmation, submitted history, and separation of existing Zoho reference logs.
SQLite tests verify restart persistence and recovery of interrupted submissions.
Mock integration tests verify OAuth/PKCE, refresh on 401, pagination, large IDs, grouped time-log responses,
payload fields, rate-limit cooldown, partial failure, exact reconciliation, and ambiguous matches.
The catalog test verifies portal-wide task pagination and association with active projects.
An HTTP handler test verifies that Zoho HTTP 500 responses during either catalog or time-log refresh
produce local HTTP 502 responses with the failing refresh stage identified.
An adapter test verifies that a 500 from the filtered time-log list falls back to a date-bounded,
task-only request, then retains only the current user's logs.

Live OAuth and real Zoho reads/writes have **not** been exercised with the user's credentials.
Complete the live acceptance described above and in [Zoho integration](ZOHO.md) before relying on
the app for a real weekly submission, especially the duration format and any required custom fields.

## Time-entry comment change (2026-10-13)
The formatted comment feature adds no new Zoho API field. Its generated text uses the existing
notes field. Frontend unit tests (7), the production Vue build, Go tests, Go vet, and the source
formatting check passed on Windows. All three browser acceptance scenarios passed with the updated
executable, including comment editing, appending, reload, report rendering, and narrow layout.
Submitted entries remain read-only. Live Zoho submission of these notes still requires the
credentialed acceptance described above.
