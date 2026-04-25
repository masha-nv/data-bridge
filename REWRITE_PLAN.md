# MARx Desktop Rewrite Plan

## Goal

Rebuild the richer `marx-maintainer-utils` feature set on top of the existing `data-bridge` Electron + Angular + Go structure, while replacing the current UI with a modern Angular Material desktop shell using signals.

The new app must:

- remain an Electron desktop application
- use `data-bridge` as the host repo and runtime shell
- use `marx-maintainer-utils` as the functional source of truth
- use a local SQLite demo database during development
- use a mock login first, then support a simple switch to real DB credentials later
- keep currently stubbed MARx screens visible but disabled in the first pass

## Locked Decisions

- Host repo: `data-bridge`
- Functional source of truth: `marx-maintainer-utils`
- Desktop runtime: Electron
- Frontend stack: Angular + Angular Material + signals
- UI shell: left navigation desktop layout
- Development database: SQLite demo DB
- Initial auth flow: mock login
- Scope: MARx functionality only
- Stubbed MARx screens: visible but disabled for now
- Workflow: one file at a time, with approval before every edit or file creation

## Execution Protocol

1. No implementation starts for a file until the file-level plan is approved.
2. Work proceeds one file at a time.
3. Before touching a file, document:
   - the file to change
   - why it is next
   - exact changes planned
   - expected behavior after that file alone
   - risks or alternatives
4. Wait for approval.
5. Edit only that file.
6. After the edit, summarize what changed, what to verify, and propose the next file.

## Target Feature Set

### Implemented in the first build

- Login shell using mock authentication
- Descriptions lookup
- Beneficiary details lookup
- DevOps active jobs view
- DevOps completed jobs view
- DevOps restart failed job
- DevOps mark job complete
- SQL runner
- Version and app bootstrap/status support

### Visible but disabled in the first build

- Bene Download
- Change Password
- Submit Batch Job
- Tester Utilities

## High-Level Implementation Order

1. Backend foundation and route/bootstrap cleanup
2. Demo database schema and seed data
3. Backend contracts for auth, bootstrap, descriptions, beneficiary lookup, jobs, and SQL execution
4. [in progress] Angular desktop shell and routing
5. Angular shared state, auth, environment, and API services
6. Feature slices in this order:
   - Descriptions
   - Bene Details
   - DevOps read-only tables
   - DevOps actions
   - SQL runner
   - disabled placeholder routes
7. Electron polish: menu, version/about, backend lifecycle, startup checks
8. Tests and developer workflow improvements
9. Real DB adapter/config switch path

## Planned File Sequence

### Backend first

1. [done] `backend/main.go`
2. [done] backend bootstrap/config helpers
3. [done] backend auth/session files
4. [done] backend demo DB bootstrap files (readiness, schema, seed data)
5. [in progress] backend feature handler/service/repository files

### Frontend shell next

1. [done] `frontend/frontend/src/app/app.routes.ts`
2. [done] `frontend/frontend/src/app/app.component.ts`
3. [done] new shell/login/layout files
4. shared app/auth/environment services
5. feature services and routed feature modules

### Electron and tests last

1. [done] `electron/main.js`
2. [done] supporting build/run config updates
3. backend tests
4. frontend unit and smoke tests

## Current Working State

- Electron desktop startup now works with the user-local Go installation.
- The demo backend starts successfully in Electron and serves the local API on port `8080`.
- The login screen is wired to backend auth in both Angular browser dev mode and Electron file-loaded mode.
- Demo credentials now authenticate successfully and route into the app.
- Shared frontend environment/bootstrap state now loads from the backend status endpoint instead of hard-coded route-local values.
- Shared frontend auth/session state now lives in a dedicated Angular service instead of the login component.
- The Descriptions feature now has a real Angular screen with environment/type/code inputs, backend lookup wiring, and result display.
- The Beneficiary Details feature now has a real Angular screen backed by the MARx beneficiary lookup endpoint.
- The DevOps feature now has active/completed job views plus restart and mark-complete actions wired to the demo backend.
- The SQL runner now has a real Angular screen for running read-only SQL against the demo backend.
- Placeholder MARx screens are now visible in app navigation and route to disabled placeholder pages instead of empty child routes.
- Post-login navigation now lands on the real Descriptions screen instead of the earlier temporary placeholder.
- The Angular build still emits a non-blocking bundle budget warning.

## Verification Standard

After each major slice, verify that:

1. the app still runs locally in Electron
2. the backend builds successfully
3. the frontend remains runnable
4. the implemented feature works against the SQLite demo DB
5. later switching to real DB mode requires configuration and adapter changes only

## Notes

- Preserve the app in a runnable state throughout the rewrite.
- Prefer backend and frontend seams over large rewrites inside existing monolith files.
- Do not reuse weaker `data-bridge` business logic except where it helps with infrastructure.
- Preserve `marx-maintainer-utils` backend semantics as much as possible; avoid unnecessary backend redesign.
- Backend changes should mainly support SQLite demo-mode data for development and keep a clean adapter path for switching to real databases later.
- A swappable backend DB adapter seam is now in place; future real DB work should extend that seam rather than changing feature contracts or UI behavior.