#!/bin/sh
# Installs the latest taskii release binary for your OS/arch.
# Usage: curl -fsSL https://raw.githubusercontent.com/parsaenami/taskii/main/install.sh | sh
set -e

repo="parsaenami/taskii"
install_dir="${TASKII_INSTALL_DIR:-/usr/local/bin}"

os=$(uname -s)
arch=$(uname -m)

case "$os" in
	Linux) os="linux" ;;
	Darwin) os="darwin" ;;
	*)
		echo "error: unsupported OS: $os (download a release manually from https://github.com/$repo/releases)" >&2
		exit 1
		;;
esac

case "$arch" in
	x86_64 | amd64) arch="amd64" ;;
	arm64 | aarch64) arch="arm64" ;;
	*)
		echo "error: unsupported architecture: $arch" >&2
		exit 1
		;;
esac

archive="taskii-${os}-${arch}.tar.gz"
url="https://github.com/${repo}/releases/latest/download/${archive}"

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

echo "Downloading $url"
curl -fsSL "$url" -o "$tmp_dir/$archive"
tar -xzf "$tmp_dir/$archive" -C "$tmp_dir"

binary="$tmp_dir/taskii-${os}-${arch}"
chmod +x "$binary"

if [ -w "$install_dir" ]; then
	mv "$binary" "$install_dir/taskii"
else
	echo "Elevated permissions required to write to $install_dir"
	sudo mv "$binary" "$install_dir/taskii"
fi

echo "Installed taskii to $install_dir/taskii"
taskii_path=$(command -v taskii || echo "$install_dir/taskii")
"$taskii_path" --help >/dev/null 2>&1 && echo "Run 'taskii' to get started."
