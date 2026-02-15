# Software Architecture Analysis

## I. Software Architecture Style

### Chosen Architecture: Microservices Architecture

### I-A. Justification by Granularity of Software Components

Our project is already structured in a way that naturally follows the microservices pattern. If we look at the repository, the system is broken down into independently deployable, fine-grained services:

| Granularity Aspect | Evidence from Workspace |
|---|---|
| **Independent Auth Service** | The auth directory is a self-contained Go module with its own go.mod, .env, and cmd/internal packages. It can be built, deployed, and scaled on its own without affecting anything else. |
| **Separate Frontend Application** | The Frontend directory is a standalone Next.js application (with its own package.json, next.config.ts) that talks to backend services over APIs, not through shared memory or direct function calls. |
| **External API Integrations as Boundaries** | The project integrates with Google Forms, Gmail, WhatsApp, and Zoom. Each of these integrations naturally fits as its own microservice (like a notification-service, forms-service, meeting-service), all communicating through well-defined API contracts. |
| **Workflow Engine as a Core Service** | The workflow/automation engine (where admins design workflows) is a distinct bounded context, separate from the workstation where employees execute their tasks. |
| **Per-service Data Ownership** | Each service (auth, workflow, notification, etc.) owns its own data store. This is a key characteristic of microservices, as opposed to having a single shared monolithic database. |

**Granularity Definition:**

- **Coarse-grained services** include: Auth, Workflow Engine, Workstation, Notification, Integration Gateway, Admin Dashboard, and the Frontend (BFF).
- **Fine-grained components within each service**: for example, inside Auth we have token management, user management, and role/permission management.

This is neither a monolith (single deployable) nor nano-services (too fine-grained). Each service wraps a single business capability, which is the defining trait of microservices granularity.

### I-B. Justification: Why Microservices is the Best Choice

| Quality Attribute | Justification |
|---|---|
| **Scalability** | Workflow automation platforms experience uneven load. For example, the notification service (Gmail/WhatsApp) may spike during bulk operations while auth stays idle. Microservices let us scale each service independently. The Zoom meeting service can scale up during peak scheduling hours without having to scale the entire system. |
| **Maintainability** | Each service has its own codebase and tech stack (Go for auth, TypeScript/Next.js for the frontend). Teams can develop, test, and deploy each service independently. If we want to add a new external API like Slack, we just create a new service without touching existing ones (following the Open/Closed Principle). |
| **Performance** | Lightweight, purpose-built services avoid the overhead of a monolithic application loading unnecessary modules. Each service can be tuned for its specific workload. For instance, the workflow engine can use an event-driven async model while auth sticks with synchronous request-response. |
| **Fault Isolation** | If the WhatsApp integration service goes down, the rest of the platform (workflow design, task execution, auth) keeps running. This is critical for enterprise workflow tools where uptime really matters. |
| **Technology Heterogeneity** | The project already uses Go (auth backend) and TypeScript/Next.js (frontend). With microservices, each team can pick the best tool for the job, whether that's Python for ML-based workflow recommendations, Go for high-throughput services, or Node.js for real-time WebSocket-based workstation updates. |
| **Independent Deployment** | Admins designing workflows and employees using the workstation have different release cycles. Microservices let us continuously deploy the workflow designer without needing to redeploy the workstation or auth. |
| **External API Integration** | The project relies heavily on third-party APIs (Google Forms, Gmail, WhatsApp, Zoom). Wrapping each one behind its own microservice gives us an anti-corruption layer, so changes in external APIs don't ripple through our core system. |

**Why NOT other architectures?**

| Architecture | Reason for Rejection |
|---|---|
| **Monolithic** | We can't scale individual components separately. Having a single deployment for auth + workflow + notifications is inefficient and risky for an enterprise platform. |
| **Layered** | Works well for simpler CRUD apps, but our project has multiple bounded contexts (auth, workflow, notifications, integrations) that need independent lifecycles. |
| **SOA** | SOA relies on a centralized ESB (Enterprise Service Bus), which tends to become a bottleneck. Microservices use lightweight communication (REST/gRPC/message queues), which fits much better with our cloud-native, API-driven approach. |

## II. Application Components

Here are the current application components of our Workflow Automation Platform, organized by service boundary. This is not a fixed list; as the project grows, new services and components can be introduced, and existing ones can be restructured or removed based on evolving requirements.

### 1. Authentication & Authorization Service

Some of the core components in this service currently include:

| Component | Responsibility |
|---|---|
| User Registration & Login | Sign-up, sign-in, password reset, email verification |
| Token Management | JWT/OAuth2 token generation, refresh, and revocation |
| Role & Permission Manager | Role-based access control with Admin, Manager, and Employee roles, each having fine-grained permissions |
| Session Manager | Active session tracking, multi-device logout |

Down the line, things like social login (Google/GitHub OAuth), two-factor authentication, or SSO support could be added here as needed.

### 2. Workflow Engine Service

The workflow engine is the heart of the platform. Its current planned components are:

| Component | Responsibility |
|---|---|
| Workflow Designer | Visual drag-and-drop builder for admins to create workflows (steps, conditions, branches) |
| Workflow Template Manager | CRUD operations on reusable workflow templates |
| Rule/Condition Engine | Evaluates conditional logic (if/else, loops, approvals) within workflows |
| Workflow Scheduler | Triggers workflows based on time-based schedules or event-based triggers |
| Workflow Execution Engine | Runs workflow instances, tracks state transitions, and handles retries |

As workflow complexity grows, additional components such as a versioning system for workflows, a sub-workflow manager, or parallel execution handlers could be introduced.

### 3. Workstation Service

The workstation is where employees interact with their assigned work. Key components so far:

| Component | Responsibility |
|---|---|
| Task Queue / Inbox | Shows pending tasks assigned to employees |
| Task Execution Interface | Form-based UI for employees to complete their assigned tasks |
| Task Status Tracker | Tracks task progress (Pending -> In Progress -> Completed -> Approved) |
| Collaboration Module | Comments, attachments, and real-time updates on tasks |

This could later expand to include things like a file manager, a kanban board view, or priority-based task sorting depending on what companies need.

### 4. Notification Service

Currently planned notification channels and components:

| Component | Responsibility |
|---|---|
| Email Notification (Gmail API) | Sends email alerts for task assignments, approvals, and deadlines |
| WhatsApp Notification (WhatsApp API) | Sends WhatsApp messages for urgent notifications |
| In-App Notification | Real-time push notifications within the workstation UI |
| Notification Preference Manager | User-level settings for notification channels and frequency |

New channels like SMS (via Twilio), Slack notifications, or Microsoft Teams alerts can be plugged in later without affecting the existing ones.

### 5. Integration Gateway Service

This service acts as the bridge between our platform and external tools. The connectors listed below are the ones we are starting with, but the gateway is designed to be extensible so that new integrations can be added over time:

| Component | Responsibility |
|---|---|
| Google Forms Connector | Creates and reads Google Forms for data collection within workflows |
| Zoom Meeting Connector | Schedules Zoom meetings as workflow steps (e.g., approval calls) |
| Gmail Connector | Sends and reads emails as part of workflow automation |
| WhatsApp Connector | Sends messages and receives responses via WhatsApp Business API |
| API Gateway / Rate Limiter | Centralized entry point for all external API calls with rate limiting and retry logic |

Future connectors could include Slack, Google Calendar, Microsoft 365, Trello, Jira, or any other tool a company already uses. The microservices approach makes it straightforward to add a new connector without disrupting existing ones.

### 6. Admin Dashboard Service

The admin dashboard gives organization admins visibility and control. Current components include:

| Component | Responsibility |
|---|---|
| Company Configuration | Company profile, branding, workspace settings |
| User Management | Invite/remove users, assign roles, manage teams |
| Workflow Analytics | Dashboards showing workflow completion rates, bottlenecks, and SLA compliance |
| Audit Log Viewer | Tracks all admin actions and workflow state changes for compliance |

Additional features like billing management, usage reports, or custom role builders could be added as the platform matures.

### 7. Frontend Application (BFF, Backend for Frontend)

The frontend is what users directly interact with. Its main components right now are:

| Component | Responsibility |
|---|---|
| Admin Portal UI | Workflow designer, user management, analytics dashboards |
| Employee Workstation UI | Task inbox, task execution forms, notifications |
| Authentication UI | Login, registration, password reset pages |
| API Client Layer | Axios/Fetch wrappers for communicating with backend microservices |
| State Management | Client-side state (React Context / Zustand / Redux) for session, tasks, and workflows |
| Real-time Updates (WebSocket) | Live task updates, notification badges, collaboration features |

As the product evolves, new UI modules (like a reporting page, a settings panel, or onboarding wizards) can be added as needed.

### 8. Data & Persistence Layer (per-service databases)

Each service has its own database. The current set looks like this:

| Component | Responsibility |
|---|---|
| Auth DB | Users, roles, permissions, sessions |
| Workflow DB | Workflow definitions, templates, execution history |
| Task DB | Task assignments, status, comments, attachments |
| Notification DB | Notification logs, delivery status, preferences |
| Audit/Log DB | Immutable audit trail of all system events |

If new services are introduced (for example, a dedicated analytics service or a file storage service), they would get their own databases following the same pattern.

### Component Interaction Diagram

```
┌──────────────┐
│   Frontend   │  (Next.js - TypeScript)
│   [BFF]      │
└──────┬───────┘
       │ REST / GraphQL / WebSocket
       ▼
┌──────────────┐     ┌──────────────┐     ┌───────────────────┐
│  Auth Service│     │  Workflow    │     │  Workstation      │
│  (Go)        │◄───►│  Engine      │◄───►│  Service          │
└──────────────┘     └──────┬───────┘     └───────────────────┘
                            │
              ┌─────────────┼─────────────┐
              ▼             ▼             ▼
     ┌──────────────┐ ┌──────────┐ ┌───────────────┐
     │ Notification │ │  Admin   │ │  Integration  │
     │ Service      │ │ Dashboard│ │  Gateway      │
     └──────────────┘ └──────────┘ └───────┬───────┘
                                           │
                            ┌──────────────┼──────────────┐
                            ▼              ▼              ▼
                      Google Forms      Zoom API     WhatsApp/Gmail
```