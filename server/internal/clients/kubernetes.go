package clients

import (
	"fmt"
	"log/slog"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/provops-org/knodex/server/internal/config"
)

const (
	// k8sRequestTimeout is the default timeout for Kubernetes API requests.
	// 10s prevents indefinite hangs while allowing for API server latency.
	// Increase for large clusters or cross-region API servers.
	k8sRequestTimeout = 10 * time.Second
)

// NewKubernetesClient creates a new Kubernetes client from configuration
// Returns nil if client cannot be created
func NewKubernetesClient(cfg *config.Kubernetes) kubernetes.Interface {
	var restConfig *rest.Config
	var err error

	if cfg.InCluster {
		// Use in-cluster configuration
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			slog.Error("failed to create in-cluster kubernetes config", "error", err)
			return nil
		}
		slog.Info("using in-cluster kubernetes configuration")
	} else {
		// Use kubeconfig file
		restConfig, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
		if err != nil {
			slog.Warn("failed to load kubeconfig, continuing without kubernetes client",
				"error", err,
				"kubeconfig", cfg.Kubeconfig)
			return nil
		}
		slog.Info("using kubeconfig", "path", cfg.Kubeconfig)
	}

	// Set timeout for requests to avoid hanging indefinitely
	restConfig.Timeout = k8sRequestTimeout

	// Increase QPS and Burst to avoid client-side throttling
	// Default is QPS=5, Burst=10 which causes throttling under moderate load
	restConfig.QPS = 50
	restConfig.Burst = 100

	// Create clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		slog.Error("failed to create kubernetes clientset", "error", err)
		return nil
	}

	// Test connection (non-blocking - log warning but still return client)
	// The watcher/tracker will handle connection errors gracefully
	version, err := clientset.Discovery().ServerVersion()
	if err != nil {
		slog.Warn("failed to connect to kubernetes API, client will retry on use", "error", err)
		// Return client anyway - connection will be retried when actually used
		return clientset
	}

	slog.Info("connected to kubernetes", "version", version.String())
	return clientset
}

// GetKubernetesConfig returns a *rest.Config for creating custom clients
// This is useful for creating CRD clients, dynamic clients, etc.
func GetKubernetesConfig(cfg *config.Kubernetes) (*rest.Config, error) {
	var restConfig *rest.Config
	var err error

	if cfg.InCluster {
		// Use in-cluster configuration
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create in-cluster kubernetes config: %w", err)
		}
	} else {
		// Use kubeconfig file
		restConfig, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	}

	// Set timeout for requests to avoid hanging indefinitely
	restConfig.Timeout = k8sRequestTimeout

	// Increase QPS and Burst to avoid client-side throttling
	// Default is QPS=5, Burst=10 which causes throttling under moderate load
	restConfig.QPS = 50
	restConfig.Burst = 100

	return restConfig, nil
}
