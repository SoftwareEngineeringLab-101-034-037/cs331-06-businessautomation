# Integrations Service Refactor (Phased)

This service now follows an integrations-platform structure while keeping Google Forms behavior intact.

## What is completed

### Phase 1: Provider abstraction

- Added provider contract and registry:
  - `internal/integrations/provider.go`
- Added Google Forms provider adapter:
  - `internal/providers/googleforms/provider.go`
- Server bootstrap now registers providers once and resolves by service id.

### Phase 2: Provider-isolated watch and poller state

- Added `provider` to watch model (`FormWatch`).
- Storage now supports provider-aware methods:
  - `ListWatchesByProvider`
  - `ListActiveWatchesByProvider`
- Backward compatibility for old data is preserved:
  - Existing records without `provider` are treated as `google_forms`.
- Poller now only loads watches for the Google Forms provider.

### Phase 3: Integrations-first API surface (with backward compatibility)

- Existing routes are still supported:
  - `/integration/status`
  - `/integration/accounts`
  - `/integration/accounts/{account_id}`
- New routes are available:
  - `GET /integrations/providers`
  - `GET /integrations/{service}/status`
  - `GET /integrations/{service}/accounts`
  - `DELETE /integrations/{service}/accounts/{account_id}`
- Service id aliases are normalized (`google-forms` and `google_forms` both resolve correctly).

## Why this scales better

- New integration providers can be added as isolated modules under `internal/providers/<provider-name>`.
- Shared platform concerns (org auth checks, route orchestration, storage patterns) stay centralized.
- Provider-specific auth and polling logic are isolated to avoid cross-provider regressions.

### Phase 4: Provider-local HTTP handlers

- Google Forms HTTP handlers now live in `internal/providers/googleforms/httpapi/handler.go`.
- `internal/api` is now a dispatch layer that routes requests to provider-local handlers.

### Phase 5: Capability contracts

- Added capability interfaces in `internal/integrations/provider.go`:
  - `TriggerSource`
  - `ActionExecutor`
  - `WebhookSource`
- Poller trigger routing now consumes `TriggerSource.TriggerEventPath()` via provider contract resolution at bootstrap.

### Phase 6: Service rename migration

- Service path was renamed from `backend/google-forms` to `backend/integrations`.
- Go module path was updated to `github.com/example/business-automation/backend/integrations`.
- Workspace task labels/paths were updated to `Integrations Service` and `backend/integrations/cmd/server`.
- Frontend now supports `NEXT_PUBLIC_INTEGRATIONS_API` (with fallback to `NEXT_PUBLIC_GOOGLE_FORMS_API` during migration).

## Optional next hardening

- Add explicit provider-specific watch models for non-form integrations as they are introduced.
- Add contract tests per provider capability to enforce isolation guarantees.
