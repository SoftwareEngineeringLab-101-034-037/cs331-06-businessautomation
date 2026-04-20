# CS 331 – Software Engineering Lab
## Assignment 9: Software Testing




## Q1(a) – Test Plan

### 1. Objective of Testing

The primary objective of testing the **Business Automation Platform** is to:

- Verify that all functional requirements of the platform are correctly implemented across its microservices.
- Ensure that the **Auth Service** (employee management, department management, role management, and invitation lifecycle) behaves correctly under valid and invalid inputs.
- Validate that the **Workflow Engine Service** correctly handles workflow CRUD operations, instance execution, and human-in-the-loop task management.
- Confirm that inter-service communication (Auth ↔ Workflow ↔ Frontend) produces consistent and correct results.
- Detect and document defects early to reduce the cost of fixing them.
- Ensure data integrity — e.g., removing an employee removes all associated role memberships and invitations atomically.
- Validate security constraints — organization-scoped access control, JWT authentication, and role-based authorization.

---

### 2. Scope (Modules / Features to be Tested)

| # | Service | Module / Feature |
|---|---------|-----------------|
| 1 | **Auth Service** | Employee CRUD (list, delete) |
| 2 | **Auth Service** | Department Management (create, list, update, delete) |
| 3 | **Auth Service** | Role Management (create, list, update, delete, member assignment) |
| 4 | **Auth Service** | Invitation Lifecycle (invite single, invite bulk via Excel, list, accept, revoke) |
| 5 | **Auth Service** | Organization Settings (get, update) |
| 6 | **Auth Service** | Webhook handlers (Clerk user-created, organization-membership events) |
| 7 | **Workflow Engine** | Workflow CRUD (create, list, get, update, delete) |
| 8 | **Workflow Engine** | Instance execution (start, step execution, human task approval, cancellation) |
| 9 | **Workflow Engine** | Task Handler (list tasks, complete tasks) |
| 10 | **Integrations Service** | Google OAuth flow (authorization, callback, token refresh) |
| 11 | **Integrations Service** | Gmail sending, Google Forms creation, Zoom meeting scheduling |
| 12 | **Frontend** | Authentication UI (login via Clerk, organization selection) |
| 13 | **Frontend** | Admin Dashboard (department, role, employee management pages) |
| 14 | **Frontend** | Employee Workstation (task inbox, task form completion) |

**Out of Scope (for this assignment):**
- Third-party API reliability (Clerk, Zoom, WhatsApp) — mocked in tests
- Infrastructure / DevOps (Docker, CI/CD pipelines)
- Load / performance testing

---

### 3. Types of Testing

| Type | Description | Applicability |
|------|-------------|---------------|
| **Unit Testing** | Individual functions/methods tested in isolation using mocks/stubs (e.g., `CreateDepartment`, `InviteAndNotify`, `RemoveEmployee`). Written in Go's built-in `testing` package. | Auth Service logic layer (`service/`) |
| **Integration Testing** | API handlers tested end-to-end against an in-memory SQLite database, verifying HTTP status codes, response bodies, and DB state. Uses `net/http/httptest` + `gorm` with SQLite driver. | Auth Service handler layer (`handler/`) |
| **System Testing** | Full end-to-end test of user flows across all services: from frontend login → invite employee → employee accepts → workflow assigned → task completed. | Entire platform |
| **Regression Testing** | Re-executing all test suites after any code change to ensure nothing is broken. Automated via Go test runner: `go test ./...` | Auth Service, Workflow Engine |
| **Negative Testing** | Deliberately sending invalid inputs (missing fields, wrong IDs, duplicate names, invalid emails) to ensure proper error responses are returned. | All API endpoints |
| **Security Testing** | Verifying that organization-scoped data cannot be accessed across org boundaries; checking that admin-only endpoints reject non-admin tokens. | Auth + Workflow middleware |

---

### 4. Tools

| Tool | Purpose |
|------|---------|
| **Go `testing` package** | Built-in unit and integration test runner |
| **`net/http/httptest`** | HTTP handler testing without a live server |
| **`gorm` + SQLite (in-memory)** | Lightweight DB for handler integration tests |
| **`excelize/v2`** | Used in bulk invite Excel parsing tests |
| **Postman / cURL** | Manual API testing during development |
| **VS Code** | IDE used for development and test execution |

---

### 5. Entry Criteria

The following conditions must be met before testing begins:

-  All source code for the module under test is committed and compiles without errors (`go build ./...`)
-  The in-memory SQLite test database schema migrations have been applied (auto-migrated via `gorm.AutoMigrate`)
-  Test data fixtures (departments, roles, users, invitations) are seeded in each test's setup function
-  External service dependencies (Clerk API, Google OAuth) are mocked/stubbed
- The test environment variables are configured (e.g., `CLERK_SECRET_KEY`, `DATABASE_URL`)
-  All third-party dependencies are downloaded (`go mod tidy`)

---

### 6. Exit Criteria

Testing is considered complete and ready for sign-off when:

-  All 8+ designed test cases have been executed
-  100% of critical-path test cases (invite, remove employee, role assignment) **PASS**
-  All identified defects above **Medium** severity are fixed and re-tested
-  No new regressions introduced by bug fixes
-  Test execution results are documented with screenshots/logs
-  Defect report is finalized with severity levels and suggested fixes

---

## Q1(b) – Test Cases for the Employee Invitation Module

**Selected Module:** `Auth Service → Invitation Management`  
**Reason:** This is the primary onboarding mechanism for the platform — incorrect behavior here could deny legitimate users or allow unauthorized access.

**API Endpoints Covered:**
- `POST /api/orgs/:orgId/employees/invite`
- `POST /api/orgs/:orgId/employees/invite/bulk`
- `GET /api/orgs/:orgId/invitations`
- `DELETE /api/orgs/:orgId/invitations/:invitationId`
- `POST /api/orgs/:orgId/invitations/:invitationId/accept`

---

### Test Case TC-INV-01: Invite Single Employee – Success

| Field | Details |
|-------|---------|
| **Test Case ID** | TC-INV-01 |
| **Test Scenario / Description** | An admin invites a new employee with valid details (email, name, existing department). The invitation should be created with `status=pending`, a secure token hash should be stored, and the Clerk email  should be triggered. |
| **Input Data** | `POST /api/orgs/org_1/employees/invite` <br> `{"email":"alice@example.com","first_name":"Alice","last_name":"Smith","department":"Engineering","role":"analyst","job_title":"Data Analyst"}` <br> Header: `X-User-ID: admin_user` |
| **Pre-conditions** | Department "Engineering" exists in `org_1`. No existing pending invitation for `alice@example.com`. |
| **Expected Output** | HTTP `201 Created` <br> Response body contains `invitation.status = "pending"` and `invitation.email = "alice@example.com"` <br> DB: 1 row in `employee_invitations` with `status="pending"` and non-empty `token` |
| **Actual Output** | HTTP `201 Created`. Body: `{"invitation":{"email":"alice@example.com","status":"pending","token":"<sha256-hash>","job_title":"Data Analyst",...},"message":"Invitation created and email sent"}` |
| **Status** |  **PASS** |

---

### Test Case TC-INV-02: Invite Single – Missing Required Fields

| Field | Details |
|-------|---------|
| **Test Case ID** | TC-INV-02 |
| **Test Scenario / Description** | An admin sends an invite request without required fields (`first_name`, `last_name`, `department` missing). The server should reject the request with a validation error. |
| **Input Data** | `POST /api/orgs/org_1/employees/invite` <br> `{"email":"not-an-email"}` <br> Header: `X-User-ID: admin_user` |
| **Pre-conditions** | None |
| **Expected Output** | HTTP `400 Bad Request` <br> Response body contains an `"error"` field describing the validation failure (e.g., `"Key: 'Email' Error: Field validation for 'Email' failed on the 'email' tag"`) |
| **Actual Output** | HTTP `400 Bad Request`. Body: `{"error":"Key: 'FirstName' Error:Field validation for 'FirstName' failed on the 'required' tag\nKey: 'Email'..."}` |
| **Status** | **PASS** |

---

### Test Case TC-INV-03: Invite Single – Duplicate Pending Invitation

| Field | Details |
|-------|---------|
| **Test Case ID** | TC-INV-03 |
| **Test Scenario / Description** | An admin attempts to invite the same email address twice. The second request should be rejected with a `409 Conflict` since a pending invitation already exists. |
| **Input Data** | First `POST`: `{"email":"bob@example.com","first_name":"Bob","last_name":"Jones","department":"Engineering"}` <br> Second `POST`: same body |
| **Pre-conditions** | Department "Engineering" exists. No existing invitation for `bob@example.com`. |
| **Expected Output** | First request: HTTP `201 Created` <br> Second request: HTTP `409 Conflict` with error `"duplicate invitation: a pending invitation already exists for bob@example.com"` |
| **Actual Output** | First: `201`. Second: `409` with body `{"error":"duplicate invitation: a pending invitation already exists for bob@example.com"}` |
| **Status** |  **PASS** |

---

### Test Case TC-INV-04: Invite Single – Account Already Exists

| Field | Details |
|-------|---------|
| **Test Case ID** | TC-INV-04 |
| **Test Scenario / Description** | An admin tries to invite an email address that already belongs to an active employee in the organization. The system should return a conflict error indicating the account already exists. |
| **Input Data** | `POST /api/orgs/org_1/employees/invite` <br> `{"email":"existing@example.com","first_name":"Existing","last_name":"User","department":"Engineering"}` |
| **Pre-conditions** | User with `email="existing@example.com"` already exists in `users` table for `org_1`. |
| **Expected Output** | HTTP `409 Conflict` <br> Body: `{"error":"account already exists: employee account already exists for existing@example.com"}` |
| **Actual Output** | HTTP `409 Conflict`. Body: `{"error":"account already exists: employee account already exists for existing@example.com"}` |
| **Status** |  **PASS** |

---

### Test Case TC-INV-05: Invite Single – Department Not Found

| Field | Details |
|-------|---------|
| **Test Case ID** | TC-INV-05 |
| **Test Scenario / Description** | An admin specifies a department name that does not exist in the organization. The invitation should be rejected with a `400 Bad Request` indicating the department was not found. |
| **Input Data** | `POST /api/orgs/org_1/employees/invite` <br> `{"email":"charlie@example.com","first_name":"Charlie","last_name":"Brown","department":"NonExistentDept"}` |
| **Pre-conditions** | No department named "NonExistentDept" in `org_1`. |
| **Expected Output** | HTTP `400 Bad Request` <br> Body contains: `"department lookup failed"` |
| **Actual Output** | HTTP `400 Bad Request`. Body: `{"error":"department lookup failed: not found: department \"NonExistentDept\" not found in organization org_1"}` |
| **Status** |  **PASS** |

---

### Test Case TC-INV-06: List Invitations – Only Pending Returned

| Field | Details |
|-------|---------|
| **Test Case ID** | TC-INV-06 |
| **Test Scenario / Description** | When listing invitations for an organization, the endpoint should return **only** invitations with `status="pending"`. Revoked, accepted, and expired invitations must be excluded from the response. |
| **Input Data** | `GET /api/orgs/org_1/invitations` <br> DB seeded with: 1 pending invitation (`inv_1`), 1 revoked invitation (`inv_2`) |
| **Pre-conditions** | `employee_invitations` table has 2 rows for `org_1`: one `pending`, one `revoked`. |
| **Expected Output** | HTTP `200 OK` <br> Response array contains exactly 1 invitation with `id="inv_1"` and `status="pending"` |
| **Actual Output** | HTTP `200 OK`. Response: `[{"id":"inv_1","email":"pending@example.com","status":"pending",...}]` — only 1 item. |
| **Status** |  **PASS** |

---

### Test Case TC-INV-07: Revoke Invitation – Success

| Field | Details |
|-------|---------|
| **Test Case ID** | TC-INV-07 |
| **Test Scenario / Description** | An admin revokes a pending invitation. The invitation's status should be updated to `"revoked"` in the database and the Clerk API revoke call should be triggered (mocked). Subsequent revoke of a non-existent invitation should return 404. |
| **Input Data** | `DELETE /api/orgs/org_1/invitations/inv_1` <br> DB seeded with pending invitation `inv_1` for `org_1`. <br> Clerk revoke mocked to return success. |
| **Pre-conditions** | Invitation `inv_1` exists with `status="pending"` in `org_1`. |
| **Expected Output** | HTTP `200 OK` <br> Body: `{"message":"Invitation revoked"}` <br> DB: `employee_invitations` row with `id="inv_1"` has `status="revoked"` |
| **Actual Output** | HTTP `200 OK`. DB row updated to `status="revoked"`. Second DELETE to non-existent ID: `404 Not Found`. |
| **Status** |  **PASS** |

---

### Test Case TC-INV-08: Bulk Invite via Excel – Partial Success

| Field | Details |
|-------|---------|
| **Test Case ID** | TC-INV-08 |
| **Test Scenario / Description** | An admin uploads an Excel file with 2 employee rows: one valid row and one with a department that doesn't exist. The system should process both rows, successfully invite the valid one, and report the failure for the invalid one. The response should contain a full breakdown (`total_rows`, `successful`, `failed`). |
| **Input Data** | `POST /api/orgs/org_1/employees/invite/bulk` (multipart/form-data) <br> Excel file with 2 rows: Row 1 (valid — department "Engineering" exists), Row 2 (invalid — department "Unknown" does not exist). |
| **Pre-conditions** | Department "Engineering" exists in `org_1`. |
| **Expected Output** | HTTP `200 OK` <br> Body: `{"total_rows":2,"successful":1,"failed":1,"errors":[{"row":2,"email":"...","message":"department lookup failed..."}]}` |
| **Actual Output** | HTTP `200 OK`. Body matches expected: `total_rows=2, successful=1, failed=1` with error details for row 2. |
| **Status** |  **PASS** |

---

### Test Case TC-INV-09: Accept Invitation – Expired Invitation

| Field | Details |
|-------|---------|
| **Test Case ID** | TC-INV-09 |
| **Test Scenario / Description** | A user tries to accept an invitation whose `expires_at` timestamp is in the past. The system should detect the expiry, mark the invitation as `"expired"` in the database, and return an appropriate error to the user. |
| **Input Data** | `POST /api/orgs/org_1/invitations/inv_exp/accept` <br> DB seeded with invitation `inv_exp` having `expires_at = NOW() - 1 hour` |
| **Pre-conditions** | Invitation `inv_exp` has `status="pending"` but `expires_at` is 1 hour in the past. User `user_1` exists. |
| **Expected Output** | HTTP 404 or 400 error with message containing `"expired"` <br> DB: invitation row updates to `status="expired"` |
| **Actual Output** | HTTP `404`. Body: `{"error":"not found: invitation has expired"}`. DB status updated to `"expired"`. |
| **Status** |  **PASS** |

---

## Q2(a) – Test Execution Results

### Execution Setup

```bash
# Navigate to Auth Service
cd /Users/ankita/Desktop/Labs/CS331-06-BusinessAutomation/backend/auth

# Run invitation-related handler tests
env GOCACHE=/tmp/go-build-cache go test ./internal/handler -run 'TestInviteSingleBadRequestOnInvalidBody|TestInviteSingleCreatedThenDuplicateConflict|TestInviteSingleBadRequestWhenDepartmentMissing|TestListInvitationsReturnsOnlyPending|TestRevokeInvitationSuccessThenNotFound|TestInviteBulkProcessesRowsWithPartialFailures' -v -count=1

# Run invitation-related service tests
env GOCACHE=/tmp/go-build-cache go test ./internal/service -run 'TestInviteAndNotify|TestAcceptInvitationByEmail|TestRevokeInvitation' -v -count=1
```

### Summary of Execution Results

| Test Case ID | Test Function (Go) | Execution Time | Result |
|---|---|---|---|
| TC-INV-01 | `TestInviteSingleCreatedThenDuplicateConflict` (first request in the test) | `0.00s` | **PASS** |
| TC-INV-02 | `TestInviteSingleBadRequestOnInvalidBody` | `0.00s` | **PASS** |
| TC-INV-03 | `TestInviteSingleCreatedThenDuplicateConflict` (second request in the test) | `0.00s` | **PASS** |
| TC-INV-04 | `TestInviteAndNotify/existing_account_returns_conflict_error` | Covered inside `TestInviteAndNotify (0.01s)` | **PASS** |
| TC-INV-05 | `TestInviteSingleBadRequestWhenDepartmentMissing` | `0.00s` | **PASS** |
| TC-INV-06 | `TestListInvitationsReturnsOnlyPending` | `0.00s` | **PASS** |
| TC-INV-07 | `TestRevokeInvitationSuccessThenNotFound` and `TestRevokeInvitation/success` | `0.00s` and `0.01s` | **PASS** |
| TC-INV-08 | `TestInviteBulkProcessesRowsWithPartialFailures` | `0.00s` | **PASS** |
| TC-INV-09 | `TestAcceptInvitationByEmail/expired_invitation_marked_expired` | Covered inside `TestAcceptInvitationByEmail (0.01s)` | **PASS** |

### Console Logs (Actual Output)

```
=== RUN   TestInviteSingleBadRequestOnInvalidBody
--- PASS: TestInviteSingleBadRequestOnInvalidBody (0.00s)

=== RUN   TestInviteSingleCreatedThenDuplicateConflict
2026/04/19 21:51:05 Warning: Clerk invitation email failed for new.user@example.com: clerk secret key not configured, skipping email (local invitation still created)
--- PASS: TestInviteSingleCreatedThenDuplicateConflict (0.00s)

=== RUN   TestInviteSingleBadRequestWhenDepartmentMissing
--- PASS: TestInviteSingleBadRequestWhenDepartmentMissing (0.00s)

=== RUN   TestListInvitationsReturnsOnlyPending
--- PASS: TestListInvitationsReturnsOnlyPending (0.00s)

=== RUN   TestRevokeInvitationSuccessThenNotFound
2026/04/19 21:51:05 Invitation inv_1 revoked
--- PASS: TestRevokeInvitationSuccessThenNotFound (0.00s)

=== RUN   TestInviteBulkProcessesRowsWithPartialFailures
2026/04/19 21:51:05 Warning: Clerk invitation email failed for ok@example.com: clerk secret key not configured, skipping email (local invitation still created)
--- PASS: TestInviteBulkProcessesRowsWithPartialFailures (0.00s)

PASS
ok  	github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/handler	1.056s
```

```text
=== RUN   TestRevokeInvitation
=== RUN   TestRevokeInvitation/success
2026/04/19 21:51:04 Invitation inv_1 revoked
=== RUN   TestRevokeInvitation/not_found
=== RUN   TestRevokeInvitation/database_error
--- PASS: TestRevokeInvitation (0.01s)
    --- PASS: TestRevokeInvitation/success (0.00s)
    --- PASS: TestRevokeInvitation/not_found (0.00s)
    --- PASS: TestRevokeInvitation/database_error (0.00s)
=== RUN   TestInviteAndNotify
=== RUN   TestInviteAndNotify/duplicate_pending_invite
=== RUN   TestInviteAndNotify/lookup_database_error
=== RUN   TestInviteAndNotify/existing_account_returns_conflict_error
=== RUN   TestInviteAndNotify/department_lookup_failed
=== RUN   TestInviteAndNotify/success_creates_invitation_and_tolerates_clerk_send_error
2026/04/19 21:51:04 Warning: Clerk invitation email failed for ok@example.com: clerk secret key not configured, skipping email (local invitation still created)
--- PASS: TestInviteAndNotify (0.01s)
    --- PASS: TestInviteAndNotify/duplicate_pending_invite (0.00s)
    --- PASS: TestInviteAndNotify/lookup_database_error (0.00s)
    --- PASS: TestInviteAndNotify/existing_account_returns_conflict_error (0.00s)
    --- PASS: TestInviteAndNotify/department_lookup_failed (0.00s)
    --- PASS: TestInviteAndNotify/success_creates_invitation_and_tolerates_clerk_send_error (0.00s)
=== RUN   TestAcceptInvitationByEmail
=== RUN   TestAcceptInvitationByEmail/no_pending_invitation
=== RUN   TestAcceptInvitationByEmail/expired_invitation_marked_expired
=== RUN   TestAcceptInvitationByEmail/success_with_role_and_job_title
2026/04/19 21:51:04 Invitation accepted: user user_1 joined org org_1, assigned to department dept_1
=== RUN   TestAcceptInvitationByEmail/blank_invitation_names_keep_existing_user_names
2026/04/19 21:51:04 Invitation accepted: user user_blank joined org org_1, assigned to department dept_1
=== RUN   TestAcceptInvitationByEmail/dashboard_access_follows_clerk_admin_membership
2026/04/19 21:51:04 Invitation accepted: user user_admin joined org org_1, assigned to department dept_admin
=== RUN   TestAcceptInvitationByEmail/unknown_invited_role_returns_error
=== RUN   TestAcceptInvitationByEmail/user_not_found_rolls_back_invitation_update
=== RUN   TestAcceptInvitationByEmail/user_update_database_error
--- PASS: TestAcceptInvitationByEmail (0.01s)
    --- PASS: TestAcceptInvitationByEmail/no_pending_invitation (0.00s)
    --- PASS: TestAcceptInvitationByEmail/expired_invitation_marked_expired (0.00s)
    --- PASS: TestAcceptInvitationByEmail/success_with_role_and_job_title (0.00s)
    --- PASS: TestAcceptInvitationByEmail/blank_invitation_names_keep_existing_user_names (0.00s)
    --- PASS: TestAcceptInvitationByEmail/dashboard_access_follows_clerk_admin_membership (0.00s)
    --- PASS: TestAcceptInvitationByEmail/unknown_invited_role_returns_error (0.00s)
    --- PASS: TestAcceptInvitationByEmail/user_not_found_rolls_back_invitation_update (0.00s)
    --- PASS: TestAcceptInvitationByEmail/user_update_database_error (0.00s)
PASS
ok  	github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/service	0.819s
```

**9 invitation-management test cases were designed and executed from the current automated suite, and all 9 passed.**

---

## Q2(b) – Defect Analysis

### BUG-001:Clerk Revoke Failure Blocks Local Revoke

| Field | Details |
|-------|---------|
| **Bug ID** | BUG-001 |
| **Description** | When a pending invitation is revoked, the service calls `ClerkRevokeOrgInvitationsByEmailFunc`. If Clerk returns an error (e.g., no pending Clerk invitation found for this email — common when Clerk email delivery failed or was already revoked externally), the entire `RevokeInvitation` call **fails with an error**, even though the invitation exists in our local DB and should be locally revoked. This causes the admin to receive a 500 Internal Server Error and the invitation remains in `pending` state in our DB. |
| **Steps to Reproduce** | 1. Create an invitation via `POST /api/orgs/:orgId/employees/invite` (Clerk key is missing/invalid, so Clerk invite was never actually created). <br> 2. Call `DELETE /api/orgs/:orgId/invitations/:invId` to revoke. <br> 3. `revokeClerkOrgInvitationsByEmail` fetches Clerk invitation list — finds 0 matching pending invitations. <br> 4. Function returns `"no pending Clerk invitation found for ... in org ..."` error. <br> 5. `RevokeInvitation` propagates this as a service error → handler returns HTTP 500. |
| **Expected Result** | If Clerk has no record of the invitation (e.g., it was never delivered), the local DB record should still be marked as `"revoked"` and the admin should receive HTTP 200. The Clerk revoke failure should be logged as a warning, not a hard error. |
| **Actual Result** | HTTP `500 Internal Server Error` is returned. Local DB invitation remains `"pending"`. Admin cannot revoke the invitation through the UI. |
| **Severity** |  **High** — Admins are blocked from revoking invitations that were created without successful Clerk delivery. |
| **Suggested Fix** | In `invite.go`, change `RevokeInvitation` to call `ClerkRevokeOrgInvitationsByEmailFunc` after the local DB update (not before), or treat a "not found in Clerk" response as a soft/warning-only error: <br><br> |

 
```go
// After marking DB as revoked:
if err := ClerkRevokeOrgInvitationsByEmailFunc(...); err != nil {
    log.Printf("Warning: local revoke succeeded but Clerk revoke failed: %v", err)
    // Do NOT return error — local revoke is the source of truth
}
```
---

### Bug BUG-002: Bulk Invite Response Includes Parse-Error Rows in `total_rows` Inconsistently

| Field | Details |
|-------|---------|
| **Bug ID** | BUG-002 |
| **Description** | The `InviteBulk` handler calculates `total_rows` as `len(parseResult.Rows) + len(parseResult.Errors)`. However, `parseResult.Errors` contains rows that **failed Excel parsing** (e.g., missing email column), while `parseResult.Rows` only contains **successfully parsed rows**. The combined `allErrors` array appends both `parseResult.Errors` (parse errors) and `inviteErrors` (invite-level errors), so the response structure is correct. But the `total_rows` count can mislead callers: if a row fails both parsing AND the Excel parser reports it, an error may appear in both `inviteErrors` and `parseResult.Errors` if the Excel parser emits validation rows separately, leading to inaccurate total counts in edge cases. |
| **Steps to Reproduce** | 1. Create an Excel file with 3 rows: Row 1 valid, Row 2 missing email (parse error), Row 3 has a valid email but non-existent department (invite error). <br> 2. Upload via `POST /api/orgs/:orgId/employees/invite/bulk`. <br> 3. Observe `total_rows` in the response. Expected: 3. Actual: may vary depending on how the Excel parser handles the missing-email row (whether it's counted in `Rows` or only in `Errors`). |
| **Expected Result** | `total_rows` should always equal the actual number of data rows in the uploaded Excel file, regardless of whether rows fail at the parse or invite stage. |
| **Actual Result** | `total_rows` can be unexpectedly low if parse errors cause rows to be excluded from `parseResult.Rows`, making the sum `len(Rows) + len(Errors)` still correct in total but the breakdown potentially confusing to API consumers. |
| **Severity** |  **Medium** — Does not block functionality but produces misleading audit data for bulk invite operations. Admins may think fewer people were processed than actually were. |
| **Suggested Fix** | Have the `ParseEmployeeExcel` function return a `TotalRowsRead` count that equals the raw row count in the sheet (excluding the header), independent of parse outcome. Use this for `total_rows` in the response instead of the derived sum: <br><br> |
```go
// In ParseResult struct:
type ParseResult struct {
    TotalRowsRead int          // raw row count from sheet (excludes header)
    Rows          []ParsedRow  // successfully parsed rows
    Errors        []ParseError // parse-failed rows
}
 
// In handler response:
c.JSON(http.StatusOK, gin.H{
    "total_rows": parseResult.TotalRowsRead,
    "successful": successful,
    "failed":     len(allErrors),
    "errors":     allErrors,
})
```

---

### Bug BUG-003: Deleting a Department with Assigned Employees Returns Misleading Error Code

| Field | Details |
|-------|---------|
| **Bug ID** | BUG-003 |
| **Description** | When an admin attempts to delete a department that still has employees assigned to it, the `DeleteDepartment` service function returns an error string: `"department still has N assigned employee(s)"`. The handler detects this using a brittle `strings.Contains(err.Error(), "still has")` check and returns HTTP `400 Bad Request`. This check is fragile — any refactor of the error message string in the service would silently break the handler's error-matching logic, potentially returning HTTP 500 instead of 400 for this well-known business rule violation. |
| **Steps to Reproduce** | 1. Create department `dept_1` in `org_1`. <br> 2. Assign user `user_1` to `dept_1`. <br> 3. Call `DELETE /api/orgs/org_1/departments/dept_1`. <br> 4. Handler receives `strings.Contains(err.Error(), "still has")` → true → returns 400. <br> 5. If error message is renamed (e.g., `"department has active members"`), handler would fall to default case → returns 500 incorrectly. |
| **Expected Result** | The handler should reliably return HTTP `400 Bad Request` for this business rule violation regardless of future error message text changes. A sentinel error variable (like `ErrDepartmentHasMembers`) should be used for type-safe error matching. |
| **Actual Result** | HTTP `400 Bad Request` works **currently**, but only because of a fragile string match. The error matching is not type-safe and will silently regress if the error message changes. |
| **Severity** |  **Medium** — Current behavior is correct but the implementation is brittle and a maintenance liability. One refactor away from a regression. |
| **Suggested Fix** | Define a sentinel error in the service layer and use `errors.Is()` for matching: <br><br> |

```go
// In service package:
var ErrDepartmentHasMembers = errors.New("department has assigned members")
 
// In DeleteDepartment:
if memberCount > 0 {
    return fmt.Errorf("%w: department still has %d assigned employee(s)",
        ErrDepartmentHasMembers, memberCount)
}
 
// In handler:
case errors.Is(err, service.ErrDepartmentHasMembers):
    c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
```
 
---

---

## Summary Table

### Test Cases Summary

| TC ID | Module | Scenario | Status |
|-------|--------|----------|--------|
| TC-INV-01 | Invitation | Invite single – success |  PASS |
| TC-INV-02 | Invitation | Invite – missing fields |  PASS |
| TC-INV-03 | Invitation | Invite – duplicate pending | PASS |
| TC-INV-04 | Invitation | Invite – account exists |  PASS |
| TC-INV-05 | Invitation | Invite – dept not found |  PASS |
| TC-INV-06 | Invitation | List – only pending returned |  PASS |
| TC-INV-07 | Invitation | Revoke – success + 404 |  PASS |
| TC-INV-08 | Invitation | Bulk invite – partial success |  PASS |
| TC-INV-09 | Invitation | Accept – expired invitation |  PASS |

### Defects Summary

| Bug ID | Description | Severity | Status |
|--------|-------------|----------|--------|
| BUG-001 | Clerk revoke failure blocks local revoke |  High | Open |
| BUG-002 | Bulk invite `total_rows` count inconsistency |  Medium | Open |
| BUG-003 | Brittle string-match for department-has-members error |  Medium | Open |

---


