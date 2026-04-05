# DAL and Testing Notes


## Part A: Data Access Layer (DAL) Implementation [20]

### 1. Backend Services
- Auth Service (`backend/auth`) - PostgreSQL (Supabase) via GORM
- Workflow Service (`backend/workflow`) - MongoDB via official Mongo Go driver
- Integrations Service (`backend/integrations`) - MongoDB via official Mongo Go driver

### 2. Database Setup

#### 2.1 Auth Service (PostgreSQL)
The DAL code is in `internal/database/db.go`.

- `Connect(databaseURL)` opens PostgreSQL connection using GORM.
- `Migrate()` runs `AutoMigrate(...)` for model-backed tables.

Tables created through migration:
- `users`
- `organizations`
- `organization_settings`
- `departments`
- `roles`
- `user_role_memberships`
- `employee_invitations`

Notes:
- The models use indexes and unique rules where needed.
- This gives a clear DAL layer for relational data.

#### 2.2 Workflow Service (MongoDB)
The DAL code is in `internal/storage/mongo.go`.

- `NewMongoStore(ctx, uri)` validates URI, connects, pings DB, and initializes collections in database `workflowdb`.
- Collections used:
  - `workflows`
  - `instances`
  - `tasks`
- `ensureIndexes(ctx)` creates important operational indexes:
  - unique `id` indexes
  - org/workflow/task lookup indexes
  - specialized unique partial indexes for
    - `workflow_id + data.form_response_id`
    - `workflow_id + data.email_message_id`

Notes:
- Mongo creates the database and collections when data is saved.
- The `Store` interface in `internal/storage/store.go` keeps the storage layer clean.

#### 2.3 Integrations Service (MongoDB)
The DAL code is in `internal/storage/mongo.go`.

- `NewMongo(uri, dbName)` connects and pings Mongo.
- Collections initialized:
  - `oauth_tokens`
  - `form_watches`
  - `gmail_watches`
- `ensureIndexes(ctx)` creates indexes for org/provider/account lookup and active-watch polling.
- `Store` interface exists in `internal/storage/store.go` and includes token/watch CRUD methods.

Notes:
- The data model supports more than one provider through the `provider` field.

### 2.4 DAL Schema Dictionary (Columns and What They Signify)

#### 2.4.1 Auth Service (PostgreSQL Tables)

##### Table: `users`
| Column | Means |
|---|---|
| `id` | Primary key (Clerk user ID). |
| `email` | User login/contact email; unique per user. |
| `first_name` | User given name. |
| `last_name` | User family name. |
| `avatar_url` | Profile image URL. |
| `organization_id` | Organization reference for tenant scoping. |
| `department_id` | Department reference within organization. |
| `job_title` | User's role title in business context. |
| `is_admin` | Whether user has org-admin privileges. |
| `preferences` | JSON settings/preferences for user customization. |
| `is_active` | Soft-active state used for deactivation logic. |
| `created_at` | Creation timestamp. |
| `updated_at` | Last update timestamp. |
| `last_sign_in_at` | Most recent login timestamp. |

##### Table: `organizations`
| Column | Means |
|---|---|
| `id` | Primary key (Clerk org ID). |
| `name` | Display name of organization. |
| `slug` | URL-friendly unique short name. |
| `image_url` | Organization logo/image URL. |
| `is_active` | Soft-active flag for organization lifecycle. |
| `created_at` | Creation timestamp. |
| `updated_at` | Last update timestamp. |

##### Table: `organization_settings`
| Column | Means |
|---|---|
| `id` | Primary key (UUID). |
| `organization_id` | Unique foreign key to organization. |
| `domain` | Organization business domain/website domain. |
| `industry` | Industry classification. |
| `size` | Organization size category. |
| `country` | Country/region location. |
| `use_case` | Primary business use-case selected. |
| `created_at` | Creation timestamp. |
| `updated_at` | Last update timestamp. |

##### Table: `departments`
| Column | Means |
|---|---|
| `id` | Primary key (UUID). |
| `name` | Department name (unique per organization). |
| `organization_id` | Organization ownership of department. |
| `description` | Human-readable department purpose. |
| `created_by_user_id` | User who created the department record. |
| `created_at` | Creation timestamp. |
| `updated_at` | Last update timestamp. |

##### Table: `roles`
| Column | Means |
|---|---|
| `id` | Primary key (UUID). |
| `name` | Role name (unique per organization). |
| `description` | Role description and usage context. |
| `organization_id` | Tenant scope for role definition. |
| `created_by_user_id` | User who defined the role. |
| `permissions` | JSON permission set (resource/action style). |
| `is_system_role` | Whether role is system-provided vs custom. |
| `created_at` | Creation timestamp. |
| `updated_at` | Last update timestamp. |

##### Table: `user_role_memberships`
| Column | Means |
|---|---|
| `id` | Primary key (UUID). |
| `organization_id` | Org scope of membership. |
| `user_id` | User linked to role. |
| `role_id` | Assigned role identifier. |
| `assigned_by` | Admin/user who granted membership. |
| `created_at` | Creation timestamp. |
| `updated_at` | Last update timestamp. |

##### Table: `employee_invitations`
| Column | Means |
|---|---|
| `id` | Primary key (UUID). |
| `organization_id` | Target organization of invitation. |
| `department_id` | Target department for invitee. |
| `email` | Invite recipient email. |
| `first_name` | Invitee first name snapshot. |
| `last_name` | Invitee last name snapshot. |
| `role_name` | Local role to assign on acceptance. |
| `role_names` | JSON array for workflow-role tags. |
| `job_title` | Job title for onboarding context. |
| `token` | Unique hashed invitation token. |
| `status` | Invitation state: pending/accepted/expired/revoked. |
| `invited_by` | Inviter user ID (admin actor). |
| `expires_at` | Invitation expiry timestamp. |
| `accepted_at` | Acceptance timestamp (nullable). |
| `accepted_user_id` | User ID that accepted invite (nullable). |
| `created_at` | Creation timestamp. |
| `updated_at` | Last update timestamp. |

#### 2.4.2 Workflow Service (MongoDB Collections)

##### Collection: `workflows`
| Field | Means |
|---|---|
| `id` | Workflow unique identifier. |
| `org_id` | Tenant scope for workflow ownership. |
| `version` | Workflow version for evolution control. |
| `name` | Workflow display name. |
| `description` | Optional workflow description. |
| `department` | Optional department scope/tag. |
| `status` | Workflow lifecycle status. |
| `trigger` | Trigger configuration object (event/source). |
| `nodes` | Ordered workflow node definitions and routing logic. |
| `tags` | Optional label tags for categorization/search. |
| `raw_json` | Raw designer payload for compatibility/debugging. |
| `created_by` | User ID who created workflow. |
| `created_at` | Creation timestamp. |
| `updated_at` | Last update timestamp. |

##### Collection: `instances`
| Field | Means |
|---|---|
| `id` | Instance unique identifier. |
| `org_id` | Tenant scope of execution. |
| `workflow_id` | Parent workflow identifier. |
| `status` | Instance execution state. |
| `current_node` | Active node during execution. |
| `data` | Runtime payload/state map for execution context. |
| `node_states` | Per-node execution states and timings. |
| `audit_log` | Historical action trail for traceability. |
| `started_at` | Start time of instance execution. |
| `completed_at` | End time when finished (nullable). |

##### Collection: `tasks`
| Field | Means |
|---|---|
| `id` | Task assignment unique identifier. |
| `org_id` | Tenant scope for task visibility. |
| `instance_id` | Parent instance reference. |
| `workflow_id` | Parent workflow reference. |
| `node_id` | Workflow node that generated this task. |
| `title` | Task title shown to assignee. |
| `description` | Optional task details. |
| `assigned_role` | Target assignee role group. |
| `assigned_position` | Optional position filter within role. |
| `assigned_user` | Explicit user assignment override. |
| `allowed_actions` | Permitted actions (approve/reject/etc.). |
| `form_template_id` | Optional required form template for task. |
| `sla_days` | SLA deadline in days. |
| `status` | Task lifecycle state. |
| `action_committed` | Final action selected by assignee. |
| `data` | Internal task payload/context data. |
| `visible_data` | Subset exposed to assignee UI/API. |
| `comment` | Optional assignee/admin notes. |
| `created_at` | Task creation timestamp. |
| `completed_at` | Completion timestamp (nullable). |

#### 2.4.3 Integrations Service (MongoDB Collections)

##### Collection: `oauth_tokens`
| Field | Means |
|---|---|
| `_id` | Mongo document identifier. |
| `provider` | Integration provider key (for example `google_forms`). |
| `org_id` | Tenant/organization identifier. |
| `account_id` | Provider account slot ID (for example `primary`). |
| `account_email` | Connected provider account email. |
| `account_name` | Connected provider account display name. |
| `is_primary` | Whether account is default for org/provider. |
| `access_token` | OAuth access token (sensitive). |
| `refresh_token` | OAuth refresh token (sensitive). |
| `token_type` | OAuth token type (typically Bearer). |
| `expiry` | Access token expiry timestamp. |
| `scopes` | Granted OAuth permission scopes. |
| `connected_at` | Time account connection was established. |

##### Collection: `form_watches`
| Field | Means |
|---|---|
| `_id` | Mongo document identifier. |
| `provider` | Watch source provider key. |
| `org_id` | Tenant/organization identifier. |
| `form_id` | Source Google Form ID being monitored. |
| `workflow_id` | Workflow triggered by form events. |
| `active` | Whether watch is currently enabled. |
| `field_mapping` | Mapping from source fields to workflow payload keys. |
| `last_polled_at` | Last time this watch was polled. |
| `last_response_ts` | Cursor/marker for last consumed response event. |
| `created_at` | Watch creation timestamp. |

##### Collection: `gmail_watches`
| Field | Means |
|---|---|
| `_id` | Mongo document identifier. |
| `org_id` | Tenant/organization identifier. |
| `account_id` | OAuth account used for mailbox polling. |
| `workflow_id` | Workflow triggered by matching emails. |
| `query` | Gmail search query/filter expression. |
| `active` | Whether watch is currently enabled. |
| `last_message_internal_ts` | Cursor for last processed Gmail message. |
| `last_polled_at` | Last poll timestamp. |
| `created_at` | Watch creation timestamp. |

### 3. DAL Summary
- Auth DAL: Implemented and migration-enabled.
- Workflow DAL: Implemented with repository interface and index hardening.
- Integrations DAL: Implemented with repository interface and index setup.

Conclusion for Part A:
- The database tables or collections are set up in all services.
- The DAL part is implemented in all services.

---

## Part B: Testing (White Box + Black Box) [10 + 10]

## B1. White Box Testing (Glass Box)
This checks the code from the inside.

### White Box Test Cases

| Service | Area | Test Case | Expected Result | Evidence/Status |
|---|---|---|---|---|
| Auth | DB connect | Call `Connect` with an invalid DSN | Returns an error | Implemented and passing (`db_test.go`) |
| Auth | DB connect | Stub opener success and check global DB | Global DB points to the opened handle | Implemented and passing |
| Auth | Migration guard | Call `Migrate` when DB is nil | Returns "not initialized" error | Implemented and passing |
| Auth | Migration error path | Force `runAutoMigrate` to fail | Error is passed back | Implemented and passing |
| Auth | Migration success path | Run `Migrate` with a valid DB and migrate function | Migrate function is called and returns nil | Implemented and passing |
| Workflow | Mongo constructor guard | Call `NewMongoStore` with an empty URI | Returns `empty mongo uri` | Implemented and passing (`mongo_test.go`) |
| Workflow | ID generation logic | Generate IDs and check format | IDs are 24-char hex and unique | Implemented and passing |
| Integrations | Storage index setup | Check `ensureIndexes` success and failure paths | Index errors are returned to caller | Implemented and passing (`internal/storage/mongo_test.go`) |
| Integrations | Provider filter logic | Check default provider fallback behavior | Missing provider uses the default value | Implemented and passing (`internal/storage/mongo_test.go`) |

---

## B2. Black Box Testing (Functional)
This checks the API from the outside, without looking at the code inside.

### Black Box Test Cases

| Service | Endpoint/Feature | Input Scenario | Expected Output | Evidence/Status |
|---|---|---|---|---|
| Auth | Create department | Send request without `name` | Returns `400 Bad Request` | Implemented and passing (`employee_handler_test.go`) |
| Auth | Create department | Create the same department twice in one org | First request returns `201`, second returns `409 Conflict` | Implemented and passing |
| Auth | Invite employee | Send invite with an invalid email address | Returns `400 Bad Request` | Implemented and passing |
| Auth | Invite employee | Send the same invite twice | First request returns `201`, second returns `409 Conflict` | Implemented and passing |
| Auth | Revoke invitation | Delete an existing invite, then delete a missing one | First request returns `200`, second returns `404 Not Found` | Implemented and passing |
| Workflow | Create workflow | Post workflow without required fields | Returns `400 Bad Request` | Implemented and passing (`handler_test.go`) |
| Workflow | Workflow CRUD | Create, list, get, update, and delete a workflow in one org | Create returns `201`, read/update returns `200`, delete returns `204`, missing record returns `404` | Implemented and passing (`handler_test.go`) |
| Workflow | Start instance | Start with a missing or inactive workflow, then start a valid one | Invalid start fails, valid start succeeds | Implemented and passing (`handler_test.go`) |
| Workflow | Task actions | List tasks by role/instance and approve a task | Correct filtered list and task status change | Implemented and passing (`handler_test.go`, `executor_test.go`) |
| Workflow | Execution flow | Run a linear flow, a condition flow, and a parallel merge flow | Instance moves through the right nodes and finishes correctly | Implemented and passing (`executor_test.go`) |
| Integrations | Org auth middleware | Missing `org_id` or missing bearer token | Returns `400 Bad Request` or `401 Unauthorized` | Implemented and passing (`internal/api/authz_test.go`) |
| Integrations | Integration-key auth | Valid key passes, invalid key is rejected | Valid key returns `200`, invalid key returns `401` | Implemented and passing (`internal/api/authz_test.go`) |
| Integrations | Status endpoint | Request service status with valid org auth | Returns service status and connected account info | Implemented and passing (`internal/api/status_test.go`) |

---

## B3. Testing Performed
I ran these commands on 2026-04-06:

```bash
cd backend/auth && go test ./...
cd backend/workflow && go test ./...
cd backend/integrations && go test ./...
```

What passed:
- Auth service: config, database, handler, middleware, models, and service tests.
- Workflow service: config, connectors, executor, handler, middleware, models, and storage tests.
- Integrations service: `internal/api`, `internal/config`, `internal/googleapi`, `internal/integrations`, `internal/models`, `internal/oauth`, `internal/poller`, `internal/providers/gmail`, `internal/providers/gmail/httpapi`, `internal/providers/googleforms`, `internal/providers/googleforms/httpapi`, and `internal/storage`.
