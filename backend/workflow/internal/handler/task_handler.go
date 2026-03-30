package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/example/business-automation/backend/workflow/internal/middleware"
	"github.com/example/business-automation/backend/workflow/internal/models"
	"github.com/example/business-automation/backend/workflow/internal/storage"
)

type TaskExecutor interface {
	ContinueTask(taskID, actorUserID, action, comment string) (models.TaskAssignment, error)
}

// TaskHandler handles listing and actioning task assignments.
type TaskHandler struct {
	Store storage.Store
	Exec  TaskExecutor
}

func NewTaskHandler(store storage.Store, exec TaskExecutor) *TaskHandler {
	return &TaskHandler{Store: store, Exec: exec}
}

// GET /api/orgs/:orgId/tasks?role=... or ?instance_id=... or ?assigned_user=...
func (h *TaskHandler) List(c *gin.Context) {
	orgId := c.Param("orgId")
	role := c.Query("role")
	instanceID := c.Query("instance_id")
	assignedUser := ""
	if c.Query("assigned_user") != "" {
		assignedUser = middleware.GetUserID(c)
	}

	var tasks []models.TaskAssignment
	var err error

	switch {
	case instanceID != "":
		all, e := h.Store.ListTasksByInstance(instanceID)
		if e != nil {
			err = e
			break
		}
		for _, t := range all {
			if t.OrgID == orgId {
				tasks = append(tasks, t)
			}
		}
	case assignedUser != "":
		tasks, err = h.Store.ListTasksByAssignee(orgId, assignedUser)
	case role != "":
		tasks, err = h.Store.ListTasksByRole(orgId, role)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "provide ?role=, ?assigned_user=, or ?instance_id="})
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

// PUT /api/orgs/:orgId/tasks/:id/:action  (action = approve | reject | clarify | complete)
func (h *TaskHandler) Action(c *gin.Context) {
	orgId := c.Param("orgId")
	taskID := c.Param("id")
	action := c.Param("action")

	task, ok := h.Store.GetTask(taskID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	if task.OrgID != orgId {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	var body struct {
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON: " + err.Error()})
		return
	}
	if action != "start" && strings.TrimSpace(body.Comment) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "comment is required for task actions"})
		return
	}
	actorUserID := middleware.GetUserID(c)
	if h.Exec == nil {
		now := time.Now()
		task.Comment = strings.TrimSpace(body.Comment)
		task.CompletedAt = &now

		switch action {
		case "start":
			if strings.TrimSpace(task.AssignedUser) == "" {
				task.AssignedUser = actorUserID
			}
			task.Comment = ""
			task.Status = models.TaskInProgress
			task.CompletedAt = nil
		case "approve":
			task.Status = models.TaskCompleted
			task.ActionCommitted = action
		case "reject":
			task.Status = models.TaskCompleted
			task.ActionCommitted = action
		case "clarify":
			task.Status = models.TaskCompleted
			task.ActionCommitted = action
		case "complete":
			task.Status = models.TaskCompleted
			task.ActionCommitted = action
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown action: " + action})
			return
		}

		if _, err := h.Store.SaveTask(task); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save task: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, task)
		return
	}

	updatedTask, err := h.Exec.ContinueTask(taskID, actorUserID, action, body.Comment)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updatedTask)
}
