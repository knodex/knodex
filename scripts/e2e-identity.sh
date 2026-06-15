#!/bin/bash
# Copyright 2026 Knodex Authors
# SPDX-License-Identifier: AGPL-3.0-only
#
# Cross-edition identity-persistence E2E runner (Story 15.5).
#
# Executes BOTH layers of the persistent-identity verification against a deployed
# cluster, per edition:
#
#   Layer 1 (live, NET-NEW): real mock-OIDC authorization-code login through the
#     deployed server → identity.users + federated_identities materialised →
#     Users API roundtrip → remove/reclaim/resurrect → entitlement count →
#     isInactive. Lives in server/test/e2e/identity_persistence_test.go
#     (//go:build e2e), run with -run TestIdentityE2E.
#
#   Layer 2 (reuse, run vs the DEPLOYED Postgres): the existing //go:build
#     integration suites that prove what a single-org HTTP API cannot —
#     cross-org RLS WITH CHECK isolation, verified/unverified email divergence,
#     email normalization, the audit-emit-failure metric, and the source_kind
#     round-trip. NO edits to those files: they are pointed at the cluster DB via
#     KNODEX_TEST_DATABASE_URL (port-forward).
#
# Usage:
#   ./scripts/e2e-identity.sh [oss|ee] [options]
#
# Options:
#   --no-setup     Assume the cluster + app are already deployed (skip deploy).
#   --no-cleanup   Leave port-forwards running for debugging.
#   --layer1-only  Run only the live Layer-1 e2e suite.
#   --layer2-only  Run only the Layer-2 integration suites.
#   --help, -h     Show this help.
#
# Editions:
#   oss    OSS deploy (go build ./...); source_kind=oidc_jit; NO audit.events.
#   ee     Enterprise deploy (ENTERPRISE_BUILD=true); source_kind=oidc_jit; audit on.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; BLUE='\033[0;34m'; CYAN='\033[0;36m'; NC='\033[0m'
log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_section() {
  echo ""
  echo -e "${BLUE}═══════════════════════════════════════════════════════════════════${NC}"
  echo -e "${CYAN}$1${NC}"
  echo -e "${BLUE}═══════════════════════════════════════════════════════════════════${NC}"
  echo ""
}

EDITION="oss"
SKIP_SETUP=false
SKIP_CLEANUP=false
RUN_LAYER1=true
RUN_LAYER2=true

NAMESPACE="${NAMESPACE:-knodex}"
DB_DSN="${KNODEX_TEST_DATABASE_URL:-postgres://knodex:knodex@localhost:5432/knodex?sslmode=disable}"

show_help() { sed -n '4,46p' "$0" | sed 's/^# \{0,1\}//'; }

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      oss|ee) EDITION="$1"; shift ;;
      --no-setup)   SKIP_SETUP=true; shift ;;
      --no-cleanup) SKIP_CLEANUP=true; shift ;;
      --layer1-only) RUN_LAYER2=false; shift ;;
      --layer2-only) RUN_LAYER1=false; shift ;;
      --help|-h)    show_help; exit 0 ;;
      *) log_error "Unknown option: $1"; show_help; exit 1 ;;
    esac
  done
}

PF_PIDS=()
cleanup() {
  if [ "$SKIP_CLEANUP" = true ]; then
    log_warn "Leaving port-forwards running (--no-cleanup): pids=${PF_PIDS[*]:-none}"
    return
  fi
  for pid in "${PF_PIDS[@]:-}"; do
    [ -n "$pid" ] && kill "$pid" 2>/dev/null || true
  done
  pkill -f "kubectl port-forward.*knodex" 2>/dev/null || true
  pkill -f "kubectl port-forward.*mock-oidc" 2>/dev/null || true
}
trap cleanup EXIT

wait_for_url() {
  local url="$1" attempts="${2:-60}" i=1
  while [ "$i" -le "$attempts" ]; do
    if curl -sf "$url" >/dev/null 2>&1; then return 0; fi
    sleep 2; i=$((i + 1))
  done
  return 1
}

deploy_edition() {
  if [ "$SKIP_SETUP" = true ]; then
    log_info "Skipping deploy (--no-setup)"; return 0
  fi
  log_section "Deploying ($EDITION)"
  case "$EDITION" in
    oss)
      ENTERPRISE_BUILD=false "$SCRIPT_DIR/qa-deploy.sh" deploy ;;
    ee)
      ENTERPRISE_BUILD=true "$SCRIPT_DIR/qa-deploy.sh" deploy ;;
  esac
}

start_port_forwards() {
  log_section "Starting port-forwards (server :8080, mock-oidc :8081, postgres :5432)"
  pkill -f "kubectl port-forward.*knodex-server" 2>/dev/null || true
  pkill -f "kubectl port-forward.*mock-oidc" 2>/dev/null || true
  pkill -f "kubectl port-forward.*knodex-postgres" 2>/dev/null || true
  sleep 1

  kubectl port-forward -n "$NAMESPACE" svc/knodex-server 8080:8080 >/tmp/pf-server.log 2>&1 &
  PF_PIDS+=("$!")
  kubectl port-forward -n "$NAMESPACE" svc/knodex-postgres 5432:5432 >/tmp/pf-postgres.log 2>&1 &
  PF_PIDS+=("$!")
  if [ "$RUN_LAYER1" = true ]; then
    kubectl port-forward -n "$NAMESPACE" svc/mock-oidc 8081:8081 >/tmp/pf-oidc.log 2>&1 &
    PF_PIDS+=("$!")
  fi
  sleep 3

  if ! wait_for_url "http://localhost:8080/healthz" 60; then
    log_error "Server did not become healthy on :8080"; exit 1
  fi
  log_info "Port-forwards ready"
}

run_layer1() {
  [ "$RUN_LAYER1" = true ] || { log_info "Skipping Layer 1 (--layer2-only)"; return 0; }
  log_section "Layer 1 — live wired-path E2E ($EDITION)"
  cd "$PROJECT_ROOT/server"
  E2E_TESTS=true \
  E2E_API_URL="http://localhost:8080" \
  E2E_OIDC_BASE_URL="http://localhost:8081" \
  E2E_OIDC_ISSUER="http://mock-oidc:8081" \
  E2E_OIDC_PROVIDER="mock-oidc" \
  E2E_EDITION="$EDITION" \
  E2E_INACTIVE_THRESHOLD_DAYS="${E2E_INACTIVE_THRESHOLD_DAYS:-30}" \
  KNODEX_TEST_DATABASE_URL="$DB_DSN" \
    go test -tags=e2e -v -count=1 -timeout 10m -run TestIdentityE2E ./test/e2e/
  cd "$PROJECT_ROOT"
}

run_layer2() {
  [ "$RUN_LAYER2" = true ] || { log_info "Skipping Layer 2 (--layer1-only)"; return 0; }
  log_section "Layer 2 — integration suites vs the deployed Postgres ($EDITION)"
  cd "$PROJECT_ROOT/server"

  log_info "Base identity store (RLS cross-org, email rules, resurrect, seat count)"
  KNODEX_TEST_DATABASE_URL="$DB_DSN" \
    go test -tags=integration -v -count=1 ./internal/identity/postgres/...

  if [ "$EDITION" = "ee" ]; then
    log_info "EE identity hooks (audit-emit-failure metric)"
    KNODEX_TEST_DATABASE_URL="$DB_DSN" \
      go test -tags='enterprise integration' -v -count=1 ./ee/identity/...
  fi
  cd "$PROJECT_ROOT"
}

main() {
  parse_args "$@"
  log_section "Identity E2E — edition=$EDITION layer1=$RUN_LAYER1 layer2=$RUN_LAYER2"
  deploy_edition
  start_port_forwards
  run_layer1
  run_layer2
  log_section "Identity E2E complete ($EDITION)"
}

main "$@"
