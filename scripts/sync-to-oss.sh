#!/usr/bin/env bash
# =============================================================================
# sync-to-oss.sh — Sync knodex-ee (monorepo) → knodex (OSS public repo)
# =============================================================================
# Strips enterprise code, internal tooling, and build-tag dispatch files.
# Produces a clean OSS repository that builds and runs independently.
#
# Usage:
#   ./scripts/sync-to-oss.sh [OSS_DIR]
#
# Arguments:
#   OSS_DIR  Path to the OSS repo clone (default: ../knodex-oss)
#
# The script is idempotent — run it repeatedly to keep OSS in sync.
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
EE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
OSS_DIR="${1:-$(cd "$EE_DIR/.." && pwd)/knodex}"
# Resolve to absolute path (relative paths break after cd operations)
OSS_DIR="$(cd "$OSS_DIR" 2>/dev/null && pwd || echo "$OSS_DIR")"
OSS_REPO="knodex/knodex"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}→${NC} $*"; }
warn() { echo -e "${YELLOW}⚠${NC} $*"; }
err()  { echo -e "${RED}✗${NC} $*" >&2; }

# Portable sed -i (macOS requires '', GNU/Linux does not)
sedi() {
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "$@"
    else
        sed -i "$@"
    fi
}

# =============================================================================
# Step 1: Prepare OSS clone
# =============================================================================
echo ""
echo "=== Knodex EE → OSS Sync ==="
echo "  Source (EE): $EE_DIR"
echo "  Target (OSS): $OSS_DIR"
echo ""

if [ -d "$OSS_DIR/.git" ]; then
    log "Updating existing OSS clone..."
    cd "$OSS_DIR"
    git fetch origin 2>/dev/null || true
    # Reset to remote main if it exists, otherwise stay on current state
    if git rev-parse --verify origin/main >/dev/null 2>&1; then
        git checkout main 2>/dev/null || git checkout -b main
        git reset --hard origin/main
    fi
else
    log "Cloning OSS repo..."
    if gh repo view "$OSS_REPO" --json name >/dev/null 2>&1; then
        gh repo clone "$OSS_REPO" "$OSS_DIR" 2>/dev/null || {
            # Repo exists but is empty
            log "Empty repo — initializing locally..."
            mkdir -p "$OSS_DIR"
            cd "$OSS_DIR"
            git init
            git remote add origin "https://github.com/$OSS_REPO.git"
            git checkout -b main
        }
    else
        err "OSS repo $OSS_REPO does not exist. Create it first:"
        err "  gh repo create $OSS_REPO --private"
        exit 1
    fi
fi

cd "$OSS_DIR"

# =============================================================================
# Step 2: Rsync files from EE → OSS (with exclusions)
# =============================================================================
log "Syncing files..."

rsync -a --delete \
    --exclude='.git/' \
    --exclude='node_modules/' \
    --exclude='bin/' \
    --exclude='.playwright-mcp/' \
    --exclude='web/dist/' \
    --exclude='web/test-results/' \
    --exclude='web/playwright-report/' \
    \
    --exclude='server/ee/' \
    --exclude='server/ee_*.go' \
    \
    --exclude='web/test/e2e/compliance_*' \
    --exclude='web/test/e2e/views_*' \
    --exclude='web/test/e2e/settings/audit-trail*' \
    --exclude='web/test/fixture/enterprise-helpers.ts' \
    --exclude='server/test/e2e/compliance_*' \
    --exclude='server/test/e2e/audit_trail_*' \
    --exclude='server/test/e2e/authorization_audit*' \
    --exclude='server/test/e2e/license_*' \
    \
    --exclude='_bmad/' \
    --exclude='_bmad-output/' \
    --exclude='.claude/' \
    --exclude='custom-agents/' \
    --exclude='snyk/' \
    \
    --exclude='CLAUDE.md' \
    --exclude='.mcp.json' \
    --exclude='.claude_settings.json' \
    --exclude='.env' \
    --exclude='.env.qa' \
    \
    --exclude='docs/stories/' \
    \
    --exclude='.github/workflows/claude.yml' \
    --exclude='.github/workflows/claude-review.yml' \
    --exclude='.github/workflows/snyk.yaml' \
    \
    --exclude='.github/workflows/ci.yml' \
    --exclude='.github/workflows/e2e-tests.yml' \
    --exclude='.github/workflows/docker.yaml' \
    --exclude='.github/workflows/release-please.yml' \
    --exclude='release-please-config.json' \
    --exclude='.release-please-manifest.json' \
    --exclude='CHANGELOG.md' \
    \
    --exclude='deploy/charts/knodex/charts/' \
    --exclude='deploy/charts/knodex/Chart.lock' \
    --exclude='deploy/examples/gatekeeper/' \
    --exclude='deploy/examples/rgds/microservices-platform.yaml' \
    --exclude='deploy/examples/test-users.yaml' \
    --exclude='deploy/test/' \
    --exclude='deploy/server/views-configmap.yaml' \
    --exclude='deploy/server/license-secret.yaml' \
    --exclude='deploy/server/gatekeeper-rbac.yaml' \
    \
    "$EE_DIR/" "$OSS_DIR/"

# =============================================================================
# Step 3: Remove directories that rsync --exclude won't delete
# =============================================================================
log "Removing excluded directories from previous syncs..."

rm -rf "$OSS_DIR/server/ee"
rm -rf "$OSS_DIR/_bmad"
rm -rf "$OSS_DIR/_bmad-output"
rm -rf "$OSS_DIR/.claude"
rm -rf "$OSS_DIR/custom-agents"
rm -rf "$OSS_DIR/snyk"
rm -rf "$OSS_DIR/docs/stories"
rm -rf "$OSS_DIR/.playwright-mcp"
rm -f  "$OSS_DIR/CLAUDE.md"
rm -f  "$OSS_DIR/.mcp.json"
rm -f  "$OSS_DIR/.claude_settings.json"

# Helm chart build artifacts
rm -rf "$OSS_DIR/deploy/charts/knodex/charts"
rm -f  "$OSS_DIR/deploy/charts/knodex/Chart.lock"

# Enterprise E2E tests
rm -f "$OSS_DIR"/web/test/e2e/compliance_*
rm -f "$OSS_DIR"/web/test/e2e/views_*
rm -f "$OSS_DIR/web/test/e2e/settings/audit-trail.spec.ts"
rm -f "$OSS_DIR/web/test/fixture/enterprise-helpers.ts"
# Strip enterprise-helpers re-export from fixture index
if [ -f "$OSS_DIR/web/test/fixture/index.ts" ]; then
    sedi '/enterprise-helpers/d' "$OSS_DIR/web/test/fixture/index.ts"
fi
rm -f "$OSS_DIR"/server/test/e2e/compliance_*
rm -f "$OSS_DIR"/server/test/e2e/audit_trail_*
rm -f "$OSS_DIR"/server/test/e2e/authorization_audit*
rm -f "$OSS_DIR"/server/test/e2e/license_*

# Enterprise-only manifests
rm -rf "$OSS_DIR/deploy/examples/gatekeeper"
rm -f  "$OSS_DIR/deploy/examples/rgds/microservices-platform.yaml"
rm -f  "$OSS_DIR/deploy/examples/test-users.yaml"
rm -rf "$OSS_DIR/deploy/test"
rm -f  "$OSS_DIR/deploy/server/views-configmap.yaml"
rm -f  "$OSS_DIR/deploy/server/license-secret.yaml"
rm -f  "$OSS_DIR/deploy/server/gatekeeper-rbac.yaml"

# Patch server kustomization to remove enterprise-only resources
if [ -f "$OSS_DIR/deploy/server/kustomization.yaml" ]; then
    sedi \
        -e '/gatekeeper-rbac.yaml/d' \
        -e '/views-configmap.yaml/d' \
        -e '/license-secret.yaml/d' \
        "$OSS_DIR/deploy/server/kustomization.yaml"
fi

# =============================================================================
# Step 4: Remove EE dispatch files and strip build tags from OSS dispatch files
# =============================================================================
log "Processing build-tag dispatch files..."

# Delete ee_*.go files (enterprise implementations)
find "$OSS_DIR/server" -maxdepth 1 -name 'ee_*.go' -delete -print 2>/dev/null || true

# Strip //go:build !enterprise tags from oss_*.go files (keep the code, remove the guard)
for f in "$OSS_DIR/server"/oss_*.go; do
    [ -f "$f" ] || continue
    echo "  stripping build tag from $(basename "$f")"
    sedi '1{/^\/\/go:build !enterprise$/d;}' "$f"
    # Also remove the blank line that typically follows the build tag
    sedi '1{/^$/d;}' "$f"
done

# =============================================================================
# Step 5: Remove enterprise-only workflows that rsync may have copied
# =============================================================================
log "Removing enterprise-only workflows..."

rm -f "$OSS_DIR/.github/workflows/claude.yml"
rm -f "$OSS_DIR/.github/workflows/claude-review.yml"
rm -f "$OSS_DIR/.github/workflows/snyk.yaml"
rm -f "$OSS_DIR/.github/workflows/burn-in.yml"
rm -f "$OSS_DIR/.github/workflows/sync-oss.yml"
rm -f "$OSS_DIR/.github/workflows/integration-tests.yml"

# =============================================================================
# Step 6: Remove any files with enterprise build tags
# =============================================================================
log "Checking for stray enterprise build tags..."

# Only match files where //go:build enterprise is an actual build constraint
# (first line of the file), not a mention in a comment or doc string.
STRAY=""
while IFS= read -r f; do
    FIRST_LINE=$(head -1 "$f" 2>/dev/null)
    if [ "$FIRST_LINE" = "//go:build enterprise" ]; then
        STRAY="$STRAY $f"
    fi
done < <(grep -rli '//go:build enterprise' "$OSS_DIR/server/" 2>/dev/null || true)

if [ -n "$STRAY" ]; then
    warn "Found files with enterprise build tags (removing):"
    for f in $STRAY; do
        echo "  - ${f#$OSS_DIR/}"
        rm -f "$f"
    done
fi

# =============================================================================
# Step 7: Clean internal references from documentation
# =============================================================================
log "Cleaning internal references from docs..."

# Remove Claude/Anthropic sections from README
if [ -f "$OSS_DIR/README.md" ]; then
    # Remove lines referencing Anthropic, Claude Code, BMAD from README
    sedi \
        -e '/ANTHROPIC_API_KEY/d' \
        -e '/Claude Code/d' \
        -e '/Anthropic Console/d' \
        -e '/anthropic\.com/d' \
        -e '/Claude Code Action/d' \
        -e '/claude mention/d' \
        "$OSS_DIR/README.md"
fi

# Remove BMAD references from architecture docs
if [ -f "$OSS_DIR/docs/architecture/SOLUTION_ARCHITECTURE.md" ]; then
    sedi \
        -e '/bmad/d' \
        "$OSS_DIR/docs/architecture/SOLUTION_ARCHITECTURE.md"
fi

# =============================================================================
# Step 7b: Fix OSS-specific values overwritten by rsync
# =============================================================================
log "Applying OSS-specific fixups..."

# README badges: point to knodex/knodex (not knodex/knodex-ee)
if [ -f "$OSS_DIR/README.md" ]; then
    sedi \
        -e 's|knodex/knodex-ee/actions|knodex/knodex/actions|g' \
        -e 's|knodex/knodex-ee)|knodex/knodex)|g' \
        "$OSS_DIR/README.md"
fi

# Helm chart: server.image default should be OSS (not enterprise)
# Only fix server.image.repository (under "server:" block), NOT enterprise.image.repository.
# Use awk to replace only the first occurrence.
if [ -f "$OSS_DIR/deploy/charts/knodex/values.yaml" ]; then
    awk '{
        if (!done && /repository: ghcr.io\/knodex\/knodex-ee/) {
            sub(/repository: ghcr.io\/knodex\/knodex-ee/, "repository: ghcr.io/knodex/knodex")
            done=1
        }
        print
    }' "$OSS_DIR/deploy/charts/knodex/values.yaml" > "$OSS_DIR/deploy/charts/knodex/values.yaml.tmp" \
        && mv "$OSS_DIR/deploy/charts/knodex/values.yaml.tmp" "$OSS_DIR/deploy/charts/knodex/values.yaml"
fi

# =============================================================================
# Step 8: Create placeholder web dist for go build (if missing)
# =============================================================================
if [ ! -d "$OSS_DIR/server/internal/static/dist" ]; then
    log "Creating placeholder dist for embed..."
    mkdir -p "$OSS_DIR/server/internal/static/dist"
    cat > "$OSS_DIR/server/internal/static/dist/index.html" << 'HTML'
<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>Knodex</title></head>
<body><div id="root"></div></body>
</html>
HTML
    echo -n > "$OSS_DIR/server/internal/static/dist/.gitkeep"
fi

# =============================================================================
# Step 9: Clean Go dependencies
# =============================================================================
log "Running go mod tidy..."

cd "$OSS_DIR/server"
go mod tidy 2>&1 || warn "go mod tidy had issues (may need manual fix)"

# =============================================================================
# Step 10: Verify build
# =============================================================================
log "Verifying OSS build..."

cd "$OSS_DIR/server"
if go build ./... 2>&1; then
    echo -e "  ${GREEN}✓${NC} go build ./... passed"
else
    err "OSS build failed!"
    exit 1
fi

# =============================================================================
# Step 11: Verify no enterprise code remains
# =============================================================================
log "Verifying no enterprise code..."

if [ -d "$OSS_DIR/server/ee" ]; then
    err "server/ee/ still exists!"
    exit 1
fi

EE_FILES=$(find "$OSS_DIR/server" -maxdepth 1 -name 'ee_*.go' 2>/dev/null || true)
if [ -n "$EE_FILES" ]; then
    err "Enterprise dispatch files still exist: $EE_FILES"
    exit 1
fi

# Verify oss_*.go files had their build tags stripped
for f in "$OSS_DIR/server"/oss_*.go; do
    [ -f "$f" ] || continue
    FIRST_LINE=$(head -1 "$f" 2>/dev/null)
    if [ "$FIRST_LINE" = "//go:build !enterprise" ]; then
        err "Build tag not stripped from $(basename "$f")"
        exit 1
    fi
done

# Check for actual enterprise build constraints (first line), not comments
ENTERPRISE_TAGS=""
while IFS= read -r f; do
    FIRST_LINE=$(head -1 "$f" 2>/dev/null)
    if [ "$FIRST_LINE" = "//go:build enterprise" ]; then
        ENTERPRISE_TAGS="$ENTERPRISE_TAGS $f"
    fi
done < <(grep -rli '//go:build enterprise' "$OSS_DIR/server/" 2>/dev/null || true)

if [ -n "$ENTERPRISE_TAGS" ]; then
    err "Enterprise build tags found:$ENTERPRISE_TAGS"
    exit 1
fi

echo -e "  ${GREEN}✓${NC} No enterprise code found"

# =============================================================================
# Step 12: Generate AI commit message
# =============================================================================
cd "$OSS_DIR"

CHANGED=$(git status --porcelain | wc -l | tr -d ' ')

if [ "$CHANGED" -gt 0 ]; then
    git add -A

    COMMIT_MSG=""
    if [ -n "${ANTHROPIC_API_KEY:-}" ]; then
        log "Generating commit message with AI..."

        DIFF_STAT=$(git diff --cached --stat 2>/dev/null || true)
        DIFF_NAMES=$(git diff --cached --name-only 2>/dev/null || true)
        DIFF_PATCH=$(git diff --cached -U2 2>/dev/null | head -c 12000)

        PROMPT='You are writing a git commit message for an automated sync from the knodex-ee (enterprise) repo to the knodex (OSS) repo.

Rules:
- Use Conventional Commits format with type "sync"
- First line: max 72 chars, imperative mood, no period
- Then a blank line
- Then bullet points summarizing the key changes (max 8 bullets)
- Group related file changes into a single bullet
- Focus on WHAT changed and WHY it matters, not file paths
- Output ONLY the commit message, no markdown fences or explanation'

        PAYLOAD=$(jq -n \
            --arg model "claude-haiku-4-5-20251001" \
            --arg prompt "$PROMPT" \
            --arg stat "$DIFF_STAT" \
            --arg names "$DIFF_NAMES" \
            --arg patch "$DIFF_PATCH" \
            '{
              model: $model,
              max_tokens: 400,
              messages: [{
                role: "user",
                content: ($prompt + "\n\nFiles changed:\n" + $stat + "\n\nFile list:\n" + $names + "\n\nDiff preview:\n" + $patch)
              }]
            }')

        RESPONSE=$(curl -sf --max-time 30 https://api.anthropic.com/v1/messages \
            -H "x-api-key: $ANTHROPIC_API_KEY" \
            -H "anthropic-version: 2023-06-01" \
            -H "content-type: application/json" \
            -d "$PAYLOAD" 2>/dev/null) || true

        if [ -n "$RESPONSE" ]; then
            COMMIT_MSG=$(echo "$RESPONSE" | jq -r '.content[0].text // empty' 2>/dev/null) || true
        fi

        if [ -n "$COMMIT_MSG" ]; then
            echo -e "  ${GREEN}✓${NC} AI commit message generated"
        else
            warn "AI generation failed, using fallback message"
        fi
    fi

    # Fallback
    if [ -z "$COMMIT_MSG" ]; then
        COMMIT_MSG="sync: update from knodex-ee"
    fi
fi

# =============================================================================
# Summary
# =============================================================================
echo ""
echo "=== Sync Complete ==="
echo ""
git status --short | head -40
echo ""
if [ "$CHANGED" -eq 0 ]; then
    echo -e "${GREEN}✓ No changes — OSS repo is already in sync${NC}"
else
    echo -e "${YELLOW}→ $CHANGED files changed${NC}"
    echo ""
    echo -e "${GREEN}Commit message:${NC}"
    echo "─────────────────────────────────────"
    echo "$COMMIT_MSG"
    echo "─────────────────────────────────────"
    echo ""
    echo "Next steps:"
    echo "  cd $OSS_DIR"
    echo "  git commit -m '<message above>'"
    echo "  git push origin main"
fi
