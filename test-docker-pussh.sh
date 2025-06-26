#!/usr/bin/env bash
set -eu

# Test script for docker-pussh --image-transfer-mode functionality

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKER_PUSSH="$SCRIPT_DIR/docker-pussh"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # no color

test_passed() {
    echo -e "${GREEN}✓${NC} $1"
}

test_failed() {
    echo -e "${RED}✗${NC} $1"
    exit 1
}

echo "Testing docker-pussh --image-transfer-mode functionality..."

# Test 1: Help should show the new option
if "$DOCKER_PUSSH" --help | grep -q "image-transfer-mode"; then
    test_passed "Help output includes image-transfer-mode option"
else
    test_failed "Help output missing image-transfer-mode option"
fi

# Test 2: Invalid mode should fail
if "$DOCKER_PUSSH" --image-transfer-mode invalid myimage:latest user@host 2>&1 | grep -q "must be either 'remote' or 'scp'"; then
    test_passed "Invalid image-transfer-mode correctly rejected"
else
    test_failed "Invalid image-transfer-mode not rejected"
fi

# Test 3: Valid modes should be accepted (we expect SSH connection to fail, but argument parsing should succeed)
if "$DOCKER_PUSSH" --image-transfer-mode remote myimage:latest user@host 2>&1 | grep -q "Connecting to user@host"; then
    test_passed "Remote mode argument parsing works"
else
    test_failed "Remote mode argument parsing failed"
fi

if "$DOCKER_PUSSH" --image-transfer-mode scp myimage:latest user@host 2>&1 | grep -q "Connecting to user@host"; then
    test_passed "SCP mode argument parsing works"
else
    test_failed "SCP mode argument parsing failed"
fi

# Test 4: Default mode should work (no --image-transfer-mode specified)
if "$DOCKER_PUSSH" myimage:latest user@host 2>&1 | grep -q "Connecting to user@host"; then
    test_passed "Default mode (remote) works"
else
    test_failed "Default mode (remote) failed"
fi

echo -e "${GREEN}All tests passed!${NC}" 