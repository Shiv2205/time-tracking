# Product requirements

## Intended use
A personal web application that makes weekly work time easy to prepare for Zoho Projects.
The first version runs on the user's computer for one Zoho account and one selected portal.
The target integration is **Zoho Projects v3**, not the separate Zoho ERP Projects module.

## Required behavior
1. Provide an intuitive, responsive interface for adding, editing, duplicating, and deleting local time drafts.
2. Connect to Zoho using OAuth and retrieve projects, tasks, the current user, and that user's existing time logs.
3. Every entry belongs to a project and a Zoho task. Only task logs are supported.
4. An entry contains a calendar date, project, task, positive duration in whole minutes,
   billable status, and optional work notes. Saving affects local storage only.
5. Keep drafts across app and computer restarts. Show save and error feedback.
6. Default to the current Monday–Sunday week in the browser's local timezone; allow other weeks.
7. Weekly review contains local drafts only, with totals by date, project, and the full week.
8. Submit only selected entries after explicit review and confirmation. Display per-entry success,
   failure, and uncertain status. Submitted entries become read-only and retain their Zoho ID.
9. Show existing Zoho logs in a separate reference view. Exclude them from draft review totals.
10. Generate a copyable plain-text report for a selected project and week from current drafts only.
    Include dates, tasks, durations, notes, and project total. Copy it before submitting those drafts.
11. Allow refreshing Zoho data and show connection status and the last successful refresh time.
12. Keep a clearly labeled demo mode with separate storage and simulated submission.

## Defaults and boundaries
- Local-only server, bound to `127.0.0.1:8080`; no multi-user account system.
- A task is required; general logs, issue logs, timers, and project/task creation are excluded.
- Time entry accepts `H:MM` or decimal hours and stores integer minutes without floating-point totals.
- Billable status is explicitly visible and editable; new drafts default to non-billable.
- Notes are plain text, preserved verbatim, with Zoho's documented 10,000-character maximum.
- Reports are generated deterministically from user notes; no AI rewriting or email delivery.
- Submitted Zoho entries are viewed, not edited or deleted, through this application.
- Creating OAuth credentials and granting API access are setup prerequisites.
