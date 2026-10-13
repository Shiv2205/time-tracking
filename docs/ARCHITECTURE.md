# Architecture and implementation plan

## Runtime
Vue 3, TypeScript, and Vite build the interface. Go's `net/http` serves its embedded assets and
same-origin JSON API. SQLite holds drafts and cached reference data. The executable needs no
Node installation at runtime. Development uses Vite's `/api` proxy to Go.

## Components and implementation sequence
1. Create documentation, Go domain/storage layers, and the Vue shell with accessible forms.
2. Implement OAuth setup, regional hosts, user/portal selection, and paginated reference reads.
3. Implement persisted draft CRUD, weekly totals, and existing-log reference view.
4. Implement reviewed submission, durable per-entry results, and uncertain-result reconciliation.
5. Implement project report preview/copy and isolated demo mode.
6. Run unit, integration, production build, and browser acceptance checks; record results.

## Storage and data flow
- Drafts use a local UUID, date, string project/task IDs, integer minutes, billing flag, notes,
  revision, status, submission attempt time, error, and optional remote log ID.
- Vue formats repeatable comment sections into the existing plain-text notes field. On draft edit,
  it parses complete sections from the end of the notes, leaving any earlier freeform text intact.
  Unchanged notes are submitted to draft storage verbatim. No schema or API payload change is needed.
- Cached catalogs retain names/status/ownership. Removed tasks remain visible for historical entries,
  but validation blocks new submission to a task no longer available.
- Existing logs are keyed by remote ID and date; refresh replaces a fetched week's cache atomically.
- Settings contain portal/user IDs, timezone, region, and sync time. Secrets live separately.
- Windows protects the secret file with DPAPI; other platforms use a private file with mode 0600.
- Database backup downloads contain drafts and cache only. Disconnect removes OAuth secrets and
  connection identity while retaining local drafts; reconnecting must use the same identity to submit them.

## API boundary
`/api/bootstrap` returns settings, connection status, catalog, and a per-process browser request token.
`/api/connection/*` configures OAuth, completes the callback, selects a portal, and disconnects.
`/api/sync` refreshes reference data and a selected week without writing to Zoho.
Upstream Zoho 5xx and transport failures return HTTP 502, upstream timeouts return 504, and
rate limits return 503. Refresh errors identify whether project, task, or time-log reads
failed while keeping the provider response body out of the browser.
`/api/drafts` provides revision-checked CRUD. `/api/week?start=YYYY-MM-DD` returns entries and totals.
`/api/submissions` accepts selected draft IDs and revisions; `/api/reconcile` checks uncertain results.
`/api/report` returns the plain-text draft report. `/api/backup` downloads a consistent SQLite snapshot.
Only the Go Zoho adapter knows external payload field names and regional endpoints.

## Submission safety
Submission serializes locally; persisted status changes to `submitting` before each remote POST.
Successful responses record the remote ID; definite validation failures return to editable `failed` state.
Transport errors, server errors, malformed success responses, or interrupted attempts become `uncertain`.
Uncertain entries cannot be edited, deleted, or submitted again. Reconciliation matches the user, task,
date, duration, billing flag, notes, and attempt creation window against fresh remote logs.
A unique candidate may be explicitly linked after inspection. With no candidate, a deliberate user
action can release the entry for retry after checking Zoho; ambiguity remains locked.
Local IDs, revisions, and transactions prevent double clicks or stale review screens from duplicating work.

## Local security and operation
Reject non-loopback Host values, cross-origin browser writes, and missing request tokens.
OAuth callbacks require an expiring state cookie and server-side verifier. Tokens are never returned
to Vue. Restrict request size, use HTTP deadlines, redact provider errors, and avoid logging notes.
Persist real data in the user's config/application-data directory, overrideable with `-data-dir`.
Demo mode uses its own subdirectory and never calls Zoho. See [setup](../README.md).
