#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
project_name="alzette-slice0-$(date +%Y%m%d%H%M%S)-$$"
evidence_directory=$(mktemp -d /tmp/alzette-slice0-evidence.XXXXXX)

compose_command() {
	env \
		ALZETTE_HTTP_BIND_ADDRESS=127.0.0.1 \
		ALZETTE_GATEWAY_PORT=0 \
		ALZETTE_CONTROL_PORT=0 \
		POSTGRES_PORT=0 \
		OPENROUTER_API_KEY_SECRET_FILE=/dev/null \
		docker compose \
			--project-name "$project_name" \
			--env-file "$repository_root/.env.example" \
			--file "$repository_root/compose.yaml" \
			--file "$repository_root/compose.slice0.yaml" \
			--profile slice0 \
			"$@"
}

cleanup() {
	compose_command down --volumes --remove-orphans >/dev/null 2>&1 || true
	rm -rf -- "$evidence_directory"
}
trap cleanup EXIT HUP INT TERM

evidence_is_safe() {
	for canary in \
		'slice0-prompt-' \
		'slice0-cross-' \
		'SLICE0_COMPATIBLE_OK' \
		'slice0-fake-provider-token-not-a-secret' \
		'http://fake-target:8090/v1' \
		'alz_k_' \
		'Bearer '
	do
		if grep -F -q -- "$canary" "$@"; then
			return 1
		fi
	done
	return 0
}

compose_command config --quiet
compose_command up --detach --build --wait gateway fake-target

smoke_status=0
compose_command run --rm --no-deps slice0-smoke \
	>"$evidence_directory/smoke.json" \
	2>"$evidence_directory/smoke.stderr" || smoke_status=$?

log_status=0
compose_command logs --no-color gateway \
	>"$evidence_directory/gateway.log" 2>&1 || log_status=$?
compose_command logs --no-color fake-target \
	>"$evidence_directory/fake-target.log" 2>&1 || log_status=$?

if ! evidence_is_safe \
	"$evidence_directory/smoke.json" \
	"$evidence_directory/smoke.stderr" \
	"$evidence_directory/gateway.log" \
	"$evidence_directory/fake-target.log"
then
	echo "Slice 0 smoke evidence redaction check failed" >&2
	exit 1
fi
if [ "$smoke_status" -ne 0 ] || [ "$log_status" -ne 0 ]; then
	echo "Slice 0 smoke execution or log capture failed" >&2
	exit 1
fi

cat "$evidence_directory/smoke.json"
