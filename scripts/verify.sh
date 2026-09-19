#!/bin/sh

set -eu

printf '%s\n' "Running project verification..."
go build ./...
go vet ./...

if [ -n "$(gofmt -l .)" ]; then
  printf '%s\n' "Verification failed: gofmt is required for:" >&2
  gofmt -l . >&2
  exit 1
fi

go test ./...
