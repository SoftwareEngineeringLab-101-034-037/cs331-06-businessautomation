"use client";

import { useState, useMemo, useCallback } from "react";
import TaskDetailDrawer from "@/components/dashboard/TaskDetailDrawer";
import { MOCK_TASKS } from "@/lib/mock-data";
import type { Task, TaskStatus, TaskPriority } from "@/types/dashboard";
import { TASK_STATUS_CONFIG, PRIORITY_CONFIG } from "@/types/dashboard";

type FilterPriority = "all" | TaskPriority;

// Kanban column config — defines the order and grouping of columns
const KANBAN_COLUMNS: { key: TaskStatus | "overdue_escalated"; label: string; statuses: TaskStatus[] }[] = [
  { key: "pending", label: "Pending", statuses: ["pending"] },
  { key: "in_progress", label: "In Progress", statuses: ["in_progress"] },
  { key: "overdue_escalated", label: "Overdue / Escalated", statuses: ["overdue", "escalated"] },
  { key: "sent_back", label: "Sent Back", statuses: ["sent_back"] },
  { key: "completed", label: "Completed", statuses: ["completed"] },
];

const PRIORITY_COLORS: Record<TaskPriority, string> = {
  critical: "#ef4444",
  high: "#f97316",
  medium: "#f59e0b",
  low: "#22c55e",
};

export default function TasksPage() {
  const [search, setSearch] = useState("");
  const [priorityFilter, setPriorityFilter] = useState<FilterPriority>("all");
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);

  const handleSelectTask = useCallback((task: Task) => setSelectedTask(task), []);
  const handleCloseDrawer = useCallback(() => setSelectedTask(null), []);

  const filtered = useMemo(() => {
    return MOCK_TASKS.filter((t) => {
      if (priorityFilter !== "all" && t.priority !== priorityFilter) return false;
      if (search) {
        const q = search.toLowerCase();
        return (
          t.title.toLowerCase().includes(q) ||
          t.id.toLowerCase().includes(q) ||
          t.workflowName.toLowerCase().includes(q) ||
          t.departmentOrigin.toLowerCase().includes(q)
        );
      }
      return true;
    });
  }, [priorityFilter, search]);

  // Group tasks by column
  const columns = useMemo(() => {
    return KANBAN_COLUMNS.map((col) => ({
      ...col,
      tasks: filtered
        .filter((t) => col.statuses.includes(t.status))
        .sort((a, b) => {
          const p: Record<string, number> = { critical: 0, high: 1, medium: 2, low: 3 };
          return (p[a.priority] ?? 9) - (p[b.priority] ?? 9);
        }),
    }));
  }, [filtered]);

  const totalFiltered = filtered.length;

  return (
    <div className="dashboard-page" style={{ maxWidth: "100%", padding: "0 16px" }}>
      <div className="page-header">
        <div>
          <h2 className="page-title">My Tasks</h2>
          <p className="page-subtitle">{totalFiltered} task{totalFiltered !== 1 ? "s" : ""} across {columns.filter((c) => c.tasks.length > 0).length} columns</p>
        </div>
      </div>

      {/* Compact filters */}
      <div className="filters-bar" style={{ marginBottom: 16 }}>
        <div className="filter-search">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
            <path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
          </svg>
          <input
            type="text"
            placeholder="Search tasks..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="filter-search-input"
          />
        </div>
        <div className="filter-select-group">
          <label className="filter-label">Priority:</label>
          <select
            value={priorityFilter}
            onChange={(e) => setPriorityFilter(e.target.value as FilterPriority)}
            className="filter-select"
          >
            <option value="all">All</option>
            {(Object.keys(PRIORITY_CONFIG) as TaskPriority[]).map((p) => (
              <option key={p} value={p}>{PRIORITY_CONFIG[p].label}</option>
            ))}
          </select>
        </div>
      </div>

      {/* Kanban Board */}
      <div className="kanban-container">
        {columns.map((col) => (
          <div key={col.key} className="kanban-column">
            <div className="kanban-column-header">
              <span className="kanban-column-title">{col.label}</span>
              <span className="kanban-column-count">{col.tasks.length}</span>
            </div>
            <div className="kanban-column-body">
              {col.tasks.length > 0 ? (
                col.tasks.map((task) => (
                  <div
                    key={task.id}
                    className={`kanban-card kanban-priority-${task.priority}`}
                    onClick={() => handleSelectTask(task)}
                  >
                    <div className="kanban-card-header">
                      <span className="kanban-card-id">{task.id}</span>
                      <span
                        className="kanban-card-priority"
                        style={{ background: PRIORITY_COLORS[task.priority] }}
                      >
                        {PRIORITY_CONFIG[task.priority].label}
                      </span>
                    </div>
                    <h4 className="kanban-card-title">{task.title}</h4>
                    <p className="kanban-card-workflow">{task.workflowName}</p>
                    <div className="kanban-card-meta">
                      <div className="kanban-card-meta-item">
                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="12" height="12">
                          <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 21h16.5M4.5 3h15M5.25 3v18m13.5-18v18M9 6.75h1.5m-1.5 3h1.5m-1.5 3h1.5m3-6H15m-1.5 3H15m-1.5 3H15M9 21v-3.375c0-.621.504-1.125 1.125-1.125h3.75c.621 0 1.125.504 1.125 1.125V21" />
                        </svg>
                        {task.departmentOrigin}
                      </div>
                      <div className="kanban-card-meta-item">
                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="12" height="12">
                          <path strokeLinecap="round" strokeLinejoin="round" d="M6.75 3v2.25M17.25 3v2.25M3 18.75V7.5a2.25 2.25 0 0 1 2.25-2.25h13.5A2.25 2.25 0 0 1 21 7.5v11.25m-18 0A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75m-18 0v-7.5A2.25 2.25 0 0 1 5.25 9h13.5A2.25 2.25 0 0 1 21 11.25v7.5" />
                        </svg>
                        {new Date(task.dueDate).toLocaleDateString("en-US", { month: "short", day: "numeric" })}
                      </div>
                    </div>
                    {/* Progress bar */}
                    <div className="kanban-card-progress">
                      <div className="kanban-card-progress-bar">
                        <div
                          className="kanban-card-progress-fill"
                          style={{ width: `${(task.stepNumber / task.totalSteps) * 100}%` }}
                        />
                      </div>
                      <span className="kanban-card-progress-text">
                        {task.stepNumber}/{task.totalSteps}
                      </span>
                    </div>
                    {/* Assignee avatar */}
                    <div className="kanban-card-footer">
                      <div className="kanban-card-avatar" title={task.assignedToName}>
                        {task.assignedToName.split(" ").map((n: string) => n[0]).join("").slice(0, 2)}
                      </div>
                      <span className="kanban-card-assignee">{task.assignedToName}</span>
                    </div>
                  </div>
                ))
              ) : (
                <div className="kanban-empty">
                  <p>No tasks</p>
                </div>
              )}
            </div>
          </div>
        ))}
      </div>

      {/* Task detail drawer */}
      <TaskDetailDrawer
        task={selectedTask}
        isOpen={selectedTask !== null}
        onClose={handleCloseDrawer}
      />
    </div>
  );
}
