#!/usr/bin/env bash
set -euo pipefail

# Ensure a new version argument is provided
if [ "$#" -lt 1 ]; then
    echo "Usage: ${0} <new-version> [--execute]"
    exit 1
fi
NEW_VERSION="$1"
EXECUTE=0

# Exit if there are uncommitted changes
if ! git diff-index --quiet HEAD --; then
    echo "Error: There are uncommitted changes. Please commit or stash them before running this script."
    exit 1
fi

# Check if the --execute flag is passed
if [ "${2:-}" == "--execute" ]; then
    EXECUTE=1
else
    echo "Dry-run mode: No changes will be committed or tagged. Use '--execute' to apply changes."
fi

# Validate semantic version format
if ! [[ "$NEW_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Error: Version must be in semantic format (e.g., 1.2.3)"
    exit 1
fi

echo "Preparing to bump version to ${NEW_VERSION}..."
echo "Updating version in relevant files..."

# Update version in README.md using backreference
perl -pi -e 's|(https://raw.githubusercontent.com/psviderski/unregistry/v)[0-9]+\.[0-9]+\.[0-9]+(/docker-pussh)|${1}'"${NEW_VERSION}"'${2}|' README.md

# Update VERSION field in docker-pussh
perl -pi -e "s|^VERSION=\"[0-9]+\.[0-9]+\.[0-9]+\"|VERSION=\"${NEW_VERSION}\"|" docker-pussh

echo -e "Changes pending:\n---"
git diff
echo "---"

TAG_NAME="v$NEW_VERSION"
COMMIT_MESSAGE="release: Bump version to ${NEW_VERSION}"

echo "Building the project with goreleaser..."
goreleaser build --clean --snapshot
echo "Project built successfully."

if [ "$EXECUTE" = "1" ]; then
    echo "Executing changes..."
    git add -u
    git commit -m "${COMMIT_MESSAGE}"
    git tag "${TAG_NAME}"
    git push origin main "${TAG_NAME}"
    echo "Version bumped to ${NEW_VERSION} and git tag ${TAG_NAME} created."
    ## TODO: uncomment after some manual testing
    # goreleaser release --clean
else
    echo "Would create commit with message: '${COMMIT_MESSAGE}'"
    echo "Would create tag: ${TAG_NAME}"
    echo "Reverting back changes..."
    git checkout .
fi
