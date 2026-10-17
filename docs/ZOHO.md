# Zoho Projects integration

## Sources
The current [Zoho Projects v3 reference](https://projects.zoho.com/api-docs) was read on 2026-10-11.
It documents regional `projects.zoho.*` hosts, Bearer authentication, ISO calendar dates, page-based
pagination, and v3.1 user endpoints. The old `/restapi/` examples and the
[Zoho ERP Projects documentation](https://www.zoho.com/erp/api/v3/projects/) are not this app's contract.

OAuth follows [Zoho's server application flow](https://www.zoho.com/developer/oauth/web-server-apps/overview.html),
with [offline authorization](https://www.zoho.com/developer/oauth/web-server-apps/get-authorization-code.html),
PKCE, an expiring state cookie, and [refresh-token exchange](https://www.zoho.com/developer/oauth/web-server-apps/refresh-access-token.html).
Profile lookup uses the Accounts `/oauth/user/info` endpoint and `AaaServer.profile.READ` scope;
the email is mapped to the portal-specific Projects user ID rather than assuming a global ZUID equals a ZPUID.

## Scopes
Request these explicitly, without ALL, UPDATE, or DELETE permissions:
- `ZohoProjects.portals.READ`
- `ZohoProjects.projects.READ`
- `ZohoProjects.tasks.READ`
- `ZohoProjects.users.READ`
- `ZohoProjects.timesheets.READ`
- `ZohoProjects.timesheets.CREATE`
- `AaaServer.profile.READ`

## Requests
- List portals: `GET /api/v3/portals`.
- Resolve user: `GET /api/v3.1/portal/{portal}/users/{email}`.
- List accessible projects: `GET /api/v3/portal/{portal}/projects`.
- List portal tasks: `GET /api/v3/portal/{portal}/tasks`; retain tasks belonging to active
  projects in the fetched project catalog.
- Read task logs: `GET /api/v3/portal/{portal}/timelogs`, using `view_type=customdate`,
  ISO `start_date`/`end_date`, `module={"type":"task"}`, and an `assignee` ZPUID filter.
- Create task log: `POST /api/v3/portal/{portal}/projects/{project}/log` with date,
  `bill_status`, `hours`, `notes`, `owner_zpuid`, `module={id,type:"task"}`, and `for_timer=false`.

Use `page` and `per_page=200`, stopping at `page_info.has_next_page=false`.
The portal-wide task list avoids project-specific task reads that returned HTTP 400 for some
projects in a live portal even though those projects appeared active in the project list.
Some documented responses are bare arrays; those use short-page termination.
The adapter decodes numbers as `json.Number` to preserve large IDs.
Time-log list responses group records by date under `time_logs[].log_details[]`.
User IDs are verified again locally to keep other people's logs out of the reference view.
Zoho notes can contain HTML. The adapter preserves their returned text in the cache; the browser
converts supported block and list markup to readable plain text for both the overview and reference view.
If Zoho returns HTTP 500 for the documented assignee filter, the adapter repeats that read
without the filter, keeping the week and task-module bounds. It checks owner IDs locally before
storing logs and uses the unfiltered form for later reads in the same process.

Hours are sent as `H:MM`, retaining exact minutes. Zoho's v3 creation reference describes hours
as a string and uses `01.00` in an example; the Projects
[time-log import guide](https://help.zoho.com/portal/en/kb/projects/timesheetsandtimelogs/manage-timelogs/articles/import-time-logs)
and older time-log reference document clock durations. Actual v3 acceptance of a non-whole-hour
entry remains part of the live check below; never silently convert or round a rejected duration.

## Failure behavior
Use bounded requests and honor cancellation. Refresh an expired token once on a definite 401;
otherwise never automatically retry a create request. A definite 400/403/429 returns an editable
failed draft. A per-endpoint cooldown honors Retry-After before another request is sent.
HTTP 5xx/408, transport errors, malformed success bodies, and missing remote IDs
are uncertain and stay locked. Display sanitized errors and rate-limit Retry-After information.

Reconciliation fetches fresh task logs, matches all submitted values plus the attempt window,
and excludes remote IDs already linked to another local entry. Zoho can augment notes, so an exact
match may be unavailable. The user must inspect Zoho before releasing a zero-match entry for retry.
Multiple matches stay locked for resolution in Zoho.

## Live validation still required
No real credentials or company portal are bundled. Mocked tests verify the wire behavior and demo
tests verify the UI; neither confirms the user's portal settings or real OAuth configuration.
On 2026-10-11, one live US-portal read returned HTTP 500 with the documented assignee filter and
HTTP 200 for the same week without it. This verifies the fallback condition only; the full live
refresh and a time-log write still require acceptance checks.
Check OAuth client/redirect support, region, user mapping, task permissions, a non-whole-hour duration,
notes, and returned log ID against one deliberately submitted test entry in the user's portal.
Portals with mandatory custom time-log fields may reject this first version's basic form;
required custom-field input is not included in the current requirements.
