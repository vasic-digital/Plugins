#!/usr/bin/env bash
# plugins_functionality_challenge.sh - Validates Plugins module core functionality and structure
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MODULE_NAME="Plugins"

PASS=0
FAIL=0
TOTAL=0

pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  FAIL: $1"; }

echo "=== ${MODULE_NAME} Functionality Challenge ==="
echo ""

# Test 1: Required packages exist
echo "Test: Required packages exist"
pkgs_ok=true
for pkg in plugin registry loader sandbox structured; do
    if [ ! -d "${MODULE_DIR}/pkg/${pkg}" ]; then
        fail "Missing package: pkg/${pkg}"
        pkgs_ok=false
    fi
done
if [ "$pkgs_ok" = true ]; then
    pass "All required packages present (plugin, registry, loader, sandbox, structured)"
fi

# Test 2: Plugin interface is defined
echo "Test: Plugin interface is defined"
if grep -rq "type Plugin interface" "${MODULE_DIR}/pkg/plugin/"; then
    pass "Plugin interface is defined in pkg/plugin"
else
    fail "Plugin interface not found in pkg/plugin"
fi

# Test 3: Plugin metadata struct exists
echo "Test: Plugin Metadata struct exists"
if grep -rq "type Metadata struct" "${MODULE_DIR}/pkg/plugin/"; then
    pass "Metadata struct is defined in pkg/plugin"
else
    fail "Metadata struct not found in pkg/plugin"
fi

# Test 4: Plugin state tracking
echo "Test: Plugin state tracking exists"
if grep -rq "type State\|StateTracker\|State " "${MODULE_DIR}/pkg/plugin/"; then
    pass "Plugin state tracking found in pkg/plugin"
else
    fail "No plugin state tracking found"
fi

# Test 5: Registry implementation exists
echo "Test: Registry implementation exists"
if grep -rq "type Registry struct" "${MODULE_DIR}/pkg/registry/"; then
    pass "Registry struct is defined in pkg/registry"
else
    fail "Registry struct not found in pkg/registry"
fi

# Test 6: Loader interface is defined
echo "Test: Loader interface is defined"
if grep -rq "type Loader interface" "${MODULE_DIR}/pkg/loader/"; then
    pass "Loader interface is defined in pkg/loader"
else
    fail "Loader interface not found in pkg/loader"
fi

# Test 7: Sandbox interface is defined
echo "Test: Sandbox interface is defined"
if grep -rq "type Sandbox interface\|Sandbox" "${MODULE_DIR}/pkg/sandbox/"; then
    pass "Sandbox support found in pkg/sandbox"
else
    fail "No sandbox support found in pkg/sandbox"
fi

# Test 8: Structured output parsing support
echo "Test: Structured output parsing support exists"
if grep -rq "Parser\|Parse\|Structured\|Output" "${MODULE_DIR}/pkg/structured/"; then
    pass "Structured output parsing support found in pkg/structured"
else
    fail "No structured output parsing support found"
fi

# Test 9: Plugin lifecycle methods (Start/Stop/Init)
echo "Test: Plugin lifecycle methods exist"
lifecycle_found=0
for method in Start Stop Init Load Unload; do
    if grep -rq "${method}" "${MODULE_DIR}/pkg/plugin/"; then
        lifecycle_found=$((lifecycle_found + 1))
    fi
done
if [ "$lifecycle_found" -ge 2 ]; then
    pass "Plugin lifecycle methods found (${lifecycle_found} methods)"
else
    fail "Insufficient plugin lifecycle methods (found ${lifecycle_found})"
fi

# Test 10: Dynamic loading support
echo "Test: Dynamic loading support exists"
if grep -rq "SharedObject\|Process\|Dynamic\|Load" "${MODULE_DIR}/pkg/loader/"; then
    pass "Dynamic loading support found in pkg/loader"
else
    fail "No dynamic loading support found"
fi

echo ""
echo "=== Results: ${PASS}/${TOTAL} passed, ${FAIL} failed ==="
[ "${FAIL}" -eq 0 ] && exit 0 || exit 1
