#!/usr/bin/env bash
# Seed local users for Tilt development.
# Creates operator + developer accounts and assigns project roles.
set -euo pipefail

CONTEXT="kind-knodex-qa"
NAMESPACE="knodex-tilt"
API="http://localhost:8088"
PASSWORD="Operator1234!"  # Shared dev password, meets complexity reqs

echo "==> Creating local user accounts..."

# Add accounts to ConfigMap
kubectl --context "$CONTEXT" -n "$NAMESPACE" patch configmap knodex-accounts --type merge -p '{
  "data": {
    "accounts.operator": "apiKey, login",
    "accounts.operator.enabled": "true",
    "accounts.developer": "apiKey, login",
    "accounts.developer.enabled": "true"
  }
}'

# Set passwords in Secret (bcrypt hash of "Operator1234!")
HASH=$(htpasswd -nbBC 12 "" "$PASSWORD" | cut -d: -f2)
HASH_B64=$(echo -n "$HASH" | base64)
MTIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
MTIME_B64=$(echo -n "$MTIME" | base64)

kubectl --context "$CONTEXT" -n "$NAMESPACE" patch secret knodex-secret --type merge -p "{
  \"data\": {
    \"accounts.operator.password\": \"$HASH_B64\",
    \"accounts.operator.passwordMtime\": \"$MTIME_B64\",
    \"accounts.developer.password\": \"$HASH_B64\",
    \"accounts.developer.passwordMtime\": \"$MTIME_B64\"
  }
}"

# Wait for server to pick up the new accounts (cache TTL is 5s)
echo "==> Waiting for account cache refresh..."
sleep 6

# Get admin password and login
ADMIN_PW=$(kubectl --context "$CONTEXT" -n "$NAMESPACE" get secret knodex-initial-admin-password -o jsonpath='{.data.password}' | base64 -d)

echo "==> Logging in as admin..."
COOKIE_JAR=$(mktemp)
curl -sf -c "$COOKIE_JAR" "$API/api/v1/auth/local/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"admin\",\"password\":\"$ADMIN_PW\"}" > /dev/null

# Assign project roles (user IDs are "user-local-{username}")
echo "==> Assigning operator role..."
curl -sf -X POST "$API/api/v1/projects/engineering/roles/operator/users/user-local-operator" \
  -b "$COOKIE_JAR" \
  -H "Content-Type: application/json" || echo "  (may already exist)"

echo "==> Assigning developer role..."
curl -sf -X POST "$API/api/v1/projects/engineering/roles/developer/users/user-local-developer" \
  -b "$COOKIE_JAR" \
  -H "Content-Type: application/json" || echo "  (may already exist)"

rm -f "$COOKIE_JAR"

echo ""
echo "==> Done! Local users created:"
echo "    operator  / $PASSWORD  → sees infrastructure + observability RGDs"
echo "    developer / $PASSWORD  → sees applications + examples RGDs"
