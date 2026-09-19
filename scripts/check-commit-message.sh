#!/bin/sh

set -eu

commit_message_file=$1
commit_subject=$(sed -n '1p' "$commit_message_file")

case "$commit_subject" in
  Merge\ *|merge\ *)
    exit 0
    ;;
esac

if ! printf '%s\n' "$commit_subject" | grep -Eq '^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([[:alnum:]][[:alnum:]./_-]*\))?!?: [^[:space:]].*$'; then
  printf '%s\n' "Commit rejected: use Conventional Commits, e.g. feat(ui): add task filter" >&2
  exit 1
fi
