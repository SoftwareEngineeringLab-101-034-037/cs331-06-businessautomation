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
