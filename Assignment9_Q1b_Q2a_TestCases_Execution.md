# CS 331 Assignment 9 - Q1(b) and Q2(a)

## Selected Major Module
Employee and Invitation Management (Auth service)

## Execution Command Used
From backend/auth:

go test ./internal/handler -run "TestCreateDepartmentBadRequestWhenNameMissing|TestCreateDepartmentCreatedThenConflict|TestListDepartmentsReturnsOrgDepartments|TestUpdateOrganizationSettingsUpsertsAndPersists|TestInviteSingleBadRequestOnInvalidBody|TestInviteSingleCreatedThenDuplicateConflict|TestRevokeInvitationSuccessThenNotFound|TestDeleteDepartmentWithAssignedEmployeeReturnsBadRequest" -v | Tee-Object -FilePath ..\\..\\Assignment9_TestExecution.log

## Evidence
- Log file: Assignment9_TestExecution.log
- Source tests: backend/auth/internal/handler/employee_handler_test.go

## Test Cases (8)

| Test Case ID | Test Scenario / Description | Input Data | Expected Output | Actual Output | Status |
|---|---|---|---|---|---|
| TC-EMP-001 | Create department with missing required name | POST /api/orgs/org_1/departments with JSON containing only description | 400 Bad Request | TestCreateDepartmentBadRequestWhenNameMissing passed; handler returned 400 | Pass |
| TC-EMP-002 | Create duplicate department in same org | Two POST requests with name=Engineering, description=Core team | First request 201, second request 409 conflict | TestCreateDepartmentCreatedThenConflict passed; first create succeeded then duplicate conflicted | Pass |
| TC-EMP-003 | List departments returns only requested org data | Seed departments for org_1 and org_2 then GET org_1 departments | 200 OK with only org_1 records | TestListDepartmentsReturnsOrgDepartments passed; response length matched org filter | Pass |
| TC-EMP-004 | Update organization settings persists normalized values | PUT /api/orgs/org_1/settings with domain and profile fields | 200 OK and DB stores updated values (trimmed where needed) | TestUpdateOrganizationSettingsUpsertsAndPersists passed; persisted values verified | Pass |
| TC-EMP-005 | Invite single employee with invalid email body | POST /api/orgs/org_1/employees/invite with malformed email | 400 Bad Request | TestInviteSingleBadRequestOnInvalidBody passed; validation rejected payload | Pass |
| TC-EMP-006 | Prevent duplicate pending single invite | Two invite requests with same email/new user payload | First request 201, second request 409 conflict | TestInviteSingleCreatedThenDuplicateConflict passed; duplicate pending invite blocked | Pass |
| TC-EMP-007 | Revoke invitation then try revoking again | DELETE invitation inv_1 twice | First revoke 200, second revoke 404 | TestRevokeInvitationSuccessThenNotFound passed; state transitioned as expected | Pass |
| TC-EMP-008 | Block deleting department with assigned employees | DELETE /api/orgs/org_1/departments/dept_1 where user assigned to dept_1 | 400 Bad Request with business-rule message | TestDeleteDepartmentWithAssignedEmployeeReturnsBadRequest passed; delete prevented | Pass |

## Summary of Execution Result
- Total executed: 8
- Passed: 8
- Failed: 0
- Result source: Assignment9_TestExecution.log

## Evidence/Attachment Checklist
- Assignment9_TestExecution.log: terminal log showing the 8 designed test cases executed successfully.
- Assignment9_DefectEvidence.log: terminal log showing the Q2(b) exploratory defect tests.
- Assignment9_DefectEvidence_Screenshot.png: screenshot of terminal evidence for the defect discovery tests.
