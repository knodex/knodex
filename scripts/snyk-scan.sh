#!/bin/bash
# Local Snyk security scan
# Results are saved to snyk/ (gitignored)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
OUTPUT_DIR="$PROJECT_ROOT/snyk"
ORG_ID="b4ef8c2f-1ac1-47da-861d-b07577ce8dad"

mkdir -p "$OUTPUT_DIR"

echo "🔍 Running Snyk dependency scan..."
snyk test --org="$ORG_ID" --all-projects --json-file-output="$OUTPUT_DIR/dependencies.json" 2>&1 | tee "$OUTPUT_DIR/dependencies.txt" || true

echo ""
echo "✅ Scan complete. Results saved to:"
echo "   - $OUTPUT_DIR/dependencies.json"
echo "   - $OUTPUT_DIR/dependencies.txt"
