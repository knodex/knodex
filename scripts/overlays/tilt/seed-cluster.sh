#!/usr/bin/env bash
# Seed a cluster-provisioner instance using the current Kind cluster's kubeconfig.
# This simulates an operator provisioning a cluster and sharing its kubeconfig.
set -euo pipefail

CONTEXT="kind-knodex-qa"
NAMESPACE="eng-infra"
CLUSTER_NAME="dev-cluster"
SHARED_NS="eng-shared"

echo "==> Extracting Kind cluster kubeconfig..."
# Get the kubeconfig for the Kind cluster, rewriting the server to the in-cluster address
KUBECONFIG_RAW=$(kubectl config view --raw --minify --context "$CONTEXT" 2>/dev/null)
# Replace the external API server with the in-cluster address
KUBECONFIG_INCLUSTER=$(echo "$KUBECONFIG_RAW" | sed "s|server: .*|server: https://kubernetes.default.svc|")

echo "==> Applying RGDs to cluster..."
kubectl apply -f deploy/examples/rgds/cluster-provisioner.yaml --context "$CONTEXT"
kubectl apply -f deploy/examples/rgds/flux-app.yaml --context "$CONTEXT"

echo "==> Waiting for CRDs to be created by KRO..."
for i in $(seq 1 30); do
  if kubectl get crd clusterprovisioners.kro.run --context "$CONTEXT" >/dev/null 2>&1; then
    echo "    ClusterProvisioner CRD ready"
    break
  fi
  sleep 2
done

echo "==> Creating cluster-provisioner instance in $NAMESPACE..."
cat <<EOF | kubectl apply --context "$CONTEXT" -f -
apiVersion: kro.run/v1alpha1
kind: ClusterProvisioner
metadata:
  name: $CLUSTER_NAME
  namespace: $NAMESPACE
spec:
  clusterName: "$CLUSTER_NAME"
  sharedNamespace: "$SHARED_NS"
  kubeconfig: |
$(echo "$KUBECONFIG_INCLUSTER" | sed 's/^/    /')
EOF

echo "==> Waiting for kubeconfig Secret in $SHARED_NS..."
for i in $(seq 1 20); do
  if kubectl get secret "${CLUSTER_NAME}-kubeconfig" -n "$SHARED_NS" --context "$CONTEXT" >/dev/null 2>&1; then
    echo "    Secret ${CLUSTER_NAME}-kubeconfig created in $SHARED_NS"
    break
  fi
  sleep 2
done

echo ""
echo "==> Done! Cluster provisioned:"
echo "    Cluster:    $CLUSTER_NAME"
echo "    Metadata:   ConfigMap cluster-${CLUSTER_NAME}-meta in $NAMESPACE"
echo "    Kubeconfig: Secret ${CLUSTER_NAME}-kubeconfig in $SHARED_NS"
echo ""
echo "    Developer can now deploy a flux-app instance referencing:"
echo "      clusterName: $CLUSTER_NAME"
echo "      externalRef.kubeconfig.name: ${CLUSTER_NAME}-kubeconfig"
echo "      externalRef.kubeconfig.namespace: $SHARED_NS"
