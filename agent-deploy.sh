#!/bin/bash
# scripts/release-agent.sh

VERSION_TYPE=${1:-patch}  # patch, minor, major

case $VERSION_TYPE in
  patch)
    git tag agent-patch
    echo "🚀 Triggering PATCH version release..."
    ;;
  minor)
    git tag agent-minor
    echo "🚀 Triggering MINOR version release..."
    ;;
  major)
    git tag agent-major
    echo "🚀 Triggering MAJOR version release..."
    ;;
  *)
    echo "❌ Invalid version type. Use: patch, minor, or major"
    echo "Usage: $0 [patch|minor|major]"
    exit 1
    ;;
esac

git push origin agent-$VERSION_TYPE
echo "✅ Release triggered! Check GitHub Actions for progress."