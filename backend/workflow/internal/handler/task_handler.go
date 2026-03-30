package handler

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/example/business-automation/backend/workflow/internal/executor"
	"github.com/example/business-automation/backend/workflow/internal/middleware"
	"github.com/example/business-automation/backend/workflow/internal/models"
	"github.com/example/business-automation/backend/workflow/internal/storage"
)

type TaskExecutor interface {
	CanActOnTask(actorUserID string, task models.TaskAssignment, action, authHeader string) error
	ContinueTask(taskID, actorUserID, action, comment, authHeader string) (models.TaskAssignment, error)
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
		log.Printf("task_handler.List org=%s role=%s assigned_user=%s instance_id=%s failed: %v", orgId, role, assignedUser, instanceID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
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
		log.Printf("task_handler.Action invalid JSON task_id=%q action=%q: %v", taskID, action, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}
	if action != "start" && strings.TrimSpace(body.Comment) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "comment is required for task actions"})
		return
	}
	actorUserID := middleware.GetUserID(c)
	authHeader := middleware.GetAuthorizationHeader(c)
	if actorUserID == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if h.Exec != nil {
		if err := h.Exec.CanActOnTask(actorUserID, task, action, authHeader); err != nil {
			writeTaskActionError(c, "task_handler.Action authorize", err)
			return
		}
	} else if err := canActOnTaskWithoutExecutor(actorUserID, task, action); err != nil {
		writeTaskActionError(c, "task_handler.Action authorize", err)
		return
	}
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
			log.Printf("task_handler.Action save org=%s task=%s action=%s failed: %v", orgId, taskID, action, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		c.JSON(http.StatusOK, task)
		return
	}

	updatedTask, err := h.Exec.ContinueTask(taskID, actorUserID, action, body.Comment, authHeader)
	if err != nil {
		writeTaskActionError(c, "task_handler.Action continue", err)
		return
	}
	c.JSON(http.StatusOK, updatedTask)
}

func writeTaskActionError(c *gin.Context, context string, err error) {
	switch {
	case errors.Is(err, executor.ErrTaskNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
	case errors.Is(err, executor.ErrForbiddenTaskAction), errors.Is(err, executor.ErrTaskClaimNotAllowed):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	case errors.Is(err, executor.ErrCommentRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": "comment is required for task actions"})
	case errors.Is(err, executor.ErrUnknownAction):
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown action"})
	case errors.Is(err, executor.ErrTaskAlreadyCompleted):
		c.JSON(http.StatusConflict, gin.H{"error": "task already completed"})
	case errors.Is(err, executor.ErrTaskConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "task was updated concurrently"})
	default:
		log.Printf("%s failed: %v", context, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

func canActOnTaskWithoutExecutor(actorUserID string, task models.TaskAssignment, action string) error {
	actorUserID = strings.TrimSpace(actorUserID)
	if actorUserID == "" {
		return executor.ErrForbiddenTaskAction
	}
	assignedUser := strings.TrimSpace(task.AssignedUser)
	if assignedUser != "" {
		if actorUserID != assignedUser {
			return executor.ErrForbiddenTaskAction
		}
		return nil
	}
	if action != "start" {
		return executor.ErrForbiddenTaskAction
	}
	return nil
}
