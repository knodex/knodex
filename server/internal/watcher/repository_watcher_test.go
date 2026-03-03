package watcher

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/provops-org/knodex/server/internal/audit"
	"github.com/provops-org/knodex/server/internal/models"
	"github.com/provops-org/knodex/server/internal/repository"
)

// mockRecorder captures audit events for test assertions.
type mockRecorder struct {
	mu     sync.Mutex
	events []audit.Event
}

func (m *mockRecorder) Record(_ context.Context, event audit.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
}

func (m *mockRecorder) Events() []audit.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]audit.Event, len(m.events))
	copy(cp, m.events)
	return cp
}

// createTestRepoSecret builds a corev1.Secret that looks like a repository credential.
func createTestRepoSecret(name string, annotations map[string]string, data map[string]string) *corev1.Secret {
	secretData := make(map[string][]byte, len(data))
	for k, v := range data {
		secretData[k] = []byte(v)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels: map[string]string{
				repository.LabelSecretType: repository.LabelSecretTypeVal,
			},
			Annotations: annotations,
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
	}
}

// defaultRepoData returns typical repository secret data fields.
func defaultRepoData() map[string]string {
	return map[string]string{
		repository.SecretKeyURL:      "https://github.com/org/repo.git",
		repository.SecretKeyRepoName: "my-repo",
		repository.SecretKeyProject:  "alpha",
		repository.SecretKeyType:     "https",
	}
}

// captureLogs returns a buffer and slog.Logger that write to that buffer.
func captureLogs() (*bytes.Buffer, *slog.Logger) {
	buf := &bytes.Buffer{}
	h := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return buf, slog.New(h)
}

func TestNewRepositoryWatcher(t *testing.T) {
	client := fake.NewSimpleClientset()
	rec := &mockRecorder{}

	w := NewRepositoryWatcher(client, "test-ns", rec)

	if w == nil {
		t.Fatal("expected watcher to be created")
	}
	if w.namespace != "test-ns" {
		t.Errorf("expected namespace 'test-ns', got %q", w.namespace)
	}
	if w.IsRunning() {
		t.Error("expected watcher not to be running initially")
	}
	if w.IsSynced() {
		t.Error("expected watcher not to be synced initially")
	}
}

func TestNewRepositoryWatcher_NilRecorder(t *testing.T) {
	client := fake.NewSimpleClientset()

	w := NewRepositoryWatcher(client, "default", nil)
	if w == nil {
		t.Fatal("expected watcher to be created with nil recorder")
	}
}

func TestRepositoryWatcher_HandleAdd_Declarative(t *testing.T) {
	client := fake.NewSimpleClientset()
	rec := &mockRecorder{}
	logBuf, logger := captureLogs()

	w := NewRepositoryWatcher(client, "default", rec)
	w.logger = logger

	secret := createTestRepoSecret("repo-decl-1", nil, defaultRepoData())
	w.handleAdd(secret)

	// Verify slog output
	logOutput := logBuf.String()
	if !bytes.Contains([]byte(logOutput), []byte("declarative repository secret detected")) {
		t.Errorf("expected log to contain 'declarative repository secret detected', got:\n%s", logOutput)
	}

	// Verify audit event
	events := rec.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	ev := events[0]
	if ev.UserID != "system/declarative" {
		t.Errorf("expected UserID 'system/declarative', got %q", ev.UserID)
	}
	if ev.Action != "create" {
		t.Errorf("expected Action 'create', got %q", ev.Action)
	}
	if ev.Resource != "repositories" {
		t.Errorf("expected Resource 'repositories', got %q", ev.Resource)
	}
	if ev.Name != "my-repo" {
		t.Errorf("expected Name 'my-repo', got %q", ev.Name)
	}
	if ev.Project != "alpha" {
		t.Errorf("expected Project 'alpha', got %q", ev.Project)
	}
	if ev.Result != "success" {
		t.Errorf("expected Result 'success', got %q", ev.Result)
	}
	if ev.Details["source"] != "declarative" {
		t.Errorf("expected Details[source]='declarative', got %v", ev.Details["source"])
	}
	if ev.Details["repo_url"] != "https://github.com/org/repo.git" {
		t.Errorf("expected Details[repo_url], got %v", ev.Details["repo_url"])
	}
}

func TestRepositoryWatcher_HandleAdd_APICreated_Skipped(t *testing.T) {
	client := fake.NewSimpleClientset()
	rec := &mockRecorder{}
	logBuf, logger := captureLogs()

	w := NewRepositoryWatcher(client, "default", rec)
	w.logger = logger

	annotations := map[string]string{
		models.AnnotationCreatedBy: "admin@example.com",
	}
	secret := createTestRepoSecret("repo-api-1", annotations, defaultRepoData())
	w.handleAdd(secret)

	// Verify no INFO-level log (only debug)
	logOutput := logBuf.String()
	if bytes.Contains([]byte(logOutput), []byte("declarative repository secret detected")) {
		t.Errorf("expected no 'declarative repository secret detected' log for API-created secret, got:\n%s", logOutput)
	}
	if !bytes.Contains([]byte(logOutput), []byte("skipping API-created repository secret")) {
		t.Errorf("expected debug log 'skipping API-created repository secret', got:\n%s", logOutput)
	}

	// Verify no audit event
	if len(rec.Events()) != 0 {
		t.Errorf("expected 0 audit events for API-created secret, got %d", len(rec.Events()))
	}
}

func TestRepositoryWatcher_HandleAdd_NilRecorder(t *testing.T) {
	client := fake.NewSimpleClientset()

	w := NewRepositoryWatcher(client, "default", nil)
	logBuf, logger := captureLogs()
	w.logger = logger

	secret := createTestRepoSecret("repo-decl-nil", nil, defaultRepoData())

	// Should not panic with nil recorder
	w.handleAdd(secret)

	logOutput := logBuf.String()
	if !bytes.Contains([]byte(logOutput), []byte("declarative repository secret detected")) {
		t.Errorf("expected slog output even with nil recorder, got:\n%s", logOutput)
	}
}

func TestRepositoryWatcher_HandleDelete_Declarative(t *testing.T) {
	client := fake.NewSimpleClientset()
	rec := &mockRecorder{}
	logBuf, logger := captureLogs()

	w := NewRepositoryWatcher(client, "default", rec)
	w.logger = logger

	secret := createTestRepoSecret("repo-decl-del", nil, defaultRepoData())
	w.handleDelete(secret)

	// Verify slog output
	logOutput := logBuf.String()
	if !bytes.Contains([]byte(logOutput), []byte("declarative repository secret removed")) {
		t.Errorf("expected log 'declarative repository secret removed', got:\n%s", logOutput)
	}

	// Verify audit event
	events := rec.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	ev := events[0]
	if ev.Action != "delete" {
		t.Errorf("expected Action 'delete', got %q", ev.Action)
	}
	if ev.Details["source"] != "declarative" {
		t.Errorf("expected Details[source]='declarative', got %v", ev.Details["source"])
	}
}

func TestRepositoryWatcher_HandleDelete_APICreated_Skipped(t *testing.T) {
	client := fake.NewSimpleClientset()
	rec := &mockRecorder{}
	logBuf, logger := captureLogs()

	w := NewRepositoryWatcher(client, "default", rec)
	w.logger = logger

	annotations := map[string]string{
		models.AnnotationCreatedBy: "admin@example.com",
	}
	secret := createTestRepoSecret("repo-api-del", annotations, defaultRepoData())
	w.handleDelete(secret)

	logOutput := logBuf.String()
	if bytes.Contains([]byte(logOutput), []byte("declarative repository secret removed")) {
		t.Errorf("expected no declarative log for API-created secret deletion")
	}
	if len(rec.Events()) != 0 {
		t.Errorf("expected 0 audit events, got %d", len(rec.Events()))
	}
}

func TestRepositoryWatcher_HandleDelete_Tombstone(t *testing.T) {
	client := fake.NewSimpleClientset()
	rec := &mockRecorder{}
	logBuf, logger := captureLogs()

	w := NewRepositoryWatcher(client, "default", rec)
	w.logger = logger

	secret := createTestRepoSecret("repo-tombstone", nil, defaultRepoData())

	// Wrap in DeletedFinalStateUnknown tombstone
	tombstone := cache.DeletedFinalStateUnknown{
		Key: "default/repo-tombstone",
		Obj: secret,
	}
	w.handleDelete(tombstone)

	logOutput := logBuf.String()
	if !bytes.Contains([]byte(logOutput), []byte("declarative repository secret removed")) {
		t.Errorf("expected tombstone to be handled, got:\n%s", logOutput)
	}

	events := rec.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event from tombstone, got %d", len(events))
	}
}

func TestRepositoryWatcher_HandleDelete_InvalidTombstone(t *testing.T) {
	client := fake.NewSimpleClientset()
	rec := &mockRecorder{}
	logBuf, logger := captureLogs()

	w := NewRepositoryWatcher(client, "default", rec)
	w.logger = logger

	// Invalid tombstone wrapping a non-Secret object
	tombstone := cache.DeletedFinalStateUnknown{
		Key: "default/invalid",
		Obj: "not-a-secret",
	}
	w.handleDelete(tombstone)

	logOutput := logBuf.String()
	if !bytes.Contains([]byte(logOutput), []byte("unexpected tombstone object type")) {
		t.Errorf("expected error log for invalid tombstone, got:\n%s", logOutput)
	}
	if len(rec.Events()) != 0 {
		t.Errorf("expected 0 audit events for invalid tombstone, got %d", len(rec.Events()))
	}
}

func TestRepositoryWatcher_HandleAdd_InvalidType(t *testing.T) {
	client := fake.NewSimpleClientset()
	rec := &mockRecorder{}
	logBuf, logger := captureLogs()

	w := NewRepositoryWatcher(client, "default", rec)
	w.logger = logger

	w.handleAdd("not-a-secret")

	logOutput := logBuf.String()
	if !bytes.Contains([]byte(logOutput), []byte("unexpected object type in repository add handler")) {
		t.Errorf("expected error log for invalid type, got:\n%s", logOutput)
	}
}

func TestRepositoryWatcher_HandleDelete_InvalidType(t *testing.T) {
	client := fake.NewSimpleClientset()
	rec := &mockRecorder{}
	logBuf, logger := captureLogs()

	w := NewRepositoryWatcher(client, "default", rec)
	w.logger = logger

	w.handleDelete(42) // neither *Secret nor tombstone

	logOutput := logBuf.String()
	if !bytes.Contains([]byte(logOutput), []byte("unexpected object type in repository delete handler")) {
		t.Errorf("expected error log for invalid type, got:\n%s", logOutput)
	}
}

func TestRepositoryWatcher_StopIdempotent(t *testing.T) {
	client := fake.NewSimpleClientset()
	w := NewRepositoryWatcher(client, "default", nil)

	// Stop without starting — should not panic
	w.Stop()
	w.Stop()
}

func TestRepositoryWatcher_StopAndWaitIdempotent(t *testing.T) {
	client := fake.NewSimpleClientset()
	w := NewRepositoryWatcher(client, "default", nil)

	result := w.StopAndWait(time.Second)
	if !result {
		t.Error("expected StopAndWait to return true when not running")
	}

	result = w.StopAndWait(time.Second)
	if !result {
		t.Error("expected StopAndWait to return true on second call")
	}
}

// fakeController implements cache.Controller for lifecycle testing without a real K8s API.
type fakeController struct {
	runCh  chan struct{} // closed when Run is called
	stopCh <-chan struct{}
	synced bool
}

func newFakeController() *fakeController {
	return &fakeController{runCh: make(chan struct{})}
}

func (f *fakeController) Run(stopCh <-chan struct{}) {
	f.stopCh = stopCh
	close(f.runCh)
	<-stopCh // block until stopped
}

func (f *fakeController) RunWithContext(ctx context.Context) {
	close(f.runCh)
	<-ctx.Done() // block until context is cancelled
}

func (f *fakeController) HasSynced() bool                 { return f.synced }
func (f *fakeController) LastSyncResourceVersion() string { return "" }

func TestRepositoryWatcher_StartStopLifecycle(t *testing.T) {
	client := fake.NewSimpleClientset()
	rec := &mockRecorder{}

	w := NewRepositoryWatcher(client, "default", rec)

	if w.IsRunning() {
		t.Fatal("expected not running before Start")
	}

	// Inject a fake controller to avoid needing a real REST client
	fc := newFakeController()
	fc.synced = true
	w.informer = fc
	w.stopCh = make(chan struct{})
	w.done = make(chan struct{})

	// Simulate Start's goroutine manually (since Start() calls NewInformer which needs a real client)
	w.running.Store(true)
	go func() {
		defer close(w.done)
		defer w.running.Store(false)
		w.informer.Run(w.stopCh)
	}()

	// Wait for the controller to be running
	<-fc.runCh

	if !w.IsRunning() {
		t.Fatal("expected running after start")
	}

	// Stop and wait
	ok := w.StopAndWait(5 * time.Second)
	if !ok {
		t.Fatal("expected StopAndWait to return true")
	}

	if w.IsRunning() {
		t.Fatal("expected not running after StopAndWait")
	}

	// Double stop should not panic
	w.Stop()
	ok = w.StopAndWait(time.Second)
	if !ok {
		t.Fatal("expected second StopAndWait to return true")
	}
}

func TestRepositoryWatcher_ConcurrentStopSafe(t *testing.T) {
	client := fake.NewSimpleClientset()
	w := NewRepositoryWatcher(client, "default", nil)

	// Inject a fake controller
	fc := newFakeController()
	fc.synced = true
	w.informer = fc
	w.stopCh = make(chan struct{})
	w.done = make(chan struct{})
	w.running.Store(true)

	go func() {
		defer close(w.done)
		defer w.running.Store(false)
		w.informer.Run(w.stopCh)
	}()

	<-fc.runCh

	// Call Stop concurrently from multiple goroutines — should not panic
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.Stop()
		}()
	}
	wg.Wait()

	// Wait for clean shutdown
	<-w.done
	if w.IsRunning() {
		t.Fatal("expected not running after concurrent stops")
	}
}

func TestRepositoryWatcher_HandleAdd_EmptyData(t *testing.T) {
	client := fake.NewSimpleClientset()
	rec := &mockRecorder{}

	w := NewRepositoryWatcher(client, "default", rec)
	logBuf, logger := captureLogs()
	w.logger = logger

	// Secret with no data fields
	secret := createTestRepoSecret("repo-empty", nil, map[string]string{})
	w.handleAdd(secret)

	logOutput := logBuf.String()
	if !bytes.Contains([]byte(logOutput), []byte("declarative repository secret detected")) {
		t.Errorf("expected log even with empty data, got:\n%s", logOutput)
	}
	if !bytes.Contains([]byte(logOutput), []byte("declarative repository secret missing expected metadata")) {
		t.Errorf("expected warning about missing metadata, got:\n%s", logOutput)
	}

	events := rec.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	// Fields should be empty strings, not panics
	if events[0].Name != "" {
		t.Errorf("expected empty Name for secret with no data, got %q", events[0].Name)
	}
}

func TestRepositoryWatcher_HandleDelete_EmptyData(t *testing.T) {
	client := fake.NewSimpleClientset()
	rec := &mockRecorder{}

	w := NewRepositoryWatcher(client, "default", rec)
	logBuf, logger := captureLogs()
	w.logger = logger

	// Secret with no data fields
	secret := createTestRepoSecret("repo-del-empty", nil, map[string]string{})
	w.handleDelete(secret)

	logOutput := logBuf.String()
	if !bytes.Contains([]byte(logOutput), []byte("declarative repository secret removed")) {
		t.Errorf("expected delete log even with empty data, got:\n%s", logOutput)
	}

	events := rec.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	// Fields should be empty strings, not panics
	if events[0].Name != "" {
		t.Errorf("expected empty Name for secret with no data, got %q", events[0].Name)
	}
	if events[0].Action != "delete" {
		t.Errorf("expected Action 'delete', got %q", events[0].Action)
	}
}

func TestRepositoryWatcher_HandleAdd_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		annotations   map[string]string
		expectLog     string
		expectAudit   bool
		expectSkipLog string
	}{
		{
			name:        "declarative (no annotation)",
			annotations: nil,
			expectLog:   "declarative repository secret detected",
			expectAudit: true,
		},
		{
			name:        "declarative (empty annotations map)",
			annotations: map[string]string{},
			expectLog:   "declarative repository secret detected",
			expectAudit: true,
		},
		{
			name: "API-created (has created-by)",
			annotations: map[string]string{
				models.AnnotationCreatedBy: "user@example.com",
			},
			expectSkipLog: "skipping API-created repository secret",
			expectAudit:   false,
		},
		{
			name: "API-created (empty created-by value)",
			annotations: map[string]string{
				models.AnnotationCreatedBy: "",
			},
			expectSkipLog: "skipping API-created repository secret",
			expectAudit:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			rec := &mockRecorder{}
			logBuf, logger := captureLogs()

			w := NewRepositoryWatcher(client, "default", rec)
			w.logger = logger

			secret := createTestRepoSecret("test-"+tt.name, tt.annotations, defaultRepoData())
			w.handleAdd(secret)

			logOutput := logBuf.String()
			if tt.expectLog != "" && !bytes.Contains([]byte(logOutput), []byte(tt.expectLog)) {
				t.Errorf("expected log %q, got:\n%s", tt.expectLog, logOutput)
			}
			if tt.expectSkipLog != "" && !bytes.Contains([]byte(logOutput), []byte(tt.expectSkipLog)) {
				t.Errorf("expected skip log %q, got:\n%s", tt.expectSkipLog, logOutput)
			}

			events := rec.Events()
			if tt.expectAudit && len(events) == 0 {
				t.Error("expected audit event but got none")
			}
			if !tt.expectAudit && len(events) > 0 {
				t.Errorf("expected no audit event, got %d", len(events))
			}
		})
	}
}

func TestRepositoryWatcher_HandleDelete_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		annotations   map[string]string
		expectLog     string
		expectAudit   bool
		expectSkipLog string
	}{
		{
			name:        "declarative (no annotation)",
			annotations: nil,
			expectLog:   "declarative repository secret removed",
			expectAudit: true,
		},
		{
			name: "API-managed (has created-by)",
			annotations: map[string]string{
				models.AnnotationCreatedBy: "admin@example.com",
			},
			expectSkipLog: "skipping API-managed repository secret deletion",
			expectAudit:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			rec := &mockRecorder{}
			logBuf, logger := captureLogs()

			w := NewRepositoryWatcher(client, "default", rec)
			w.logger = logger

			secret := createTestRepoSecret("del-"+tt.name, tt.annotations, defaultRepoData())
			w.handleDelete(secret)

			logOutput := logBuf.String()
			if tt.expectLog != "" && !bytes.Contains([]byte(logOutput), []byte(tt.expectLog)) {
				t.Errorf("expected log %q, got:\n%s", tt.expectLog, logOutput)
			}
			if tt.expectSkipLog != "" && !bytes.Contains([]byte(logOutput), []byte(tt.expectSkipLog)) {
				t.Errorf("expected skip log %q, got:\n%s", tt.expectSkipLog, logOutput)
			}

			events := rec.Events()
			if tt.expectAudit && len(events) == 0 {
				t.Error("expected audit event but got none")
			}
			if !tt.expectAudit && len(events) > 0 {
				t.Errorf("expected no audit event, got %d", len(events))
			}
		})
	}
}
