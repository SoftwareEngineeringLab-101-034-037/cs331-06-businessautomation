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