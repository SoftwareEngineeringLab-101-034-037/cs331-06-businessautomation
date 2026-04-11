package handler

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/models"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/storage"
)

type AnalyticsHandler struct {
	Store storage.Store
}

func NewAnalyticsHandler(store storage.Store) *AnalyticsHandler {
	return &AnalyticsHandler{Store: store}
}

type analyticsSummary struct {
	GeneratedAt            time.Time `json:"generated_at"`
	WorkflowsTotal         int       `json:"workflows_total"`
	WorkflowsActive        int       `json:"workflows_active"`
	TasksTotal             int       `json:"tasks_total"`
	TasksOpen              int       `json:"tasks_open"`
	TasksPending           int       `json:"tasks_pending"`
	TasksInProgress        int       `json:"tasks_in_progress"`
	TasksResolved          int       `json:"tasks_resolved"`
	TasksOverdue           int       `json:"tasks_overdue"`
	TasksOverduePending    int       `json:"tasks_overdue_pending"`
	TasksOverdueInProgress int       `json:"tasks_overdue_in_progress"`
	TasksEscalated         int       `json:"tasks_escalated"`
	InstancesTotal         int       `json:"instances_total"`
	InstancesActive        int       `json:"instances_active"`
	InstancesCompleted     int       `json:"instances_completed"`
	InstancesFailed        int       `json:"instances_failed"`
	TaskResolutionRate     int       `json:"task_resolution_rate"`
	InstanceSuccessRate    int       `json:"instance_success_rate"`
	AvgLeadHours           float64   `json:"avg_lead_hours"`
	BacklogDelta24h        int       `json:"backlog_delta_24h"`
}

type statusSliceItem struct {
	Status string `json:"status"`
	Label  string `json:"label"`
	Count  int    `json:"count"`
	Pct    int    `json:"pct"`
}

type prioritySliceItem struct {
	Priority string `json:"priority"`
	Count    int    `json:"count"`
	Pct      int    `json:"pct"`
}

type queueAging struct {
	LtHalfSLA         int `json:"lt_half_sla"`
	LtSLA             int `json:"lt_sla"`
	Between1And2_5SLA int `json:"between_1_and_2_5_sla"`
	Gt2_5SLA          int `json:"gt_2_5_sla"`
	OverdueOpen       int `json:"overdue_open"`
}

type throughputDay struct {
	Key              string `json:"key"`
	Label            string `json:"label"`
	TasksResolved    int    `json:"tasks_resolved"`
	InstancesStarted int    `json:"instances_started"`
}

type workflowRollup struct {
	WorkflowID      string  `json:"workflow_id"`
	WorkflowName    string  `json:"workflow_name"`
	TotalTasks      int     `json:"total_tasks"`
	OpenTasks       int     `json:"open_tasks"`
	ClosedTasks     int     `json:"closed_tasks"`
	ResolvedTasks   int     `json:"resolved_tasks"`
	OverdueTasks    int     `json:"overdue_tasks"`
	FailedInstances int     `json:"failed_instances"`
	ResolutionRate  int     `json:"resolution_rate"`
	AvgLeadHours    float64 `json:"avg_lead_hours"`
}

type failureReason struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type failureNode struct {
	NodeID string `json:"node_id"`
	Count  int    `json:"count"`
}

type auditSnippetEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	NodeID    string    `json:"node_id,omitempty"`
	Actor     string    `json:"actor,omitempty"`
	Message   string    `json:"message,omitempty"`
}

type analyticsFilters struct {
	WorkflowID           string     `json:"workflow_id,omitempty"`
	NodeID               string     `json:"node_id,omitempty"`
	Reason               string     `json:"reason,omitempty"`
	Since                *time.Time `json:"since,omitempty"`
	Until                *time.Time `json:"until,omitempty"`
	Window               string     `json:"window,omitempty"`
	LimitProblemTasks    int        `json:"limit_problem_tasks"`
	LimitFailedInstances int        `json:"limit_failed_instances"`
	reasonMatch          string     `json:"-"`
}

type failureSnapshot struct {
	NodeID       string
	Reason       string
	FailedAt     *time.Time
	AuditSnippet []auditSnippetEntry
}

type failedInstance struct {
	InstanceID   string              `json:"instance_id"`
	WorkflowID   string              `json:"workflow_id"`
	WorkflowName string              `json:"workflow_name"`
	Status       string              `json:"status"`
	StartedAt    time.Time           `json:"started_at"`
	NodeID       string              `json:"node_id,omitempty"`
	Error        string              `json:"error,omitempty"`
	FailedAt     *time.Time          `json:"failed_at,omitempty"`
	AuditSnippet []auditSnippetEntry `json:"audit_snippet,omitempty"`
}

type taskProblem struct {
	TaskID        string     `json:"task_id"`
	Title         string     `json:"title,omitempty"`
	Description   string     `json:"description,omitempty"`
	WorkflowID    string     `json:"workflow_id"`
	WorkflowName  string     `json:"workflow_name"`
	Status        string     `json:"status"`
	DisplayStatus string     `json:"display_status"`
	Priority      string     `json:"priority"`
	CreatedAt     time.Time  `json:"created_at"`
	DueAt         time.Time  `json:"due_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	InstanceID    string     `json:"instance_id"`
	NodeID        string     `json:"node_id"`
	AssignedUser  string     `json:"assigned_user,omitempty"`
	AssignedRole  string     `json:"assigned_role,omitempty"`
	AgeHours      float64    `json:"age_hours"`
	IsOverdue     bool       `json:"is_overdue"`
}

type taskTypeRollup struct {
	TaskType     string         `json:"task_type"`
	NodeID       string         `json:"node_id"`
	WaitingCount int            `json:"waiting_count"`
	FailedCount  int            `json:"failed_count"`
	TotalCount   int            `json:"total_count"`
	ActionCounts map[string]int `json:"action_counts"`
}

type analyticsResponse struct {
	Filters          analyticsFilters    `json:"filters"`
	OrgID            string              `json:"org_id"`
	Summary          analyticsSummary    `json:"summary"`
	StatusDist       []statusSliceItem   `json:"status_distribution"`
	PriorityDistOpen []prioritySliceItem `json:"priority_distribution_open"`
	QueueAging       queueAging          `json:"queue_aging"`
	Throughput7d     []throughputDay     `json:"throughput_7d"`
	WorkflowRollups  []workflowRollup    `json:"workflow_rollups"`
	FailureReasons   []failureReason     `json:"failure_reasons"`
	FailureNodes     []failureNode       `json:"failure_nodes"`
	FailedInstances  []failedInstance    `json:"failed_instances"`
	ProblemTasks     []taskProblem       `json:"problem_tasks"`
	TaskTypeRollups  []taskTypeRollup    `json:"task_type_rollups"`
}

func (h *AnalyticsHandler) Get(c *gin.Context) {
	orgID := c.Param("orgId")
	now := time.Now()

	filters, err := parseAnalyticsFilters(c, now)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	workflows, err := h.Store.ListWorkflows(orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load workflows"})
		return
	}

	workflowNameByID := make(map[string]string, len(workflows))
	workflowByID := make(map[string]models.Workflow, len(workflows))
	for _, wf := range workflows {
		workflowNameByID[wf.ID] = wf.Name
		workflowByID[wf.ID] = wf
	}

	scopedWorkflowIDs := make(map[string]struct{}, len(workflows))
	for _, wf := range workflows {
		if filters.WorkflowID != "" && !sameFold(wf.ID, filters.WorkflowID) {
			continue
		}
		scopedWorkflowIDs[wf.ID] = struct{}{}
	}

	allInstances := make([]models.Instance, 0)
	taskByID := make(map[string]models.TaskAssignment)

	instancesByOrg, listInstancesErr := h.Store.ListInstancesByOrg(orgID)
	if listInstancesErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load workflow instances"})
		return
	}

	scopedInstanceIDs := make(map[string]struct{}, len(instancesByOrg))
	for _, inst := range instancesByOrg {
		if inst.OrgID != orgID {
			continue
		}
		if _, ok := scopedWorkflowIDs[inst.WorkflowID]; !ok {
			continue
		}
		if !withinRange(inst.StartedAt, filters.Since, filters.Until) {
			continue
		}
		allInstances = append(allInstances, inst)
		scopedInstanceIDs[inst.ID] = struct{}{}
	}

	tasksByOrg, listTasksErr := h.Store.ListTasksByOrg(orgID)
	if listTasksErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load instance tasks"})
		return
	}
	for _, task := range tasksByOrg {
		if task.OrgID != orgID {
			continue
		}
		if _, ok := scopedWorkflowIDs[task.WorkflowID]; !ok {
			continue
		}
		if _, ok := scopedInstanceIDs[task.InstanceID]; !ok {
			continue
		}
		if !withinRange(task.CreatedAt, filters.Since, filters.Until) {
			continue
		}
		taskByID[task.ID] = task
	}

	allTasks := make([]models.TaskAssignment, 0, len(taskByID))
	for _, task := range taskByID {
		allTasks = append(allTasks, task)
	}

	instanceSnapshots := make(map[string]failureSnapshot, len(allInstances))
	for _, inst := range allInstances {
		instanceSnapshots[inst.ID] = buildFailureSnapshot(inst)
	}

	filteredInstances := filterInstances(allInstances, instanceSnapshots, filters)
	allowedInstanceIDs := make(map[string]struct{}, len(filteredInstances))
	for _, inst := range filteredInstances {
		allowedInstanceIDs[inst.ID] = struct{}{}
	}

	filteredTasks := filterTasks(allTasks, allowedInstanceIDs, filters)

	scopedWorkflows := make([]models.Workflow, 0, len(workflows))
	for _, wf := range workflows {
		if _, ok := scopedWorkflowIDs[wf.ID]; !ok {
			continue
		}
		scopedWorkflows = append(scopedWorkflows, wf)
	}

	summary := analyticsSummary{
		GeneratedAt:     now,
		WorkflowsTotal:  len(scopedWorkflows),
		WorkflowsActive: 0,
		TasksTotal:      len(filteredTasks),
	}

	for _, wf := range scopedWorkflows {
		if strings.EqualFold(string(wf.Status), "active") {
			summary.WorkflowsActive += 1
		}
	}

	statusCounts := map[string]int{
		"pending":     0,
		"in_progress": 0,
		"overdue":     0,
		"completed":   0,
		"rejected":    0,
		"escalated":   0,
		"cancelled":   0,
		"sent_back":   0,
	}
	priorityCountsOpen := map[string]int{"low": 0, "medium": 0, "high": 0, "critical": 0}
	resolvedByWorkflowLead := make(map[string][]float64)
	workflowRollupMap := make(map[string]*workflowRollup)
	taskTypeRollupMap := make(map[string]*taskTypeRollup)
	problemTasks := make([]taskProblem, 0)

	created24h := 0
	resolved24h := 0
	leadHoursTotal := 0.0
	leadHoursCount := 0

	for _, task := range filteredTasks {
		baseStatus := mapTaskStatus(task.Status)
		dueAt := task.CreatedAt.Add(defaultSLADuration(task.SLADays))
		isOverdue := (baseStatus == "pending" || baseStatus == "in_progress") && dueAt.Before(now)
		displayStatus := baseStatus
		if isOverdue {
			displayStatus = "overdue"
		}

		priority := priorityFromSLA(task.SLADays)
		ageHours := now.Sub(task.CreatedAt).Hours()

		// statusCounts intentionally double-counts overdue as a subset bucket; statusDist
		// therefore shows overlapping percentages where overdue can exceed 100% when summed.
		statusCounts[baseStatus] += 1
		if isOverdue {
			statusCounts["overdue"] += 1
		}

		if strings.EqualFold(baseStatus, "pending") {
			summary.TasksPending += 1
			summary.TasksOpen += 1
		}
		if strings.EqualFold(baseStatus, "in_progress") {
			summary.TasksInProgress += 1
			summary.TasksOpen += 1
		}
		if displayStatus == "overdue" {
			summary.TasksOverdue += 1
			if baseStatus == "pending" {
				summary.TasksOverduePending += 1
			}
			if baseStatus == "in_progress" {
				summary.TasksOverdueInProgress += 1
			}
		}
		if displayStatus == "escalated" {
			summary.TasksEscalated += 1
		}
		if isResolvedStatus(baseStatus) {
			summary.TasksResolved += 1
		}

		taskTypeName := strings.TrimSpace(task.Title)
		if taskTypeName == "" {
			taskTypeName = strings.TrimSpace(task.NodeID)
		}
		if taskTypeName == "" {
			taskTypeName = "Task"
		}
		taskTypeKey := strings.ToLower(strings.TrimSpace(task.NodeID + "::" + taskTypeName))
		taskTypeRow := taskTypeRollupMap[taskTypeKey]
		if taskTypeRow == nil {
			taskTypeRow = &taskTypeRollup{
				TaskType:     taskTypeName,
				NodeID:       strings.TrimSpace(task.NodeID),
				ActionCounts: map[string]int{},
			}
			taskTypeRollupMap[taskTypeKey] = taskTypeRow
		}
		taskTypeRow.TotalCount += 1
		if baseStatus == "pending" || baseStatus == "in_progress" {
			taskTypeRow.WaitingCount += 1
		}
		if baseStatus == "rejected" || baseStatus == "sent_back" || baseStatus == "cancelled" || baseStatus == "escalated" {
			taskTypeRow.FailedCount += 1
		}
		actionKey := strings.ToLower(strings.TrimSpace(task.ActionCommitted))
		if actionKey == "" && isResolvedStatus(baseStatus) {
			actionKey = baseStatus
		}
		if actionKey != "" {
			taskTypeRow.ActionCounts[actionKey] += 1
		}

		if (baseStatus == "pending" || baseStatus == "in_progress") && ageHours > 0 {
			priorityCountsOpen[priority] += 1
		}

		rollup := workflowRollupMap[task.WorkflowID]
		if rollup == nil {
			rollup = &workflowRollup{
				WorkflowID:   task.WorkflowID,
				WorkflowName: workflowNameByID[task.WorkflowID],
			}
			if rollup.WorkflowName == "" {
				rollup.WorkflowName = "Workflow"
			}
			workflowRollupMap[task.WorkflowID] = rollup
		}
		rollup.TotalTasks += 1
		if baseStatus == "pending" || baseStatus == "in_progress" {
			rollup.OpenTasks += 1
		}
		if baseStatus == "completed" {
			rollup.ClosedTasks += 1
		}
		if isResolvedStatus(baseStatus) {
			rollup.ResolvedTasks += 1
		}
		if displayStatus == "overdue" || displayStatus == "escalated" {
			rollup.OverdueTasks += 1
		}

		if task.CreatedAt.After(now.Add(-24 * time.Hour)) {
			created24h += 1
		}
		if task.CompletedAt != nil && task.CompletedAt.After(now.Add(-24*time.Hour)) {
			resolved24h += 1
		}

		if task.CompletedAt != nil {
			leadHours := task.CompletedAt.Sub(task.CreatedAt).Hours()
			if leadHours > 0 {
				leadHoursTotal += leadHours
				leadHoursCount += 1
				resolvedByWorkflowLead[task.WorkflowID] = append(resolvedByWorkflowLead[task.WorkflowID], leadHours)
			}
		}

		if displayStatus == "overdue" || displayStatus == "escalated" {
			problemTasks = append(problemTasks, taskProblem{
				TaskID:        task.ID,
				Title:         strings.TrimSpace(task.Title),
				Description:   strings.TrimSpace(task.Description),
				WorkflowID:    task.WorkflowID,
				WorkflowName:  fallbackWorkflowName(workflowNameByID, task.WorkflowID),
				Status:        baseStatus,
				DisplayStatus: displayStatus,
				Priority:      priority,
				CreatedAt:     task.CreatedAt,
				DueAt:         dueAt,
				CompletedAt:   task.CompletedAt,
				InstanceID:    task.InstanceID,
				NodeID:        task.NodeID,
				AssignedUser:  strings.TrimSpace(task.AssignedUser),
				AssignedRole:  strings.TrimSpace(task.AssignedRole),
				AgeHours:      ageHours,
				IsOverdue:     isOverdue,
			})
		}
	}

	if summary.TasksTotal > 0 {
		summary.TaskResolutionRate = pct(summary.TasksResolved, summary.TasksTotal)
	}
	if leadHoursCount > 0 {
		summary.AvgLeadHours = leadHoursTotal / float64(leadHoursCount)
	}
	summary.BacklogDelta24h = created24h - resolved24h

	instancesCompleted := 0
	instancesFailed := 0
	instancesActive := 0
	failureReasons := make(map[string]int)
	failureNodes := make(map[string]int)
	failedList := make([]failedInstance, 0)

	for _, inst := range filteredInstances {
		status := strings.ToLower(string(inst.Status))
		switch status {
		case "completed":
			instancesCompleted += 1
		case "failed":
			instancesFailed += 1
		case "running", "waiting", "pending":
			instancesActive += 1
		}

		snapshot := instanceSnapshots[inst.ID]

		if status == "failed" {
			nodeID := snapshot.NodeID
			reason := snapshot.Reason
			failedAt := snapshot.FailedAt
			if reason == "" {
				reason = "unknown failure"
			}
			failureReasons[reason] += 1
			if nodeID != "" {
				failureNodes[nodeID] += 1
			}

			failedList = append(failedList, failedInstance{
				InstanceID:   inst.ID,
				WorkflowID:   inst.WorkflowID,
				WorkflowName: fallbackWorkflowName(workflowNameByID, inst.WorkflowID),
				Status:       status,
				StartedAt:    inst.StartedAt,
				NodeID:       nodeID,
				Error:        reason,
				FailedAt:     failedAt,
				AuditSnippet: snapshot.AuditSnippet,
			})
		}

		rollup := workflowRollupMap[inst.WorkflowID]
		if rollup == nil {
			rollup = &workflowRollup{WorkflowID: inst.WorkflowID, WorkflowName: fallbackWorkflowName(workflowNameByID, inst.WorkflowID)}
			workflowRollupMap[inst.WorkflowID] = rollup
		}
		if status == "failed" {
			rollup.FailedInstances += 1
		}
	}

	summary.InstancesTotal = len(filteredInstances)
	summary.InstancesCompleted = instancesCompleted
	summary.InstancesFailed = instancesFailed
	summary.InstancesActive = instancesActive
	if instancesCompleted+instancesFailed > 0 {
		summary.InstanceSuccessRate = pct(instancesCompleted, instancesCompleted+instancesFailed)
	}

	statusOrder := []string{"pending", "in_progress", "overdue", "escalated", "completed", "rejected", "sent_back", "cancelled"}
	statusLabels := map[string]string{
		"pending":     "Pending",
		"in_progress": "In Progress",
		"overdue":     "Overdue (subset of open)",
		"escalated":   "Escalated",
		"completed":   "Completed",
		"rejected":    "Rejected",
		"sent_back":   "Sent Back",
		"cancelled":   "Cancelled",
	}
	statusDist := make([]statusSliceItem, 0, len(statusOrder))
	for _, key := range statusOrder {
		count := statusCounts[key]
		statusDist = append(statusDist, statusSliceItem{
			Status: key,
			Label:  statusLabels[key],
			Count:  count,
			Pct:    pct(count, summary.TasksTotal),
		})
	}

	priorityOrder := []string{"critical", "high", "medium", "low"}
	priorityDist := make([]prioritySliceItem, 0, len(priorityOrder))
	for _, key := range priorityOrder {
		count := priorityCountsOpen[key]
		priorityDist = append(priorityDist, prioritySliceItem{
			Priority: key,
			Count:    count,
			Pct:      pct(count, summary.TasksOpen),
		})
	}

	aging := queueAging{}
	for _, task := range filteredTasks {
		baseStatus := mapTaskStatus(task.Status)
		if baseStatus != "pending" && baseStatus != "in_progress" {
			continue
		}
		ageHours := now.Sub(task.CreatedAt).Hours()
		slaHours := defaultSLADuration(task.SLADays).Hours()
		if slaHours <= 0 {
			slaHours = 48
		}
		ratio := ageHours / slaHours

		if ratio >= 1.0 {
			aging.OverdueOpen += 1
		}

		if ratio < 0.5 {
			aging.LtHalfSLA += 1
		} else if ratio < 1.0 {
			aging.LtSLA += 1
		} else if ratio <= 2.5 {
			aging.Between1And2_5SLA += 1
		} else {
			aging.Gt2_5SLA += 1
		}
	}

	throughput := buildThroughput7d(now, filteredTasks, filteredInstances)

	rollups := make([]workflowRollup, 0, len(workflowRollupMap))
	for _, row := range workflowRollupMap {
		row.ResolutionRate = pct(row.ResolvedTasks, row.TotalTasks)
		lead := resolvedByWorkflowLead[row.WorkflowID]
		if len(lead) > 0 {
			sum := 0.0
			for _, h := range lead {
				sum += h
			}
			row.AvgLeadHours = sum / float64(len(lead))
		}
		rollups = append(rollups, *row)
	}
	sort.Slice(rollups, func(i, j int) bool {
		if rollups[i].OverdueTasks == rollups[j].OverdueTasks {
			return rollups[i].TotalTasks > rollups[j].TotalTasks
		}
		return rollups[i].OverdueTasks > rollups[j].OverdueTasks
	})

	reasonList := make([]failureReason, 0, len(failureReasons))
	for reason, count := range failureReasons {
		reasonList = append(reasonList, failureReason{Reason: reason, Count: count})
	}
	sort.Slice(reasonList, func(i, j int) bool { return reasonList[i].Count > reasonList[j].Count })
	if len(reasonList) > 10 {
		reasonList = reasonList[:10]
	}

	nodeList := make([]failureNode, 0, len(failureNodes))
	for nodeID, count := range failureNodes {
		nodeList = append(nodeList, failureNode{NodeID: nodeID, Count: count})
	}
	sort.Slice(nodeList, func(i, j int) bool { return nodeList[i].Count > nodeList[j].Count })
	if len(nodeList) > 10 {
		nodeList = nodeList[:10]
	}

	sort.Slice(failedList, func(i, j int) bool {
		left := failedList[i].FailedAt
		right := failedList[j].FailedAt
		if left == nil {
			return false
		}
		if right == nil {
			return true
		}
		return left.After(*right)
	})
	if len(failedList) > filters.LimitFailedInstances {
		failedList = failedList[:filters.LimitFailedInstances]
	}

	sort.Slice(problemTasks, func(i, j int) bool {
		if problemTasks[i].DueAt.Equal(problemTasks[j].DueAt) {
			return problemTasks[i].AgeHours > problemTasks[j].AgeHours
		}
		return problemTasks[i].DueAt.Before(problemTasks[j].DueAt)
	})
	if len(problemTasks) > filters.LimitProblemTasks {
		problemTasks = problemTasks[:filters.LimitProblemTasks]
	}

	taskTypeRollups := make([]taskTypeRollup, 0, len(taskTypeRollupMap))
	for _, row := range taskTypeRollupMap {
		taskTypeRollups = append(taskTypeRollups, *row)
	}
	sort.Slice(taskTypeRollups, func(i, j int) bool {
		if taskTypeRollups[i].WaitingCount == taskTypeRollups[j].WaitingCount {
			if taskTypeRollups[i].FailedCount == taskTypeRollups[j].FailedCount {
				return taskTypeRollups[i].TotalCount > taskTypeRollups[j].TotalCount
			}
			return taskTypeRollups[i].FailedCount > taskTypeRollups[j].FailedCount
		}
		return taskTypeRollups[i].WaitingCount > taskTypeRollups[j].WaitingCount
	})

	c.JSON(http.StatusOK, analyticsResponse{
		Filters:          filters,
		OrgID:            orgID,
		Summary:          summary,
		StatusDist:       statusDist,
		PriorityDistOpen: priorityDist,
		QueueAging:       aging,
		Throughput7d:     throughput,
		WorkflowRollups:  rollups,
		FailureReasons:   reasonList,
		FailureNodes:     nodeList,
		FailedInstances:  failedList,
		ProblemTasks:     problemTasks,
		TaskTypeRollups:  taskTypeRollups,
	})
}

func parseAnalyticsFilters(c *gin.Context, now time.Time) (analyticsFilters, error) {
	filters := analyticsFilters{
		WorkflowID:           strings.TrimSpace(c.Query("workflow_id")),
		NodeID:               strings.TrimSpace(c.Query("node_id")),
		Reason:               strings.TrimSpace(c.Query("reason")),
		Window:               strings.ToLower(strings.TrimSpace(c.Query("window"))),
		LimitProblemTasks:    100,
		LimitFailedInstances: 25,
	}

	if raw := strings.TrimSpace(c.Query("limit_problem_tasks")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return filters, errBadFilter("limit_problem_tasks")
		}
		if v > 0 {
			if v > 500 {
				v = 500
			}
			filters.LimitProblemTasks = v
		}
	}

	if raw := strings.TrimSpace(c.Query("limit_failed_instances")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return filters, errBadFilter("limit_failed_instances")
		}
		if v > 0 {
			if v > 200 {
				v = 200
			}
			filters.LimitFailedInstances = v
		}
	}

	if filters.Window != "" {
		dur, err := parseWindow(filters.Window)
		if err != nil {
			return filters, errBadFilter("window")
		}
		since := now.Add(-dur)
		filters.Since = &since
	}

	if raw := strings.TrimSpace(c.Query("since")); raw != "" {
		t, err := parseTimeBound(raw, false)
		if err != nil {
			return filters, errBadFilter("since")
		}
		filters.Since = &t
	}

	if raw := strings.TrimSpace(c.Query("until")); raw != "" {
		t, err := parseTimeBound(raw, true)
		if err != nil {
			return filters, errBadFilter("until")
		}
		filters.Until = &t
	}

	if filters.Since != nil && filters.Until != nil && filters.Since.After(*filters.Until) {
		return filters, errBadFilter("since/until")
	}

	filters.reasonMatch = strings.ToLower(filters.Reason)
	return filters, nil
}

func errBadFilter(name string) error {
	return errors.New("invalid " + name + " filter")
}

func parseWindow(raw string) (time.Duration, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if strings.HasSuffix(raw, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(raw, "d"))
		if err != nil || days <= 0 {
			return 0, strconv.ErrSyntax
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(raw)
}

func parseTimeBound(raw string, upper bool) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	if d, err := time.Parse("2006-01-02", raw); err == nil {
		if upper {
			return d.Add(24*time.Hour - time.Nanosecond), nil
		}
		return d, nil
	}
	return time.Time{}, strconv.ErrSyntax
}

func withinRange(ts time.Time, since, until *time.Time) bool {
	if since != nil && ts.Before(*since) {
		return false
	}
	if until != nil && ts.After(*until) {
		return false
	}
	return true
}

func sameFold(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func filterInstances(instances []models.Instance, snapshots map[string]failureSnapshot, filters analyticsFilters) []models.Instance {
	out := make([]models.Instance, 0, len(instances))
	for _, inst := range instances {
		if filters.WorkflowID != "" && !sameFold(inst.WorkflowID, filters.WorkflowID) {
			continue
		}
		if filters.NodeID != "" {
			snapshot := snapshots[inst.ID]
			if !sameFold(snapshot.NodeID, filters.NodeID) && !sameFold(inst.CurrentNode, filters.NodeID) {
				continue
			}
		}
		if filters.reasonMatch != "" {
			snapshot := snapshots[inst.ID]
			if !strings.Contains(strings.ToLower(snapshot.Reason), filters.reasonMatch) {
				continue
			}
		}
		out = append(out, inst)
	}
	return out
}

func filterTasks(tasks []models.TaskAssignment, allowedInstanceIDs map[string]struct{}, filters analyticsFilters) []models.TaskAssignment {
	out := make([]models.TaskAssignment, 0, len(tasks))
	for _, task := range tasks {
		if filters.WorkflowID != "" && !sameFold(task.WorkflowID, filters.WorkflowID) {
			continue
		}
		if filters.NodeID != "" && !sameFold(task.NodeID, filters.NodeID) {
			continue
		}
		if filters.reasonMatch != "" {
			if _, ok := allowedInstanceIDs[task.InstanceID]; !ok {
				continue
			}
		}
		out = append(out, task)
	}
	return out
}

func buildFailureSnapshot(inst models.Instance) failureSnapshot {
	nodeID, reason, failedAt := lastFailureSnapshot(inst)
	status := strings.ToLower(string(inst.Status))

	if status != "failed" {
		reason = ""
		nodeID = ""
		failedAt = nil
	}

	if status == "failed" && strings.TrimSpace(reason) == "" {
		reason = "unknown failure"
	}

	return failureSnapshot{
		NodeID:       nodeID,
		Reason:       strings.TrimSpace(reason),
		FailedAt:     failedAt,
		AuditSnippet: buildAuditSnippet(inst.AuditLog, 6),
	}
}

func buildAuditSnippet(entries []models.AuditEntry, limit int) []auditSnippetEntry {
	if limit <= 0 || len(entries) == 0 {
		return nil
	}

	out := make([]auditSnippetEntry, 0, limit)
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		msg := extractErrorText(entry.Details["error"])
		if msg == "" {
			msg = extractErrorText(entry.Details["reason"])
		}
		if msg == "" {
			msg = extractErrorText(entry.Details["message"])
		}

		out = append(out, auditSnippetEntry{
			Timestamp: entry.Timestamp,
			Action:    entry.Action,
			NodeID:    entry.NodeID,
			Actor:     entry.Actor,
			Message:   msg,
		})

		if len(out) >= limit {
			break
		}
	}

	return out
}

func mapTaskStatus(status models.TaskStatus) string {
	switch strings.ToLower(string(status)) {
	case "pending":
		return "pending"
	case "in_progress":
		return "in_progress"
	case "approved", "completed":
		return "completed"
	case "rejected":
		return "rejected"
	case "clarification_requested":
		return "sent_back"
	case "cancelled":
		return "cancelled"
	case "escalated":
		return "escalated"
	default:
		return "in_progress"
	}
}

func priorityFromSLA(slaDays int) string {
	if slaDays <= 0 {
		return "medium"
	}
	if slaDays <= 1 {
		return "critical"
	}
	if slaDays <= 2 {
		return "high"
	}
	if slaDays <= 5 {
		return "medium"
	}
	return "low"
}

func defaultSLADuration(slaDays int) time.Duration {
	if slaDays > 0 {
		return time.Duration(slaDays) * 24 * time.Hour
	}
	return 48 * time.Hour
}

func isResolvedStatus(status string) bool {
	switch status {
	case "completed", "rejected", "sent_back", "cancelled":
		return true
	default:
		return false
	}
}

func pct(value, total int) int {
	if total <= 0 {
		return 0
	}
	return int((float64(value) / float64(total)) * 100.0)
}

func fallbackWorkflowName(m map[string]string, workflowID string) string {
	name := m[workflowID]
	if strings.TrimSpace(name) == "" {
		return "Workflow"
	}
	return name
}

func buildThroughput7d(now time.Time, tasks []models.TaskAssignment, instances []models.Instance) []throughputDay {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	rows := make([]throughputDay, 0, 7)
	idx := make(map[string]*throughputDay, 7)

	for i := 6; i >= 0; i-- {
		day := today.AddDate(0, 0, -i)
		key := day.Format("2006-01-02")
		row := throughputDay{
			Key:              key,
			Label:            day.Format("Jan 2"),
			TasksResolved:    0,
			InstancesStarted: 0,
		}
		rows = append(rows, row)
	}

	for i := range rows {
		idx[rows[i].Key] = &rows[i]
	}

	for _, task := range tasks {
		if task.CompletedAt == nil {
			continue
		}
		key := task.CompletedAt.Format("2006-01-02")
		if row := idx[key]; row != nil {
			row.TasksResolved += 1
		}
	}

	for _, inst := range instances {
		key := inst.StartedAt.Format("2006-01-02")
		if row := idx[key]; row != nil {
			row.InstancesStarted += 1
		}
	}

	return rows
}

func lastFailureSnapshot(inst models.Instance) (string, string, *time.Time) {
	var latestTime *time.Time
	nodeID := ""
	reason := ""

	for i := len(inst.AuditLog) - 1; i >= 0; i-- {
		entry := inst.AuditLog[i]
		action := strings.ToLower(entry.Action)
		if action != "instance_failed" && action != "action_failed" {
			continue
		}
		ts := entry.Timestamp
		latestTime = &ts
		nodeID = entry.NodeID
		details := entry.Details
		reason = extractErrorText(details["error"])
		if reason == "" {
			reason = extractErrorText(details["reason"])
		}
		if reason == "" {
			reason = "unknown failure"
		}
		return nodeID, reason, latestTime
	}

	for id, ns := range inst.NodeStates {
		if strings.ToLower(ns.Status) != "failed" {
			continue
		}
		nodeID = id
		reason = strings.TrimSpace(ns.Output)
		if reason == "" {
			reason = "node failed without output"
		}
		if ns.CompletedAt != nil {
			latestTime = ns.CompletedAt
		}
		break
	}

	if reason == "" {
		reason = "instance failed"
	}
	return nodeID, reason, latestTime
}

func extractErrorText(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]interface{}:
		if err, ok := v["error"]; ok {
			if s, ok := err.(string); ok {
				return strings.TrimSpace(s)
			}
		}
		if reason, ok := v["reason"]; ok {
			if s, ok := reason.(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}
