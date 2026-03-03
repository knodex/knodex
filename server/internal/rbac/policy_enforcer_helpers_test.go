package rbac

import (
	"context"
	"sync"
	"testing"
)

// mockProjectReader implements ProjectReader for testing
type mockProjectReader struct {
	projects map[string]*Project
	mu       sync.RWMutex
	getErr   error
	listErr  error
}

func newMockProjectReader() *mockProjectReader {
	return &mockProjectReader{
		projects: make(map[string]*Project),
	}
}

func (m *mockProjectReader) AddProject(project *Project) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.projects[project.Name] = project
}

func (m *mockProjectReader) RemoveProject(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.projects, name)
}

func (m *mockProjectReader) GetProject(ctx context.Context, name string) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getErr != nil {
		return nil, m.getErr
	}

	project, ok := m.projects[name]
	if !ok {
		return nil, ErrProjectNotFound
	}
	return project, nil
}

func (m *mockProjectReader) ListProjects(ctx context.Context) ([]Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.listErr != nil {
		return nil, m.listErr
	}

	result := make([]Project, 0, len(m.projects))
	for _, p := range m.projects {
		result = append(result, *p)
	}
	return result, nil
}

func (m *mockProjectReader) ProjectExists(ctx context.Context, name string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getErr != nil {
		return false, m.getErr
	}

	_, ok := m.projects[name]
	return ok, nil
}

func (m *mockProjectReader) FindProjectForNamespace(ctx context.Context, namespace string) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check each project's destinations to find one that allows this namespace
	for _, project := range m.projects {
		for _, dest := range project.Spec.Destinations {
			// Check for exact match or wildcard pattern match
			if dest.Namespace == namespace || dest.Namespace == "*" {
				return project, nil
			}
			// Check for prefix wildcard match (e.g., "beta*" matches "beta-team")
			if MatchGlob(dest.Namespace, namespace) {
				return project, nil
			}
		}
	}

	return nil, ErrProjectNotFound
}

// mockEmptyProjectReader is a mock that returns empty project list
type mockEmptyProjectReader struct{}

func (m *mockEmptyProjectReader) GetProject(ctx context.Context, name string) (*Project, error) {
	return nil, ErrProjectNotFound
}

func (m *mockEmptyProjectReader) ListProjects(ctx context.Context) ([]Project, error) {
	return []Project{}, nil
}

func (m *mockEmptyProjectReader) ProjectExists(ctx context.Context, name string) (bool, error) {
	return false, nil
}

func (m *mockEmptyProjectReader) FindProjectForNamespace(ctx context.Context, namespace string) (*Project, error) {
	return nil, ErrProjectNotFound
}

// newTestEnforcer creates a PolicyEnforcer with a nil ProjectReader for testing.
func newTestEnforcer(t *testing.T) PolicyEnforcer {
	t.Helper()
	enforcer, err := NewCasbinEnforcer()
	if err != nil {
		t.Fatal("failed to create Casbin enforcer:", err)
	}
	return NewPolicyEnforcer(enforcer, nil)
}

// newTestEnforcerWithMock creates a PolicyEnforcer with a mockProjectReader for testing.
func newTestEnforcerWithMock(t *testing.T) (PolicyEnforcer, *mockProjectReader) {
	t.Helper()
	enforcer, err := NewCasbinEnforcer()
	if err != nil {
		t.Fatal("failed to create Casbin enforcer:", err)
	}
	reader := newMockProjectReader()
	return NewPolicyEnforcer(enforcer, reader), reader
}
