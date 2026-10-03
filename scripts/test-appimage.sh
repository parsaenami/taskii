#!/usr/bin/env bash
# Exercise the AppDir contract without downloading packaging tools in CI.
set -euo pipefail
repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
work_dir=$(mktemp -d)
trap 'rm -rf -- "$work_dir"' EXIT
mkdir -p "$work_dir/working directory" "$work_dir/output directory"
cat > "$work_dir/binary" <<'EOF'
#!/bin/sh
printf '%s\n' "$PWD" "$@"
EOF
cat > "$work_dir/packager" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
test "$1" = --no-appstream
test "$2" = --runtime-file
test "$3" = "$APPIMAGE_RUNTIME"
test "$4" = --updateinformation
test "$5" = "gh-releases-zsync|parsaenami|taskii|latest|taskii-linux-${TEST_GOARCH}.AppImage.zsync"
test "$ARCH" = "$TEST_ARCH"
test "$VERSION" = 1.2.3
test "$APPIMAGE_EXTRACT_AND_RUN" = 1
test -x "$6/AppRun"
test -x "$6/usr/bin/taskii"
test -s "$6/taskii.png"
test -s "$6/.DirIcon"
test -s "$6/usr/share/licenses/taskii/LICENSE"
cmp "$6/taskii.desktop" "$6/usr/share/applications/taskii.desktop"
cmp "$6/taskii.png" "$6/usr/share/icons/hicolor/70x70/apps/taskii.png"
grep -qx 'Terminal=true' "$6/taskii.desktop"
# A launched AppRun preserves cwd, whitespace, empty arguments and literal globs.
actual=$(cd "$TEST_CWD" && "$6/AppRun" --mock 'two words' '' '*')
expected=$(printf '%s\n' "$TEST_CWD" --mock 'two words' '' '*')
test "$actual" = "$expected"
printf 'AppImage\n' > "$7"
if [[ "${TEST_NO_ZSYNC:-0}" != 1 ]]; then
    # Match appimagetool: sidecars use the output basename in the current cwd.
    printf 'zsync\n' > "$(basename -- "$7").zsync"
fi
EOF
chmod +x "$work_dir/binary" "$work_dir/packager"
touch "$work_dir/runtime"
export APPIMAGETOOL="$work_dir/packager" APPIMAGE_RUNTIME="$work_dir/runtime" TEST_CWD="$work_dir/working directory"
for TEST_GOARCH in amd64 arm64; do
	export TEST_GOARCH
	case "$TEST_GOARCH" in
	amd64) export TEST_ARCH=x86_64 ;;
	arm64) export TEST_ARCH=aarch64 ;;
	esac
	bash "$repo_root/scripts/build-appimage.sh" "$work_dir/binary" "$TEST_GOARCH" v1.2.3 "$work_dir/output directory"
	test -x "$work_dir/output directory/taskii-linux-$TEST_GOARCH.AppImage"
	test -s "$work_dir/output directory/taskii-linux-$TEST_GOARCH.AppImage.zsync"
done
if bash "$repo_root/scripts/build-appimage.sh" "$work_dir/binary" unsupported v1.2.3 "$work_dir/output directory" >/dev/null 2>&1; then
	echo "Unsupported architecture was accepted" >&2
	exit 1
fi
export TEST_GOARCH=amd64 TEST_ARCH=x86_64 TEST_NO_ZSYNC=1
mkdir -p "$work_dir/missing-zsync"
printf 'stale image\n' > "$work_dir/missing-zsync/taskii-linux-amd64.AppImage"
printf 'stale zsync\n' > "$work_dir/missing-zsync/taskii-linux-amd64.AppImage.zsync"
if bash "$repo_root/scripts/build-appimage.sh" "$work_dir/binary" amd64 v1.2.3 "$work_dir/missing-zsync" >/dev/null 2>&1; then
	echo "Missing zsync update file was accepted" >&2
	exit 1
fi
# Tool/runtime paths may be relative to the caller, even after staging changes cwd.
export APPIMAGETOOL=./packager APPIMAGE_RUNTIME=./runtime TEST_NO_ZSYNC=0
(cd "$work_dir" && bash "$repo_root/scripts/build-appimage.sh" ./binary amd64 v1.2.3 ./relative-output)
test -s "$work_dir/relative-output/taskii-linux-amd64.AppImage.zsync"
echo "AppImage packaging tests passed"
