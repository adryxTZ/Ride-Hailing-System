# Spec: configurable approval workflows (and staff with several roles)

Written 2026-10-24. Owner request (Kalen): "add a dynamic configurable workflow feature to enable flow editing on approvals etc.; also add assign role on the user page, a user can have multiple roles". Migration `00044`. Builds on staff roles and permissions (`00029`), the audit log, and the modules whose actions it gates (pricing profiles, refunds, payouts, privacy, regulator, staff roles, branding).

## 1. Goal
Sensitive staff actions can need someone else's approval, configured as data in the dashboard. A workflow is per action: on or off, when it applies (amount range, countries), how long a request may wait, and ordered steps. Each step needs approvals from people holding listed roles (or a permission), as "any of" (N approvals) or "one from each role". The requester never approves their own request. When a workflow holds a change, the endpoint applies nothing and answers **202 `{approval_request}`**; on the last approval the change runs once, as the requester of record, with "approved by …" in its audit reason.

**Default: every workflow is off**, so behaviour is unchanged until an admin turns one on.

## 2. Approvable actions (`internal/approvals/actions`)
| Key | Gated endpoint | Condition | Country |
|---|---|---|---|
| `pricing.profile_publish` | `POST /admin/pricing/profiles/{id}/versions` | — | profile's |
| `pricing.profile_status` | `POST /admin/pricing/profiles/{id}/status` | — | profile's |
| `pricing.profile_default` | `POST /admin/pricing/profiles/{id}/make-default` | — | profile's |
| `pricing.profile_assign` | `POST /admin/pricing/profiles/{id}/assignments` | — | profile's |
| `trips.refund` | `POST /admin/trips/{id}/refunds` | amount | trip's |
| `payouts.approve` | `POST /admin/finance/payouts/{id}/approve` | amount | first country of the payout's currency in the caller's scope |
| `privacy.delete` | `POST /admin/privacy/requests/{id}/approve` (DELETE requests only; exports never) | — | request's |
| `compliance.regulator` | `PATCH /admin/compliance/regulator/{country}` | — | path |
| `staff.roles` | `PUT /admin/staff` (only when the roles actually change) | — | none |
| `branding.publish` | `POST /admin/branding/themes/{id}/publish` | — | none |

Each action has a **describer** (the readable diff approvers see, computed when the request is made; it also refuses a change that cannot apply, e.g. a refund larger than what is left) and an **executor** that calls the owning service with the stored payload. Adding an action = one registration in `actions/` plus one `sentForApproval` call in its handler.

## 3. Gate API (backend)
- `approvals.Service.Submit(ctx, Submission{Action, RequesterID, Reason, Country, Amount, Target, Payload}) (*Request, error)`: `nil` = workflow off or conditions do not match, apply the change as before; a request = held. The same pending change asked again returns the same request (unique pending payload hash), so retries are safe. A held change needs a reason (`REASON_REQUIRED`).
- `approvals.Service.Enabled(ctx, action)`: cached check so gates skip extra lookups when off.
- HTTP: `sentForApproval(w, req, deps, Submission) bool` in `kernel/httpx/approval_gate.go`, called after the handler's own checks; true = 202 written, stop.
- Workflows are cached in-process, versioned by `config_versions('approvals')` (5 s check); a save in this process is seen at once (`cache.Versioned.Stale`).

## 4. Rules
- Conditions: `amount_min` ≤ amount ≤ `amount_max` (inclusive; only on actions with an amount), `countries` (only on country-bound actions). Amounts compare in the request's own currency; add a country condition where currencies differ.
- Steps run in order. ANY: `approvals` distinct people holding any listed role or the permission (`*` holds every permission; a role name alone does not). ALL: one distinct person per listed role (and one with the permission if set).
- A person decides a request at most once (across steps). The requester can never approve or reject (service check + database trigger); they can withdraw while pending.
- Reject needs a note and ends the request. Requests expire after `expire_hours` (1–720, default 72): a River job every 5 minutes, and on the spot when someone decides an overdue one.
- A request keeps the workflow definition and version it started under; saves are versioned (append-only history) and audited.
- Country-limited staff see and decide only requests in their countries (a region limit counts as its country); country-less requests (branding, staff roles) need an unlimited viewer. Everyone always sees their own.
- Execution: `APPROVED → EXECUTING → EXECUTED | FAILED`, claimed with a conditional update so it runs once; refunds also carry the idempotency key `approval:<id>`. A failure stores the error code on the request (e.g. `REFUND_TOO_LARGE`) and is audited.
- A held staff-role change stores no email or password: set them after approval (an edit that keeps the same roles is not gated).

## 5. API
- `GET /admin/approvals/requests?box=pending|mine|done`, `GET …/requests/summary` (nav badge), `GET …/requests/{id}`, `POST …/{id}/approve|reject|cancel` `{note}`: any staff (decisions checked per step).
- `GET /admin/approvals/workflows`, `GET|PUT /admin/approvals/workflows/{action}`: permission `approvals.manage` (new; Super Admin has all; granted to Operations by the migration).
- Gated routes document `202` as `#/components/responses/SentForApproval` (`ApprovalAccepted`).
- Errors: `APPROVAL_NOT_FOUND`, `APPROVAL_UNKNOWN_ACTION`, `APPROVAL_NOT_PENDING`, `APPROVAL_EXPIRED`, `APPROVAL_SELF`, `APPROVAL_NOT_APPROVER`, `APPROVAL_ALREADY_DECIDED`, `APPROVAL_NOT_REQUESTER`, `APPROVAL_NOTE_REQUIRED`, `APPROVAL_WORKFLOW_INVALID` (`details.problems`), stored `APPROVAL_ACTION_UNAVAILABLE`.

## 6. Data (`00044_approvals.sql`)
`approval_workflows` (current per action), `approval_workflow_versions` (append-only), `approval_requests` (payload, summary, workflow snapshot, status, expiry; unique pending payload hash), `approval_decisions` (append-only, one per person per request, not the requester). Down drops them and removes `approvals.manage` from roles.

## 7. Admin
- Operate › **Approvals** inbox (Waiting / Mine / Done) with a nav badge of requests the person can decide; a request page (`/approvals/{id}`) with the field diff, the request, the decision (approve / reject with note, or withdraw) and the steps with who approved.
- Settings › **Approval workflows**: list of actions, and an editor page per action (toggle, amount and country conditions, expiry, steps: add, reorder, remove, roles, permission, mode, approvals) with the version history.
- Shared 202 handling: `lib/approval.ts` `unwrapGated` (used by every gated hook) + `ApprovalNotice` in the shell ("Sent for approval · View request").
- Roles: one `RolePicker` for Add staff, Edit roles (row button no longer opens the person page by accident) and Staff detail; a **Dashboard roles** card on rider and driver pages gives an existing user staff roles (several at once). Role edits now send the dialog's reason.

## 8. Acceptance
- Go tests (`internal/approvals`): conditions, step rules (ANY/ALL/permission), definition validation, diff; DB-backed: off by default, threshold and country, self-approval refused (also by the database), country-limited viewer, executor runs once with "approved by", steps in order and distinct people, reject needs a note, pending request keeps its version, expiry, duplicate pending change, cancel by requester only.
- `apps/tools/e2e-approvals.mjs` prints PASS lines: second staff created via API; multiple roles; refund below threshold 201; at threshold 202 with nothing refunded; self-approval 403; Finance approval executes; audit names the approver and the requester stays the actor; ledger owes the rider and verifies; decided request 409.

## Factory log
- 2026-10-24: built on branch `worktree-agent-a9b6376126222af75` (migration 00044, isolated DB `nexa_workflow`, API :8092, admin :3022).
  - Backend: `internal/approvals` (+ `actions/`), gate in 10 handlers, `identity/grants.go`, `approvals.manage`, River expiry job, contract + i18n, both TS clients regenerated.
  - Verified: gofmt, go vet, golangci-lint (0 issues), brand-check, `go test ./...` against `nexa_workflow`, redocly lint (valid; 54 warnings, same as before), admin tsc + Biome, e2e PASS, Down migration checked in a rolled-back transaction.
  - Screenshots: `apps/tools/shot-approvals.mjs` (workflow list and editor, inbox, request as requester and approver, staff edit, "Sent for approval" notice, rider roles card); read and fixed (versions list instead of a cramped table, thousands separators in titles).
  - Not done: no notification (push/email) to approvers when a request arrives (the badge refreshes every 30 s); a request whose executor crashes mid-run stays `EXECUTING` (visible, not retried automatically).
