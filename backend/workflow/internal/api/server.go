package api

import (
"context"
"encoding/json"
"log"
"net/http"
"os"
"strings"
"time"

"github.com/example/business-automation/backend/workflow/internal/connectors"
"github.com/example/business-automation/backend/workflow/internal/executor"
"github.com/example/business-automation/backend/workflow/internal/models"
"github.com/example/business-automation/backend/workflow/internal/storage"
)

type Server struct {
store storage.Store
exec  *executor.Executor
email connectors.EmailConnector
mux   *http.ServeMux
}

func NewServer() *Server {
var store storage.Store
mongoURI := os.Getenv("MONGO_URI")
if mongoURI != "" {
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
if ms, err := storage.NewMongoStore(ctx, mongoURI); err == nil {
log.Printf("connected to mongo: %s", mongoURI)
store = ms
} else {
log.Printf("mongo error: %v - falling back to memory store", err)
}
}
if store == nil {
log.Println("using in-memory store")
store = storage.NewMemoryStore()
}
email := connectors.NewMockEmail()
exec := executor.NewExecutor(store, email)
s := &Server{store: store, exec: exec, email: email, mux: http.NewServeMux()}
s.routes()
return s
}

func (s *Server) Handler() http.Handler { return corsMiddleware(s.mux) }

func corsMiddleware(next http.Handler) http.Handler {
return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
w.Header().Set("Access-Control-Allow-Origin", "*")
w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
if r.Method == http.MethodOptions {
w.WriteHeader(http.StatusNoContent)
return
}
next.ServeHTTP(w, r)
})
}

func (s *Server) routes() {
s.mux.HandleFunc("/health", s.handleHealth)

// Workflow CRUD
s.mux.HandleFunc("/workflows", s.handleWorkflows)        // GET list, POST create
s.mux.HandleFunc("/workflows/", s.handleWorkflowByID)    // GET, PUT, DELETE /workflows/{id}

// Instance management
s.mux.HandleFunc("/instances", s.handleInstances)         // POST start
s.mux.HandleFunc("/instances/", s.handleInstanceByID)     // GET /instances/{id}

// Task management
s.mux.HandleFunc("/tasks", s.handleTasks)                 // GET ?role=...
s.mux.HandleFunc("/tasks/", s.handleTaskAction)           // PUT /tasks/{id}/approve, /tasks/{id}/reject
}

// -- Health --

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// -- Workflows --

func (s *Server) handleWorkflows(w http.ResponseWriter, r *http.Request) {
switch r.Method {
case http.MethodGet:
s.listWorkflows(w, r)
case http.MethodPost:
s.createWorkflow(w, r)
default:
http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}
}

func (s *Server) listWorkflows(w http.ResponseWriter, r *http.Request) {
wfs, err := s.store.ListWorkflows()
if err != nil {
http.Error(w, err.Error(), http.StatusInternalServerError)
return
}
writeJSON(w, http.StatusOK, wfs)
}

func (s *Server) createWorkflow(w http.ResponseWriter, r *http.Request) {
var wf models.Workflow
if err := json.NewDecoder(r.Body).Decode(&wf); err != nil {
http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
return
}
if wf.Name == "" {
http.Error(w, "name is required", http.StatusBadRequest)
return
}
now := time.Now()
if wf.Status == "" {
wf.Status = models.WorkflowActive
}
wf.CreatedAt = now
wf.UpdatedAt = now
if wf.Version == 0 && wf.Status != "draft" {
wf.Version = 1
}
id, err := s.store.SaveWorkflow(wf)
if err != nil {
http.Error(w, "save failed: "+err.Error(), http.StatusInternalServerError)
return
}
wf.ID = id
writeJSON(w, http.StatusCreated, map[string]interface{}{"id": id, "workflow": wf})
}

func (s *Server) handleWorkflowByID(w http.ResponseWriter, r *http.Request) {
id := extractID(r.URL.Path, "/workflows/")
if id == "" {
http.Error(w, "missing workflow id", http.StatusBadRequest)
return
}
switch r.Method {
case http.MethodGet:
wf, ok := s.store.GetWorkflow(id)
if !ok {
http.Error(w, "not found", http.StatusNotFound)
return
}
writeJSON(w, http.StatusOK, wf)
case http.MethodPut:
// Wrapper struct so we can receive commit_message without persisting it
var req struct {
models.Workflow
CommitMessage string `json:"commit_message"`
}
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
http.Error(w, "invalid JSON", http.StatusBadRequest)
return
}
wf := req.Workflow
wf.ID = id
wf.UpdatedAt = time.Now()
// Preserve fields from the existing document
if existing, ok := s.store.GetWorkflow(id); ok {
wf.CreatedAt = existing.CreatedAt
wf.CreatedBy = existing.CreatedBy
if wf.Status == "draft" {
wf.Version = 0
} else if wf.Version <= existing.Version {
wf.Version = existing.Version + 1
}
} else {
// First time seeing this ID — treat as new
wf.CreatedAt = wf.UpdatedAt
if wf.Status == "draft" {
wf.Version = 0
} else if wf.Version == 0 {
wf.Version = 1
}
}
// Audit log — commit message is NOT stored in the workflow document
if req.CommitMessage != "" {
log.Printf("[AUDIT] workflow %s (v%d) updated — %s", wf.ID, wf.Version, req.CommitMessage)
}
if _, err := s.store.SaveWorkflow(wf); err != nil {
http.Error(w, "save failed: "+err.Error(), http.StatusInternalServerError)
return
}
writeJSON(w, http.StatusOK, wf)
case http.MethodDelete:
if err := s.store.DeleteWorkflow(id); err != nil {
http.Error(w, err.Error(), http.StatusNotFound)
return
}
writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
default:
http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}
}

// -- Instances --

func (s *Server) handleInstances(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodPost {
http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
return
}
var req struct {
WorkflowID string                 `json:"workflow_id"`
Data       map[string]interface{} `json:"data"`
}
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
http.Error(w, "invalid JSON", http.StatusBadRequest)
return
}
wf, ok := s.store.GetWorkflow(req.WorkflowID)
if !ok {
http.Error(w, "workflow not found", http.StatusNotFound)
return
}
if wf.Status != models.WorkflowActive {
http.Error(w, "workflow is not active", http.StatusBadRequest)
return
}
instID, err := s.exec.StartInstance(wf, req.Data)
if err != nil {
http.Error(w, "start failed: "+err.Error(), http.StatusInternalServerError)
return
}
writeJSON(w, http.StatusCreated, map[string]string{"instance_id": instID})
}

func (s *Server) handleInstanceByID(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodGet {
http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
return
}
id := extractID(r.URL.Path, "/instances/")
if id == "" {
http.Error(w, "missing instance id", http.StatusBadRequest)
return
}
inst, ok := s.store.GetInstance(id)
if !ok {
http.Error(w, "not found", http.StatusNotFound)
return
}
writeJSON(w, http.StatusOK, inst)
}

// -- Tasks --

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodGet {
http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
return
}
role := r.URL.Query().Get("role")
instanceID := r.URL.Query().Get("instance_id")
var tasks []models.TaskAssignment
var err error
if instanceID != "" {
tasks, err = s.store.ListTasksByInstance(instanceID)
} else if role != "" {
tasks, err = s.store.ListTasksByRole(role)
} else {
http.Error(w, "provide ?role= or ?instance_id=", http.StatusBadRequest)
return
}
if err != nil {
http.Error(w, err.Error(), http.StatusInternalServerError)
return
}
if tasks == nil {
tasks = []models.TaskAssignment{}
}
writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleTaskAction(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodPut {
http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
return
}
// Path: /tasks/{id}/{action}  where action is approve | reject | clarify | complete
path := strings.TrimPrefix(r.URL.Path, "/tasks/")
parts := strings.SplitN(path, "/", 2)
if len(parts) < 2 {
http.Error(w, "expected /tasks/{id}/{action}", http.StatusBadRequest)
return
}
taskID := parts[0]
action := parts[1]

task, ok := s.store.GetTask(taskID)
if !ok {
http.Error(w, "task not found", http.StatusNotFound)
return
}

var body struct {
Comment string `json:"comment"`
}
json.NewDecoder(r.Body).Decode(&body)

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
http.Error(w, "unknown action: "+action, http.StatusBadRequest)
return
}

s.store.SaveTask(task)
writeJSON(w, http.StatusOK, task)
}

// -- Helpers --

func extractID(path, prefix string) string {
id := strings.TrimPrefix(path, prefix)
// Remove any trailing sub-path
if idx := strings.Index(id, "/"); idx != -1 {
id = id[:idx]
}
return id
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(status)
json.NewEncoder(w).Encode(v)
}
