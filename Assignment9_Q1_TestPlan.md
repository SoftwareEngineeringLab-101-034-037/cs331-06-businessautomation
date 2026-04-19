# CS 331 Assignment 9 - Q1(a) Test Plan

## 1. Objective of Testing
The objective is to verify that the Auth service Employee and Invitation Management module is functionally correct, stable under normal and negative inputs, and safe for organization-level onboarding operations.

Primary goals:
- Validate department, invitation, and org-settings workflows.
- Validate API error handling for invalid inputs and duplicate operations.
- Validate service behavior for business constraints (duplicate prevention, revocation flow, assignment constraints).
- Validate data consistency between handler responses and persisted database state.

## 2. Scope
In scope module: Employee and Invitation Management in Auth service.

In-scope features:
- Organization settings read and update.
- Department create, list, update, delete.
- Single employee invite, bulk invite.
- Invitation list and revoke.
- Employee list and delete.

Primary code references:
- backend/auth/internal/handler/employee_handler.go
- backend/auth/internal/service/employee_service.go
- backend/auth/internal/service/invite.go
- backend/auth/internal/service/org_settings.go

## 3. Types of Testing
- Unit testing:
  - Service-layer and handler-layer function behavior.
- Integration testing:
  - Handler + service + database flow using test DB setup.
- System/API sanity checks:
  - Endpoint-level behavior under realistic request patterns.
- Negative testing:
  - Missing fields, invalid emails, duplicate requests, not-found states.

## 4. Tools
- Go test framework.
- Gin HTTP test utilities (httptest).
- SQLite in-memory database for isolated tests.
- PowerShell terminal for command execution and log capture.
- Log files as evidence:
  - Assignment9_TestExecution.log
  - Assignment9_DefectEvidence.log

## 5. Entry Criteria
- Project dependencies installed successfully.
- Auth service test suites compile and execute.
- Required test files available.
- Database schema available through test setup/migrations.

## 6. Exit Criteria
- At least 8 test cases designed and executed for the selected major module.
- Each test case contains expected output, actual output, and status.
- Evidence logs captured and attached.
- At least 3 reproducible defects identified with severity and suggested fix.

## 7. Test Data and Environment
Environment:
- OS: Windows
- Service: backend/auth
- Command runtime: PowerShell

Representative data:
- Org IDs: org_1, org_2
- Department names: Engineering, Finance
- Invitation emails: new.user@example.com, pending@example.com
- Roles: member, admin

## 8. Risks and Mitigation
- Risk: External Clerk API dependence can introduce non-determinism.
- Mitigation: Use mocked/stubbed behavior in tests and focus on local DB side effects.

- Risk: Case sensitivity and input normalization issues may pass happy-path tests.
- Mitigation: Add targeted exploratory edge-case tests and log failures.
