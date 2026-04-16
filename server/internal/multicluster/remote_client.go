// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package multicluster

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/knodex/knodex/server/internal/capi"
)

// BuildRemoteDynamicClient reads the CAPI kubeconfig secret for clusterName
// and returns a dynamic.Interface for that remote cluster.
// Uses capi.KubeconfigSecretName to derive the secret name and reads the
// kubeconfig data from key "value" (CAPI convention).
func BuildRemoteDynamicClient(ctx context.Context, k8sClient kubernetes.Interface, clusterName string) (dynamic.Interface, error) {
	restConfig, err := restConfigFromCAPI(ctx, k8sClient, clusterName)
	if err != nil {
		return nil, err
	}

	client, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create dynamic client from kubeconfig for %q: %w", clusterName, err)
	}

	return client, nil
}

// restConfigFromCAPI reads the CAPI kubeconfig secret and returns a rest.Config.
func restConfigFromCAPI(ctx context.Context, k8sClient kubernetes.Interface, clusterName string) (*rest.Config, error) {
	secretName := capi.KubeconfigSecretName(clusterName)

	// Search all namespaces for the kubeconfig secret (CAPI creates it in the cluster's namespace)
	secrets, err := k8sClient.CoreV1().Secrets("").List(ctx, metav1.ListOptions{
		FieldSelector: "metadata.name=" + secretName,
	})
	if err != nil {
		return nil, fmt.Errorf("list secrets for %q: %w", secretName, err)
	}
	if len(secrets.Items) == 0 {
		return nil, fmt.Errorf("kubeconfig secret %q not found in any namespace", secretName)
	}

	// Use the first matching secret (CAPI creates one per cluster)
	secret := secrets.Items[0]

	// CAPI convention: kubeconfig data is at key "value"
	kubeconfigData, ok := secret.Data["value"]
	if !ok {
		return nil, fmt.Errorf("kubeconfig secret %q/%q missing 'value' key", secret.Namespace, secret.Name)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigData)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig from secret %q/%q: %w", secret.Namespace, secret.Name, err)
	}

	return restConfig, nil
}

// buildRemoteClientFromCAPI reads a CAPI kubeconfig secret and builds a K8s client for the remote cluster.
// Shared by NamespaceProvisioner and NamespaceReaper.
func buildRemoteClientFromCAPI(ctx context.Context, k8sClient kubernetes.Interface, clusterName string) (kubernetes.Interface, error) {
	restConfig, err := restConfigFromCAPI(ctx, k8sClient, clusterName)
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create client from kubeconfig for %q: %w", clusterName, err)
	}

	return client, nil
}
