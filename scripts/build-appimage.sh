#!/usr/bin/env bash
# Package an already-built static Linux binary. Run with appimagetool and a
# matching type2-runtime supplied explicitly so builds never fetch latest tools.
set -euo pipefail

if [[ $# -ne 4 ]]; then
	echo "Usage: $0 <linux-binary> <amd64|arm64> <version> <output-directory>" >&2
	echo "Set APPIMAGETOOL and APPIMAGE_RUNTIME to the packaging tool and target runtime." >&2
	exit 1
fi

binary=$1
goarch=$2
version=${3#v}
output_dir=$4
case "$goarch" in
amd64) arch=x86_64 ;;
arm64) arch=aarch64 ;;
*) echo "Unsupported AppImage architecture: $goarch" >&2; exit 1 ;;
esac

: "${APPIMAGETOOL:?Set APPIMAGETOOL to an appimagetool executable}"
: "${APPIMAGE_RUNTIME:?Set APPIMAGE_RUNTIME to the target AppImage runtime}"
[[ -f "$binary" ]] || { echo "Binary not found: $binary" >&2; exit 1; }
[[ -x "$APPIMAGETOOL" ]] || { echo "appimagetool is not executable: $APPIMAGETOOL" >&2; exit 1; }
[[ -f "$APPIMAGE_RUNTIME" ]] || { echo "Runtime not found: $APPIMAGE_RUNTIME" >&2; exit 1; }
[[ -n "$version" && "$version" != *$'\n'* ]] || { echo "Invalid version" >&2; exit 1; }
APPIMAGETOOL=$(cd -- "$(dirname -- "$APPIMAGETOOL")" && pwd)/$(basename -- "$APPIMAGETOOL")
APPIMAGE_RUNTIME=$(cd -- "$(dirname -- "$APPIMAGE_RUNTIME")" && pwd)/$(basename -- "$APPIMAGE_RUNTIME")
export APPIMAGE_RUNTIME

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
mkdir -p -- "$output_dir"
output_dir=$(cd -- "$output_dir" && pwd)
work_dir=$(mktemp -d)
trap 'rm -rf -- "$work_dir"' EXIT
appdir="$work_dir/taskii.AppDir"
mkdir -p "$appdir/usr/bin" "$appdir/usr/share/applications" "$appdir/usr/share/icons/hicolor/70x70/apps" "$appdir/usr/share/licenses/taskii"
install -m 755 "$binary" "$appdir/usr/bin/taskii"
install -m 755 "$repo_root/packaging/appimage/AppRun" "$appdir/AppRun"
install -m 644 "$repo_root/packaging/appimage/taskii.desktop" "$appdir/taskii.desktop"
install -m 644 "$repo_root/docs/images/logo.png" "$appdir/taskii.png"
install -m 644 "$repo_root/LICENSE" "$appdir/usr/share/licenses/taskii/LICENSE"
cp "$appdir/taskii.desktop" "$appdir/usr/share/applications/taskii.desktop"
cp "$appdir/taskii.png" "$appdir/usr/share/icons/hicolor/70x70/apps/taskii.png"
ln -s taskii.png "$appdir/.DirIcon"
if command -v desktop-file-validate >/dev/null 2>&1; then
	desktop-file-validate "$appdir/taskii.desktop"
fi

filename="taskii-linux-${goarch}.AppImage"
update_info="gh-releases-zsync|parsaenami|taskii|latest|${filename}.zsync"
# APPIMAGE_EXTRACT_AND_RUN makes appimagetool itself usable without FUSE.
# appimagetool writes zsync into its cwd even with an absolute destination.
# Package in a clean staging directory so a stale sidecar cannot mask failure.
(
	cd -- "$work_dir"
	ARCH="$arch" VERSION="$version" APPIMAGE_EXTRACT_AND_RUN=1 \
		"$APPIMAGETOOL" --no-appstream --runtime-file "$APPIMAGE_RUNTIME" \
		--updateinformation "$update_info" "$appdir" "$work_dir/$filename"
)
[[ -s "$work_dir/$filename" && -s "$work_dir/$filename.zsync" ]] || {
	echo "Packaging failed to produce both the AppImage and its zsync update file (install zsyncmake)." >&2
	exit 1
}
install -m 755 "$work_dir/$filename" "$output_dir/$filename"
install -m 644 "$work_dir/$filename.zsync" "$output_dir/$filename.zsync"
