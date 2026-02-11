#!/bin/bash
# Feature: C043
# Verification script for C043 Quick Wins Code Cleanup

set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

echo "=== C043 Functional Test Verification ==="
echo ""

# Test 1: Documentation Status Filter Alignment
echo "[1/4] Testing documentation status filter alignment..."
COMMANDS_MD="docs/user-guide/commands.md"
if ! [ -f "$COMMANDS_MD" ]; then
    echo "  ✗ FAIL: $COMMANDS_MD not found"
    exit 1
fi

# Check for "cancelled" in status filter line
if grep -A 1 "Filter by status" "$COMMANDS_MD" | grep -q "cancelled"; then
    echo "  ✓ PASS: Status filter references 'cancelled'"
else
    echo "  ✗ FAIL: Status filter should reference 'cancelled'"
    exit 1
fi

# Test 2: No stale WARNING comments (C055 cleanup)
echo "[2/4] Verifying no stale WARNING comments about checkUnknownKeys..."
LOADER_TEST="internal/infrastructure/config/loader_test.go"
if ! [ -f "$LOADER_TEST" ]; then
    echo "  ✗ FAIL: $LOADER_TEST not found"
    exit 1
fi

WARNING_COUNT=$(grep -c "WARNING:.*checkUnknownKeys" "$LOADER_TEST" || true)

if [ "$WARNING_COUNT" -eq 0 ]; then
    echo "  ✓ PASS: No stale WARNING comments about checkUnknownKeys"
else
    echo "  ✗ FAIL: Found $WARNING_COUNT stale WARNING comments about checkUnknownKeys (should be 0 after C055)"
    exit 1
fi

# Test 3: Formatting Compliance
echo "[3/4] Testing gofmt compliance..."
GOFMT_OUTPUT=$(gofmt -d . 2>&1 || true)
if [ -z "$GOFMT_OUTPUT" ]; then
    echo "  ✓ PASS: All Go files pass gofmt (zero diff)"
else
    echo "  ✗ FAIL: gofmt found formatting issues:"
    echo "$GOFMT_OUTPUT" | head -20
    exit 1
fi

# Test 4: Required Files Exist
echo "[4/4] Testing required files exist..."
REQUIRED_FILES=(
    "docs/user-guide/commands.md"
    "internal/infrastructure/config/loader_test.go"
)

for file in "${REQUIRED_FILES[@]}"; do
    if ! [ -f "$file" ]; then
        echo "  ✗ FAIL: Required file missing: $file"
        exit 1
    fi
done
echo "  ✓ PASS: All required files exist"

echo ""
echo "=== All C043 Functional Tests PASSED ==="
