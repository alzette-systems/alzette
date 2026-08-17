#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repository_root"

for source_path in \
	cmd/alzette/main.go \
	cmd/alzette/main_test.go \
	internal/secrets/secrets.go \
	internal/secrets/secrets_test.go
do
	if git check-ignore -q --no-index "$source_path"; then
		echo "gitignore verification failed: required Go source is ignored" >&2
		exit 1
	fi
done

ignored_go_sources=$(
	find . -type f -name '*.go' -print |
		sed 's#^\./##' |
		git check-ignore --no-index --stdin 2>/dev/null || true
)
if [ -n "$ignored_go_sources" ]; then
	echo "gitignore verification failed: at least one Go source is ignored" >&2
	exit 1
fi

if ! git check-ignore -q --no-index alzette; then
	echo "gitignore verification failed: root build artifact is not ignored" >&2
	exit 1
fi

for secret_path in .secrets/provider-token.txt secrets/provider-token.txt; do
	if ! git check-ignore -q --no-index "$secret_path"; then
		echo "gitignore verification failed: root secret material is not ignored" >&2
		exit 1
	fi
done

echo "gitignore verification passed"
