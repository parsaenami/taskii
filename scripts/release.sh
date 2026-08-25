#!/usr/bin/env bash
# Tags and pushes a new release. Version lives only in git tags (injected at
# build time via -ldflags -X in .github/workflows/release.yml), so this
# script's whole job is: pick the next tag, confirm, tag, push.
set -euo pipefail

usage() {
	echo "Usage: $0 <major|minor|patch|vX.Y.Z>" >&2
	exit 1
}

[[ $# -eq 1 ]] || usage

if [[ -n "$(git status --porcelain)" ]]; then
	echo "Error: working tree is not clean. Commit or stash changes first." >&2
	exit 1
fi

branch=$(git rev-parse --abbrev-ref HEAD)
if [[ "$branch" != "main" ]]; then
	read -rp "You're on '$branch', not main. Continue anyway? [y/N] " reply
	[[ "$reply" =~ ^[Yy]$ ]] || exit 1
fi

latest=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
current=${latest#v}
IFS='.' read -r major minor patch <<<"$current"

arg=$1
case "$arg" in
v[0-9]*.[0-9]*.[0-9]*)
	next=$arg
	;;
major)
	next="v$((major + 1)).0.0"
	;;
minor)
	next="v${major}.$((minor + 1)).0"
	;;
patch)
	next="v${major}.${minor}.$((patch + 1))"
	;;
*)
	usage
	;;
esac

if git rev-parse "$next" >/dev/null 2>&1; then
	echo "Error: tag $next already exists." >&2
	exit 1
fi

echo "Latest tag: $latest"
echo "Next tag:   $next"
read -rp "Tag and push $next? [y/N] " reply
[[ "$reply" =~ ^[Yy]$ ]] || exit 1

git push origin "$branch"
git tag "$next"
git push origin "$next"

remote_url=$(git remote get-url origin)
repo=$(echo "$remote_url" | sed -E 's#(git@github.com:|https://github.com/)([^/]+/[^/.]+)(\.git)?#\2#')
echo "Tagged and pushed $next."
echo "Watch the release build: https://github.com/${repo}/actions"
