# CS 331 Assignment 9 - Q2(b) Defect Report

## Defect Discovery Method
Targeted exploratory tests were executed to validate edge-case behavior not covered by happy-path test cases.

Execution command used:

go test ./internal/service -run "TestAssignment9Defect_" -v | Tee-Object -FilePath ..\\..\\Assignment9_DefectEvidence.log

Evidence log:
- Assignment9_DefectEvidence.log

Screenshot evidence:
- Assignment9_DefectEvidence_Screenshot.png

Source exploratory tests:
- backend/auth/internal/service/assignment9_exploratory_test.go

Relevant implementation references:
- backend/auth/internal/service/invite.go:63
- backend/auth/internal/service/invite.go:346
- backend/auth/internal/service/invite.go:387

---

## Defect 1
- Bug ID: BUG-A9-001
- Description: Invitation acceptance compares invited email and user email case-sensitively, causing valid invitations to fail when only letter case differs.
- Steps to Reproduce:
  1. Create user with email User.Case@Example.com.
  2. Create pending invitation for user.case@example.com in same org.
  3. Call invitation acceptance by ID for that user.
- Expected Result: Acceptance should succeed because email local/domain comparison should be case-insensitive for this workflow.
- Actual Result: Rejected with message invitation email does not match user email.
- Severity: Medium
- Suggested Fix: Compare normalized emails using strings.EqualFold(strings.TrimSpace(invitation.Email), strings.TrimSpace(user.Email)) and ensure the follow-up pending-invitation lookup also uses the normalized invitation email or a case-insensitive query.

## Defect 2
- Bug ID: BUG-A9-002
- Description: Department lookup for invitation resolution does not trim input name, causing valid departments to be unresolved when UI/client sends extra spaces.
- Steps to Reproduce:
  1. Create department named Engineering.
  2. Call department resolution/invite flow with input department value " Engineering ".
  3. Attempt invitation creation.
- Expected Result: Department should resolve successfully after whitespace normalization.
- Actual Result: Returns not found for department " Engineering ".
- Severity: Medium
- Suggested Fix: Normalize department input before lookup (trim spaces; optionally case-insensitive matching by normalized key).

## Defect 3
- Bug ID: BUG-A9-003
- Description: Duplicate pending invite check is case-sensitive for email, allowing logically duplicate invites for same address with case variants.
- Steps to Reproduce:
  1. Create first invite for First.User@Example.com.
  2. Create second invite for first.user@example.com in same org while first is pending.
  3. Observe insert behavior.
- Expected Result: Second invite should be blocked as duplicate pending invite.
- Actual Result: Second invite is created; duplicate prevention bypassed due to case-sensitive equality.
- Severity: High
- Suggested Fix: Use case-insensitive duplicate query (lower(email) = lower(?)) and add DB-level normalized uniqueness for pending invites where possible.

---

## Evidence Snippets from Log
- Defect 1 evidence line: expected acceptance to be case-insensitive for email; got invitation email does not match user email
- Defect 2 evidence line: expected department lookup to trim input; got not found: department " Engineering " not found in organization org_1
- Defect 3 evidence line: expected duplicate invitation blocked for case-variant email

## Suggested Retest After Fix
1. Re-run exploratory tests for the three defect scenarios.
2. Add permanent regression tests in service tests to prevent recurrence.
3. Re-run full suite: go test ./...
