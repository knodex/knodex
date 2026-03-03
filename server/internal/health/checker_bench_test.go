package health

import (
	"context"
	"testing"
)

// mockWatcher implements RGDWatcherHealth for benchmarking
type benchMockWatcher struct {
	running bool
	synced  bool
}

func (m *benchMockWatcher) IsRunning() bool { return m.running }
func (m *benchMockWatcher) IsSynced() bool  { return m.synced }

// BenchmarkCheckLiveness benchmarks the liveness probe
func BenchmarkCheckLiveness(b *testing.B) {
	checker := NewChecker(nil, nil, nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = checker.CheckLiveness(ctx)
	}
}

// BenchmarkCheckReadiness_NoClients benchmarks readiness without clients
func BenchmarkCheckReadiness_NoClients(b *testing.B) {
	checker := NewChecker(nil, nil, nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = checker.CheckReadiness(ctx)
	}
}

// BenchmarkCheckReadiness_WatcherOnly benchmarks readiness with only watcher
func BenchmarkCheckReadiness_WatcherOnly(b *testing.B) {
	watcher := &benchMockWatcher{running: true, synced: true}
	checker := NewChecker(nil, nil, watcher)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = checker.CheckReadiness(ctx)
	}
}

// BenchmarkCheckRGDWatcher benchmarks watcher health check in isolation
func BenchmarkCheckRGDWatcher(b *testing.B) {
	watcher := &benchMockWatcher{running: true, synced: true}
	checker := NewChecker(nil, nil, watcher)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = checker.checkRGDWatcher()
	}
}
