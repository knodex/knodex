// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/repository"
)

// RepositoryWatcher watches for repository credential Secrets created or deleted
// declaratively (via kubectl apply/delete) and emits structured logs and audit events.
// Secrets created via the Knodex API are deduplicated by checking the "knodex.io/created-by" annotation.
type RepositoryWatcher struct {
	k8sClient kubernetes.Interface
	namespace string
	recorder  audit.Recorder
	logger    *slog.Logger

	informer cache.Controller
	stopCh   chan struct{}
	done     chan struct{}
	synced   atomic.Bool
	running  atomic.Bool
	stopped  bool       // true after stopCh is closed, reset on Start; guarded by mu
	mu       sync.Mutex // guards Start/Stop lifecycle transitions
}

// NewRepositoryWatcher creates a new watcher for repository credential Secrets.
// namespace is the K8s namespace where credential secrets are stored.
// recorder may be nil (OSS builds) — audit events are skipped in that case.
func NewRepositoryWatcher(k8sClient kubernetes.Interface, namespace string, recorder audit.Recorder) *RepositoryWatcher {
	return &RepositoryWatcher{
		k8sClient: k8sClient,
		namespace: namespace,
		recorder:  recorder,
		stopCh:    make(chan struct{}),
		done:      make(chan struct{}),
		logger:    slog.Default().With("component", "repository-watcher"),
	}
}

// Start begins watching for repository secret events.
func (w *RepositoryWatcher) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running.Load() {
		w.logger.Warn("repository watcher already running")
		return nil
	}

	w.logger.Info("starting repository secret watcher", "namespace", w.namespace)

	// Reinitialize channels and state for fresh start (safe — mutex held, no goroutine running)
	w.stopCh = make(chan struct{})
	w.done = make(chan struct{})
	w.stopped = false
	w.synced.Store(false)

	// Create a ListWatch scoped to the credential namespace with label selector
	labelSelector := fmt.Sprintf("%s=%s", repository.LabelSecretType, repository.LabelSecretTypeVal)
	lw := cache.NewFilteredListWatchFromClient(
		w.k8sClient.CoreV1().RESTClient(),
		"secrets",
		w.namespace,
		func(options *metav1.ListOptions) {
			options.LabelSelector = labelSelector
		},
	)

	// Use cache.NewInformer (not SharedIndexInformer) — Secrets are core API objects,
	// and we need only a single handler pair with no indexing or shared access.
	_, w.informer = cache.NewInformer(lw, &corev1.Secret{}, 30*time.Second, cache.ResourceEventHandlerFuncs{
		AddFunc:    w.handleAdd,
		DeleteFunc: w.handleDelete,
	})

	// Mark running before launching goroutine to close TOCTOU window
	w.running.Store(true)

	// Start informer in background
	go func() {
		defer close(w.done)
		defer w.running.Store(false)
		w.informer.Run(w.stopCh)
		w.logger.Info("repository secret watcher stopped")
	}()

	// Wait for initial sync — use stopCh so Stop() also unblocks this goroutine,
	// avoiding a leak when the caller's context outlives the watcher.
	stopCh := w.stopCh
	go func() {
		if !cache.WaitForCacheSync(stopCh, w.informer.HasSynced) {
			w.logger.Error("failed to sync repository secret cache")
			return
		}
		w.synced.Store(true)
		w.logger.Info("repository secret cache synced", "namespace", w.namespace)
	}()

	return nil
}

// Stop stops the watcher without waiting for completion.
// Safe to call multiple times and concurrently with Start.
func (w *RepositoryWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stopped || !w.running.Load() {
		return
	}
	w.logger.Info("stopping repository secret watcher")
	w.stopped = true
	close(w.stopCh)
}

// StopAndWait stops the watcher and waits for the informer goroutine to exit.
// Returns true if stopped cleanly, false if timeout was reached.
func (w *RepositoryWatcher) StopAndWait(timeout time.Duration) bool {
	w.mu.Lock()
	if w.stopped || !w.running.Load() {
		w.mu.Unlock()
		return true
	}

	w.logger.Info("stopping repository secret watcher and waiting for completion")
	w.stopped = true
	close(w.stopCh)
	w.mu.Unlock() // release before blocking wait

	select {
	case <-w.done:
		w.logger.Info("repository secret watcher stopped cleanly")
		return true
	case <-time.After(timeout):
		w.logger.Warn("repository secret watcher stop timed out", "timeout", timeout)
		return false
	}
}

// IsSynced returns true if the initial cache sync is complete.
func (w *RepositoryWatcher) IsSynced() bool {
	return w.synced.Load()
}

// IsRunning returns true if the watcher is running.
func (w *RepositoryWatcher) IsRunning() bool {
	return w.running.Load()
}

// handleAdd processes a new repository secret event.
func (w *RepositoryWatcher) handleAdd(obj interface{}) {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		w.logger.Error("unexpected object type in repository add handler", "type", fmt.Sprintf("%T", obj))
		return
	}

	// Dedup: skip API-created secrets (already audited by the API handler).
	// Check key presence, not value — an empty-string annotation still means API-created.
	if _, ok := secret.Annotations[models.AnnotationCreatedBy]; ok {
		w.logger.Debug("skipping API-created repository secret", "secret_name", secret.Name)
		return
	}

	// Extract metadata from secret data
	repoURL := string(secret.Data[repository.SecretKeyURL])
	repoName := string(secret.Data[repository.SecretKeyRepoName])
	project := string(secret.Data[repository.SecretKeyProject])
	authType := string(secret.Data[repository.SecretKeyType])

	if repoURL == "" || repoName == "" {
		w.logger.Warn("declarative repository secret missing expected metadata",
			"secret_name", secret.Name,
			"has_url", repoURL != "",
			"has_name", repoName != "",
			"has_project", project != "",
		)
	}

	w.logger.Info("declarative repository secret detected",
		"action", "created",
		"secret_name", secret.Name,
		"repo_url", repoURL,
		"repo_name", repoName,
		"project", project,
		"auth_type", authType,
	)

	audit.RecordEvent(w.recorder, context.Background(), audit.Event{
		UserID:    "system/declarative",
		UserEmail: "system/declarative",
		Action:    "create",
		Resource:  "repositories",
		Name:      repoName,
		Project:   project,
		Result:    "success",
		Details: map[string]any{
			"repo_url":  repoURL,
			"auth_type": authType,
			"source":    "declarative",
		},
	})
}

// handleDelete processes a deleted repository secret event.
func (w *RepositoryWatcher) handleDelete(obj interface{}) {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		// Handle DeletedFinalStateUnknown tombstone
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			w.logger.Error("unexpected object type in repository delete handler", "type", fmt.Sprintf("%T", obj))
			return
		}
		secret, ok = tombstone.Obj.(*corev1.Secret)
		if !ok {
			w.logger.Error("unexpected tombstone object type in repository delete handler", "type", fmt.Sprintf("%T", tombstone.Obj))
			return
		}
	}

	// Dedup: skip API-managed secrets (API handler already audits deletion).
	// Check key presence, not value — an empty-string annotation still means API-managed.
	if _, ok := secret.Annotations[models.AnnotationCreatedBy]; ok {
		w.logger.Debug("skipping API-managed repository secret deletion", "secret_name", secret.Name)
		return
	}

	// Extract metadata from secret data
	repoURL := string(secret.Data[repository.SecretKeyURL])
	repoName := string(secret.Data[repository.SecretKeyRepoName])
	project := string(secret.Data[repository.SecretKeyProject])
	authType := string(secret.Data[repository.SecretKeyType])

	w.logger.Info("declarative repository secret removed",
		"action", "deleted",
		"secret_name", secret.Name,
		"repo_url", repoURL,
		"repo_name", repoName,
		"project", project,
		"auth_type", authType,
	)

	audit.RecordEvent(w.recorder, context.Background(), audit.Event{
		UserID:    "system/declarative",
		UserEmail: "system/declarative",
		Action:    "delete",
		Resource:  "repositories",
		Name:      repoName,
		Project:   project,
		Result:    "success",
		Details: map[string]any{
			"repo_url":  repoURL,
			"auth_type": authType,
			"source":    "declarative",
		},
	})
}
