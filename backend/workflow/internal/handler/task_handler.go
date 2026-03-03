package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/example/business-automation/backend/workflow/internal/models"
	"github.com/example/business-automation/backend/workflow/internal/storage"
)

// TaskHandler handles listing and actioning task assignments.
type TaskHandler struct {
	Store storage.Store
}

func NewTaskHandler(store storage.Store) *TaskHandler {
	return &TaskHandler{Store: store}
}

// GET /api/tasks?role=...  or  GET /api/tasks?instance_id=...
func (h *TaskHandler) List(c *gin.Context) {
	role := c.Query("role")
	instanceID := c.Query("instance_id")

	var tasks []models.TaskAssignment
	var err error

	switch {
	case instanceID != "":
		tasks, err = h.Store.ListTasksByInstance(instanceID)
	case role != "":
		tasks, err = h.Store.ListTasksByRole(role)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "provide ?role= or ?instance_id="})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if tasks == nil {
		tasks = []models.TaskAssignment{}
	}
	c.JSON(http.StatusOK, tasks)
}

// PUT /api/tasks/:id/:action  (action = approve | reject | clarify | complete)
func (h *TaskHandler) Action(c *gin.Context) {
	taskID := c.Param("id")
	action := c.Param("action")

	task, ok := h.Store.GetTask(taskID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	var body struct {
		Comment string `json:"comment"`
	}
	_ = c.ShouldBindJSON(&body) // comment is optional

	now := time.Now()
	task.Comment = body.Comment
	task.CompletedAt = &now

	switch action {
	case "approve":
		task.Status = models.TaskApproved
	case "reject":
		task.Status = models.TaskRejected
	case "clarify":
		task.Status = models.TaskClarify
	case "complete":
		task.Status = models.TaskCompleted
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown action: " + action})
		return
	}

	h.Store.SaveTask(task)
	c.JSON(http.StatusOK, task)
}
