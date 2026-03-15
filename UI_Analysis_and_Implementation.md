# UI Analysis and Implementation

## I. Selected UI Type and Justification

For this project, the most appropriate interface is a **Direct Manipulation Interface (DMI)**.

### Chosen UI Type
- **Direct manipulation interface (DMI)** as the single chosen UI model.

Menu navigation and forms in this system are interaction components inside the same DMI experience, not a separate UI type.

### Why this is appropriate for this project
The software is a Business Automation Platform where users create, edit, and manage workflows. Workflow design is inherently visual (nodes, connections, and process paths), so users should be able to interact with the system by manipulating objects on screen rather than writing commands. Delivering this as a web-based solution further strengthens the direct UI approach because users access the same interactive canvas from any modern browser without local installation.

#### 1. Natural fit for workflow engineering
Users can build processes by interacting directly with workflow nodes and edges. This matches user mental models better than command syntax.

#### 2. Better learnability for non-technical users
The system targets business teams (operations, HR, finance, etc.), not only developers. A visual builder lowers the learning curve and reduces training time.

#### 3. Immediate feedback and lower error rate
When users drag, connect, or remove workflow elements, they receive instant visual feedback. This helps prevent modeling mistakes early.

#### 4. Faster modeling and iteration
Direct manipulation supports quick experimentation (add step, connect path, adjust trigger, review flow), which is critical for process optimization.

#### 5. Works well with role-based enterprise usage
The application includes visual editing plus structured forms for organization setup, authentication, and controlled access. These are integrated features within the same DMI workflow and are suitable for enterprise systems.

#### 6. Web-based delivery improves usability and adoption
A browser-based interface is ideal for direct manipulation workflows because it is easy to access, easy to update, and consistent across devices. Teams in different departments can use the same interface immediately through a URL, while updates to the UI are deployed centrally without requiring users to reinstall software.

### Why other UI styles are less suitable
- **Command-language interface:** powerful but too technical for most intended users and hard to visualize workflows.
- **Pure menu-based interface:** good for navigation but weak for complex process modeling.
- **Form-only interface:** too rigid and inefficient for dynamic workflow graph editing.

### Conclusion
A **Direct Manipulation UI** is the best choice for this project because the core business function is visual workflow orchestration. Implementing it as a web-based solution makes the interface more practical for organizations by improving accessibility, maintainability, and cross-team adoption. Menu and form interactions in the product are part of this same DMI implementation.

---

## II. UI Code Components and User Interactions (Brief)

The UI is already implemented in the frontend using Next.js and React.

### Key implemented UI components
- Landing and navigation interface with entry actions (Create Organisation, Join Organisation).
- Organization setup wizard with multi-step form flow and progress indicators.
- Authentication and sign-in screens integrated with Clerk.
- Visual workflow builder canvas using React Flow (`@xyflow/react`) for node/edge interactions.
- Step/Trigger editors for configuring workflow behavior from side panels.
- Theme controls, notifications (toasts), and dashboard-level role-gated access.

### Main user interaction flow
1. User opens the platform home page.
2. User creates or joins an organisation.
3. User authenticates and accesses the dashboard.
4. User opens Workflow Builder.
5. User adds nodes, drags nodes, connects edges, edits step/trigger details, and removes edges/steps.
6. User validates and proceeds to publish/save workflow.

