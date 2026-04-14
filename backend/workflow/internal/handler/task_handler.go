package handler

import (
	"errors"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/executor"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/middleware"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/models"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/storage"
)

type TaskExecutor interface {
	CanActOnTask(actorUserID string, task models.TaskAssignment, action, authHeader string) error
	ContinueTask(taskID, actorUserID, action, comment, authHeader string) (models.TaskAssignment, error)
	ListEscalationCandidates(task models.TaskAssignment, authHeader string) ([]string, error)
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
	role := strings.TrimSpace(c.Query("role"))
	rolesCSV := strings.TrimSpace(c.Query("roles"))
	instanceID := strings.TrimSpace(c.Query("instance_id"))
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
	case rolesCSV != "":
		roles := parseCSVValues(rolesCSV)
		if len(roles) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "roles cannot be empty"})
			return
		}
		roleTasks, e := h.Store.ListTasksByRoles(orgId, roles)
		if e != nil {
			err = e
			break
		}
		byID := make(map[string]models.TaskAssignment)
		for _, t := range roleTasks {
			byID[t.ID] = t
		}
		tasks = make([]models.TaskAssignment, 0, len(byID))
		for _, task := range byID {
			tasks = append(tasks, task)
		}
	case role != "":
		tasks, err = h.Store.ListTasksByRole(orgId, role)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "provide ?role=, ?roles=, ?assigned_user=, or ?instance_id="})
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
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})
	for i := range tasks {
		tasks[i] = sanitizeTaskForResponse(tasks[i])
	}
	c.JSON(http.StatusOK, tasks)
}

func parseCSVValues(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

// PUT /api/orgs/:orgId/tasks/:id/:action
// actions:
//   - start (pending -> in_progress)
//   - approve | reject | clarify | complete
//   - escalate_notify | escalate_reassign (admin only, pending tasks)
func (h *TaskHandler) Action(c *gin.Context) {
	orgId := c.Param("orgId")
	taskID := c.Param("id")
	action := strings.TrimSpace(c.Param("action"))
	isEscalationAction := action == "escalate_notify" || action == "escalate_reassign"

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
	if isEscalationAction && !strings.EqualFold(middleware.GetOrgRole(c), "admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if h.Exec == nil && isEscalationAction {
		c.JSON(http.StatusBadRequest, gin.H{"error": "escalation actions require executor"})
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
		c.JSON(http.StatusOK, sanitizeTaskForResponse(task))
		return
	}

	updatedTask, err := h.Exec.ContinueTask(taskID, actorUserID, action, body.Comment, authHeader)
	if err != nil {
		writeTaskActionError(c, "task_handler.Action continue", err)
		return
	}
	c.JSON(http.StatusOK, sanitizeTaskForResponse(updatedTask))
}

// GET /api/orgs/:orgId/tasks/:id/escalation-candidates
func (h *TaskHandler) EscalationCandidates(c *gin.Context) {
	orgID := c.Param("orgId")
	taskID := c.Param("id")

	task, ok := h.Store.GetTask(taskID)
	if !ok || task.OrgID != orgID {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	if !strings.EqualFold(middleware.GetOrgRole(c), "admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if task.Status != models.TaskPending {
		c.JSON(http.StatusOK, gin.H{"candidates": []string{}})
		return
	}
	actorUserID := middleware.GetUserID(c)
	if strings.TrimSpace(actorUserID) == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	authHeader := middleware.GetAuthorizationHeader(c)
	if h.Exec == nil {
		log.Printf("task_handler.EscalationCandidates org=%s task=%s skipped: executor unavailable", orgID, taskID)
		c.JSON(http.StatusOK, gin.H{"candidates": []string{}})
		return
	}

	if err := h.Exec.CanActOnTask(actorUserID, task, "escalate_reassign", authHeader); err != nil {
		writeTaskActionError(c, "task_handler.EscalationCandidates authorize", err)
		return
	}

	candidates, err := h.Exec.ListEscalationCandidates(task, authHeader)
	if err != nil {
		log.Printf("task_handler.EscalationCandidates org=%s task=%s failed: %v", orgID, taskID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	filtered := make([]string, 0, len(candidates))
	currentAssignee := strings.TrimSpace(task.AssignedUser)
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}
		if currentAssignee != "" && strings.EqualFold(trimmed, currentAssignee) {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	c.JSON(http.StatusOK, gin.H{"candidates": filtered})
}

func sanitizeTaskForResponse(task models.TaskAssignment) models.TaskAssignment {
	return task
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
	case errors.Is(err, executor.ErrNoEligibleAssignee):
		c.JSON(http.StatusConflict, gin.H{"error": "no eligible assignee found"})
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

	for _, userID := range task.AssignedUsers {
		if strings.EqualFold(strings.TrimSpace(userID), actorUserID) {
			return nil
		}
	}

	if len(task.AssignedRoles) > 0 || strings.TrimSpace(task.AssignedRole) != "" {
		// Without executor role-directory lookups, we cannot verify role membership safely.
		return executor.ErrForbiddenTaskAction
	}

	return nil
}
