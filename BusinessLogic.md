# Business Logic Layer Analysis

## Project Context

This project is a **Business Automation Platform** with:

- a **Frontend** built in Next.js/React for organisation setup, team management, dashboard access, and visual workflow design
- an **Auth service** in Go for departments, roles, employees, invitations, and organisation membership
- a **Workflow service** in Go for workflow definition storage, workflow execution, task assignment, and action connectors

In this system, the **business logic layer (BLL)** sits mainly inside the Go service layer and workflow engine. It enforces organisational rules, workflow rules, execution rules, validation rules, and data-shaping rules before data reaches the UI.

---

## Q1. Core Functional Modules of the Business Logic Layer and Their Interaction with the Presentation Layer

The main BLL modules already implemented in this project are:

### 1. Organisation Structure Management Module

This module is implemented primarily in:

- `backend/auth/internal/service/employee_service.go`
- `backend/auth/internal/service/role_management.go`
- `backend/auth/internal/handler/employee_handler.go`

Its responsibilities are:

- creating departments
- updating and deleting departments
- creating workflow roles
- updating role membership
- listing department summaries and role summaries
- preventing duplicate department and role names inside the same organisation
- preventing invalid role membership assignments
- preventing department deletion when employees are still assigned to it

### UI interaction

This BLL module interacts with UI components such as:

- `Frontend/src/components/dashboard/CreateDepartmentDialog.tsx`
- `Frontend/src/components/dashboard/CreateRoleDialog.tsx`
- `Frontend/src/app/dashboard/team/page.tsx`

Interaction flow:

1. The admin enters department or role data in the dialog.
2. The frontend sends authenticated API requests to the auth service.
3. The service layer validates names, organisation scope, uniqueness, and membership rules.
4. The handler returns structured JSON.
5. The frontend refreshes the team and role views and displays success/error messages.

So the UI collects input, but the real business rule enforcement happens in the BLL.

---

### 2. Employee Invitation and Onboarding Module

This module is implemented in:

- `backend/auth/internal/service/invite.go`
- `backend/auth/internal/service/excel.go`
- `backend/auth/internal/handler/employee_handler.go`

Its responsibilities are:

- inviting a single employee
- bulk inviting employees from Excel
- resolving department references
- preventing duplicate pending invitations
- setting invitation expiry
- accepting invitations
- assigning department and workflow roles to the accepted user
- integrating with Clerk invitation email delivery

### UI interaction

This module interacts with:

- `Frontend/src/components/dashboard/InviteDialog.tsx`
- `Frontend/src/app/join/page.tsx`
- `Frontend/src/app/dashboard/team/page.tsx`

Interaction flow:

1. Admin opens the invite dialog and submits single or bulk invite data.
2. Frontend validates basic required fields and file type.
3. Backend invitation logic checks duplicate invites, department existence, and role assignment rules.
4. Invitation records are stored and Clerk invitation email logic is triggered.
5. When the invited employee joins/signs in, the backend accepts the invitation and assigns organisation, department, and role metadata to the user record.

This is a strong example of the BLL acting as the mediator between UI events and persistent organisation state.

---

### 3. Workflow Definition Management Module

This module is implemented in:

- `backend/workflow/internal/handler/workflow_handler.go`
- `backend/workflow/internal/models/types.go`

Its responsibilities are:

- creating workflows
- loading workflows by organisation
- updating workflows
- deleting workflows
- generating server-side workflow IDs
- attaching workflow ownership to the authenticated user
- maintaining workflow version rules
- distinguishing `draft` and `active` workflows

### UI interaction

This module interacts with:

- `Frontend/src/app/workflow-builder/page.tsx`
- `Frontend/src/components/dashboard/WorkflowCanvas.tsx`
- `Frontend/src/components/dashboard/StepEditor.tsx`

Interaction flow:

1. User visually designs a workflow in the workflow builder.
2. The frontend stores the builder state as steps and edges.
3. On save/publish, the frontend transforms the canvas model into the backend workflow schema.
4. The backend handler applies BLL rules such as server-generated IDs, organisation ownership, versioning, creator attribution, and active/draft handling.
5. The workflow is stored and later reloaded into the builder.

This module is central because it converts a visual UI model into an executable business process.

---

### 4. Workflow Execution Engine Module

This module is implemented in:

- `backend/workflow/internal/executor/executor.go`
- `backend/workflow/internal/handler/instance_handler.go`

Its responsibilities are:

- starting workflow instances
- locating the start node
- walking the workflow graph
- evaluating condition nodes
- creating task assignments for human steps
- executing action connectors
- handling parallel branches
- synchronising merge nodes
- writing execution audit logs
- updating per-node execution states
- marking instances as completed or failed

### UI interaction

This module is connected to the presentation layer through:

- `Frontend/src/app/workflow-builder/page.tsx`
- `Frontend/src/app/dashboard/team/page.tsx`

Interaction flow:

1. A published workflow becomes executable.
2. When an instance is started through the workflow API, the executor creates runtime state.
3. Human task nodes generate task assignments with assigned role, position, user, SLA, and allowed actions.
4. The team dashboard then reads tasks by role from the workflow service and shows them in the employee drawer.
5. The UI can therefore display operational consequences of the workflow logic that the executor has produced.

Important accuracy note:

- The executor currently **simulates human completion** by automatically choosing the first allowed action after a short delay.
- Therefore, the task creation logic is implemented, but full pause-and-resume human workflow execution is still only partially implemented.

---

### 5. Task Assignment and Task Action Module

This module is implemented in:

- `backend/workflow/internal/handler/task_handler.go`
- `backend/workflow/internal/models/types.go`

Its responsibilities are:

- listing tasks by role
- listing tasks by workflow instance
- changing task status through actions such as `approve`, `reject`, `clarify`, and `complete`
- storing optional task comments
- recording task completion timestamps

### UI interaction

This module interacts with:

- `Frontend/src/app/dashboard/team/page.tsx`
- `Frontend/src/components/dashboard/TaskActions.tsx`

Interaction flow:

1. The UI loads workflow tasks for the roles assigned to an employee.
2. The backend filters tasks by organisation and role.
3. Returned tasks are shown in the employee detail drawer.
4. Task action UI components define user actions that map to backend task action routes.

Important accuracy note:

- `TaskActions.tsx` currently contains richer UI actions such as `start_progress`, `escalate`, `send_back`, `cancel`, and `reopen`.
- The workflow backend currently supports only `approve`, `reject`, `clarify`, and `complete`.
- So the presentation layer is ahead of the current backend BLL for task lifecycle richness.

---

## Summary of BLL to UI Mapping

| Business Logic Module | Backend Implementation | UI Components Already Implemented |
|---|---|---|
| Organisation structure | Auth service layer for departments and roles | Department dialog, role dialog, team page |
| Employee invitation and onboarding | Invite service, Excel parser, invitation acceptance logic | Invite dialog, join page, team page |
| Workflow definition management | Workflow handler and workflow models | Workflow builder page, workflow canvas, step editor |
| Workflow execution engine | Executor and instance handler | Workflow builder output, team task display |
| Task assignment and action processing | Task handler and task models | Team page task queue, task action UI |

Therefore, the BLL of this project is not a single file or class. It is a set of coordinated backend modules that convert user actions from the UI into validated organisational and workflow operations.

---

## Q2(A). How Business Rules Are Implemented for Different Modules

Business rules are implemented mainly in the service layer and workflow engine.

### A.1 Organisation and Department Rules

Implemented in `employee_service.go` and `role_management.go`.

Rules implemented include:

- **Department name must not be empty**
  - `CreateDepartment` normalises and trims the name, then rejects empty values.
- **Department names must be unique within the same organisation**
  - duplicates are checked using a case-insensitive trimmed lookup
- **A department cannot be deleted if employees are still assigned to it**
  - `DeleteDepartment` blocks deletion if users still reference that department
- **Role names must be unique within the same organisation**
  - enforced in `CreateRole` and `UpdateRole`
- **Only users belonging to the same organisation can be assigned to a role**
  - `addUsersToRole` validates membership before inserting role mappings

These are clear business rules because they protect organisational consistency, not just data format.

### A.2 Invitation and Access Rules

Implemented in `invite.go` and `employee_handler.go`.

Rules implemented include:

- **Only one pending invitation per email per organisation**
- **Invitation must belong to a valid department**
- **Invitation expires after 7 days**
- **Only the invited user can accept the invitation**
  - the accepting user email must match the invitation email
- **Accepted invitation automatically updates organisation, department, and role membership**

These rules ensure onboarding follows the organisation’s access policy.

### A.3 Workflow Definition Rules

Implemented in `workflow_handler.go`, `types.go`, and frontend publish logic.

Rules implemented include:

- **Workflow name is required**
- **Workflow ID is generated on the server**
  - client-supplied ID is ignored during creation
- **Workflow belongs to a specific organisation**
  - org ID is derived from the route, not trusted from the client
- **Only active workflows can be started**
  - enforced in `instance_handler.go`
- **Draft workflows use version `0`**
- **Published workflows increment version numbers**
- **Workflow creator is attributed from the authenticated user**

### A.4 Workflow Routing and Execution Rules

Implemented in `executor.go`.

Rules implemented include:

- **Workflow execution starts only from a start node**
- **Condition nodes route execution to `yes` or `no` branches**
- **Task nodes create human task assignments**
- **Task nodes can branch by action through `next_actions`**
- **Parallel nodes fan out to multiple branches concurrently**
- **Merge nodes wait until required branches arrive before continuing**
- **Action nodes call the appropriate connector based on connector type**
- **Instance and node execution history is stored in audit logs**

These are the core process-automation rules of the system.

### A.5 UI-Side Workflow Publishing Rules

Implemented in `Frontend/src/app/workflow-builder/page.tsx`.

Before a workflow is published, the builder validates several structural rules:

- exactly one start node
- at least one end node
- start must have outgoing edges
- end must have incoming edges
- non-start nodes must be reachable
- non-end nodes must have outgoing edges
- condition nodes must have both yes and no branches
- parallel and merge nodes must have valid branch/input counts
- at least one end node must be reachable from the start

This means some business rules are enforced in the UI first for immediate feedback, then reinforced again by backend execution rules.

---

## Q2(B). Validation Logic Implemented in the Application

Yes, validation logic has been implemented in both the frontend and backend.

### B.1 Frontend Validation

Examples:

- `CreateDepartmentDialog.tsx`
  - department name is required before submission
- `CreateRoleDialog.tsx`
  - role name is required before submission
- `InviteDialog.tsx`
  - checks required fields for single invite
  - checks email format with regex
  - checks Excel file extension for bulk upload
- `workflow-builder/page.tsx`
  - validates workflow graph correctness before publishing
  - prevents publishing without a workflow name
  - requires commit message when updating a non-draft workflow

Frontend validation improves usability by catching errors early and giving immediate feedback.

### B.2 Backend Validation

Examples:

- `employee_handler.go`
  - uses Gin binding such as `binding:"required"` and `binding:"required,email"`
- `employee_service.go`
  - trims names and rejects empty department/role names
  - checks duplicate names
  - checks whether selected role members belong to the same organisation
- `invite.go`
  - checks duplicate invitations
  - validates department resolution
  - validates invitation ownership and expiry at acceptance time
- `excel.go`
  - validates required spreadsheet headers
  - validates row-level required fields for bulk invite import
- `workflow_handler.go`
  - rejects invalid JSON
  - requires workflow name
- `task_handler.go`
  - rejects unknown task actions
- `instance_handler.go`
  - rejects invalid JSON
  - rejects missing workflow IDs
  - blocks execution of non-active workflows

### B.3 Why This Validation Matters

The validation logic ensures:

- organisational records remain consistent
- invalid workflow structures are not published
- duplicate or conflicting records are prevented
- only valid employees, roles, departments, and invitations enter the system
- workflows are executable before runtime

So validation is definitely implemented and is a major part of this project’s BLL.

---

## Q2(C). How Data Transformation Is Handled Between Data Layer and Presentation Layer

Data transformation is also clearly present in this project.

### C.1 Workflow Builder Transformation

This is the strongest example of data transformation.

The UI builder stores workflows as:

- `steps`
- `edges`
- React Flow positions
- UI-oriented configuration state

Before sending data to the backend, `Frontend/src/app/workflow-builder/page.tsx` transforms this into the backend workflow schema:

- `start`, `task`, and `action` nodes become nodes with a single `next`
- condition edges become `next_yes` and `next_no`
- task action edges become `next_actions`
- parallel branches become `next_branches`
- merge node incoming edges become `required_inputs` and `optional_inputs`
- the trigger type `form_submission` is converted to backend value `form_submit`
- the original UI graph is preserved in `raw_json` for later re-import

This is a direct BLL-related transformation from a UI editing model to an executable process model.

### C.2 Re-import Transformation from Backend to UI

When a workflow is opened for editing, the frontend loads the stored workflow and rebuilds the editor state:

- it reads normal workflow metadata from the backend object
- it restores `steps` and `edges` from `raw_json` when available
- it maps backend trigger values and workflow fields back into the editor draft

So transformation is not one-way; it supports both publish and reload cycles.

### C.3 Department and Role Summary Transformation

The auth service transforms raw database entities into richer summary objects:

- `DepartmentSummary`
- `RoleSummary`
- `RoleMemberSummary`
- `ActorSummary`

These summaries include:

- member counts
- creator details
- nested role member data
- department names instead of only foreign keys where useful

This makes backend responses much more presentation-friendly and reduces UI-side data stitching.

### C.4 Invitation and Employee Transformation

Invitation acceptance transforms invitation data into actual user membership state by:

- moving organisation and department data into the user record
- converting stored role tags into actual role memberships
- updating user job title when supplied

This is a business transformation from temporary onboarding state to permanent operational state.

---

## Final Conclusion

The business logic layer of this project is already implemented across the **auth service** and **workflow service**, with the strongest modules being:

- organisation structure management
- invitation and onboarding management
- workflow definition management
- workflow execution
- task assignment and task action handling

The presentation layer is already connected to these modules through the implemented dialogs, dashboard screens, team views, and workflow builder. Business rules, validation logic, and data transformation are all present in the current codebase.

One important project-specific observation is that the architecture is already strong for the lab requirement, but some parts are still evolving:

- workflow execution currently simulates human task completion instead of fully pausing for real user action
- the task action UI exposes more lifecycle options than the current workflow task backend supports
- some dashboard pages still use mock data, so not every presentation feature is backed by live BLL yet

Even with those gaps, the implemented code already provides a valid and concrete business logic layer suitable for this assignment.
