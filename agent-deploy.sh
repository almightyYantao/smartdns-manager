#!/bin/bash
# scripts/release-agent.sh 或 agent-deploy.sh

VERSION_TYPE=${1:-patch}  # patch, minor, major

echo "🔍 Preparing to release agent with $VERSION_TYPE version bump..."

# 验证版本类型
case $VERSION_TYPE in
  patch|minor|major)
    ;;
  *)
    echo "❌ Invalid version type: $VERSION_TYPE"
    echo "Usage: $0 [patch|minor|major]"
    exit 1
    ;;
esac

TRIGGER_TAG="agent-$VERSION_TYPE"

# 检查是否有未提交的更改
if ! git diff-index --quiet HEAD --; then
    echo "⚠️  You have uncommitted changes. Please commit or stash them first."
    echo "Uncommitted files:"
    git status --porcelain
    read -p "Continue anyway? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# 确保我们在最新的代码上
echo "📡 Fetching latest changes..."
git fetch origin

# 删除本地和远程的触发标签（如果存在）
echo "🧹 Cleaning up existing trigger tags..."

# 删除本地标签
if git tag -l | grep -q "^$TRIGGER_TAG$"; then
    echo "   Deleting local tag: $TRIGGER_TAG"
    git tag -d $TRIGGER_TAG
fi

# 删除远程标签
if git ls-remote --tags origin | grep -q "refs/tags/$TRIGGER_TAG$"; then
    echo "   Deleting remote tag: $TRIGGER_TAG"
    git push origin :refs/tags/$TRIGGER_TAG
fi

# 等待一下确保远程标签删除完成
sleep 1

# 创建新的触发标签
echo "🏷️  Creating trigger tag: $TRIGGER_TAG"
git tag $TRIGGER_TAG

# 推送触发标签
echo "🚀 Pushing trigger tag to start the release process..."
if git push origin $TRIGGER_TAG; then
    echo "✅ Release triggered successfully!"
    echo ""
    echo "📋 What happens next:"
    echo "   1. GitHub Actions will detect the $TRIGGER_TAG tag"
    echo "   2. Generate new version number automatically"
    echo "   3. Build binaries for all platforms"
    echo "   4. Create GitHub release with packages"
    echo ""
    echo "🔗 Monitor progress at:"
    echo "   https://github.com/$(git remote get-url origin | sed 's/.*github.com[:/]\([^/]*\/[^/]*\).*/\1/' | sed 's/\.git$//')/actions"
else
    echo "❌ Failed to push trigger tag"
    exit 1
fi