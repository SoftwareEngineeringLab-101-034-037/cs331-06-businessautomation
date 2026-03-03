package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/example/business-automation/backend/workflow/internal/models"
)

type MemoryStore struct {
	mu        sync.RWMutex
	workflows map[string]models.Workflow
	instances map[string]models.Instance
	tasks     map[string]models.TaskAssignment
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		workflows: make(map[string]models.Workflow),
		instances: make(map[string]models.Instance),
		tasks:     make(map[string]models.TaskAssignment),
	}
}

func genID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ── Workflows ──

func (m *MemoryStore) SaveWorkflow(w models.Workflow) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w.ID == "" {
		w.ID = genID()
	}
	m.workflows[w.ID] = w
	return w.ID, nil
}

func (m *MemoryStore) GetWorkflow(id string) (models.Workflow, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	w, ok := m.workflows[id]
	return w, ok
}

func (m *MemoryStore) ListWorkflows() ([]models.Workflow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]models.Workflow, 0, len(m.workflows))
	for _, w := range m.workflows {
		out = append(out, w)
	}
	return out, nil
}

func (m *MemoryStore) DeleteWorkflow(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.workflows[id]; !ok {
		return errors.New("not found")
	}
	delete(m.workflows, id)
	return nil
}

// ── Instances ──

func (m *MemoryStore) SaveInstance(inst models.Instance) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if inst.ID == "" {
		inst.ID = genID()
	}
	m.instances[inst.ID] = inst
	return inst.ID, nil
}

func (m *MemoryStore) GetInstance(id string) (models.Instance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inst, ok := m.instances[id]
	return inst, ok
}

func (m *MemoryStore) ListInstancesByWorkflow(workflowID string) ([]models.Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []models.Instance
	for _, inst := range m.instances {
		if inst.WorkflowID == workflowID {
			out = append(out, inst)
		}
	}
	return out, nil
}

// ── Tasks ──

func (m *MemoryStore) SaveTask(t models.TaskAssignment) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t.ID == "" {
		t.ID = genID()
	}
	m.tasks[t.ID] = t
	return t.ID, nil
}

func (m *MemoryStore) GetTask(id string) (models.TaskAssignment, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tasks[id]
	return t, ok
}

func (m *MemoryStore) ListTasksByRole(role string) ([]models.TaskAssignment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []models.TaskAssignment
	for _, t := range m.tasks {
		if t.AssignedRole == role && t.Status == models.TaskPending {
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *MemoryStore) ListTasksByInstance(instanceID string) ([]models.TaskAssignment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []models.TaskAssignment
	for _, t := range m.tasks {
		if t.InstanceID == instanceID {
			out = append(out, t)
		}
	}
	return out, nil
}
