package storage

import "github.com/example/business-automation/backend/workflow/internal/models"

// Store defines persistence operations required by the service.
type Store interface {
	// Workflows
	SaveWorkflow(models.Workflow) (string, error)
	GetWorkflow(id string) (models.Workflow, bool)
	ListWorkflows(orgID string) ([]models.Workflow, error)
	DeleteWorkflow(id string) error

	// Instances
	SaveInstance(models.Instance) (string, error)
	GetInstance(id string) (models.Instance, bool)
	ListInstancesByOrg(orgID string) ([]models.Instance, error)
	ListInstancesByWorkflow(workflowID string) ([]models.Instance, error)

	// Task Assignments
	SaveTask(models.TaskAssignment) (string, error)
	GetTask(id string) (models.TaskAssignment, bool)
	ListTasksByAssignee(orgID, userID string) ([]models.TaskAssignment, error)
	ListTasksByRole(orgID, role string) ([]models.TaskAssignment, error)
	ListTasksByInstance(instanceID string) ([]models.TaskAssignment, error)
}
