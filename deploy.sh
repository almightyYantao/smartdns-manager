#!/bin/bash
# deploy.sh
VERSION_TYPE=${1:-patch}  # patch, minor, major

case $VERSION_TYPE in
  patch)
    git tag docker-patch
    ;;
  minor)
    git tag docker-minor
    ;;
  major)
    git tag docker-major
    ;;
esac

git push origin docker-$VERSION_TYPE