package storage

import "github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/models"

// Store defines persistence operations required by the service.
type Store interface {
	// Workflows
	SaveWorkflow(models.Workflow) (string, error)
	GetWorkflow(id string) (models.Workflow, bool)
	GetWorkflowsByIDs(ids []string) (map[string]models.Workflow, error)
	ListWorkflows(orgID string) ([]models.Workflow, error)
	DeleteWorkflow(id string) error

	// Instances
	SaveInstance(models.Instance) (string, error)
	GetInstance(id string) (models.Instance, bool)
	FindInstanceByWorkflowAndFormResponse(workflowID, formResponseID string) (models.Instance, bool, error)
	ListInstancesByOrg(orgID string) ([]models.Instance, error)
	ListInstancesByWorkflow(workflowID string) ([]models.Instance, error)

	// Task Assignments
	SaveTask(models.TaskAssignment) (string, error)
	GetTask(id string) (models.TaskAssignment, bool)
	CompareAndSwapTask(models.TaskAssignment, models.TaskStatus) (bool, error)
	HasActiveTasks(instanceID string) (bool, error)
	ListTasksByAssignee(orgID, userID string) ([]models.TaskAssignment, error)
	ListTasksByRole(orgID, role string) ([]models.TaskAssignment, error)
	ListTasksByRoles(orgID string, roles []string) ([]models.TaskAssignment, error)
	ListTasksByInstance(instanceID string) ([]models.TaskAssignment, error)
}
