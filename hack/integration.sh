#!/usr/bin/env bash
set -euo pipefail

floci_image="${FLOCI_IMAGE:-floci/floci:latest}"
floci_container_name="${FLOCI_CONTAINER_NAME:-telegram-jung2-bot-it-floci}"
floci_port="${FLOCI_PORT:-4566}"
floci_cpus="${FLOCI_CPUS:-4}"
floci_memory="${FLOCI_MEMORY:-2G}"

started_container=""

cleanup() {
  local status=$?

  if [[ -n "${started_container}" ]]; then
    container stop "${started_container}" >/dev/null 2>&1 || true
    container rm "${started_container}" >/dev/null 2>&1 || true
  fi

  exit "${status}"
}
trap cleanup EXIT

docker_available() {
  command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1
}

apple_container_available() {
  command -v container >/dev/null 2>&1
}

start_floci_via_apple_container() {
  echo "Docker unavailable; starting Floci via Apple container" >&2

  container rm -f "${floci_container_name}" >/dev/null 2>&1 || true
  container run -d --name "${floci_container_name}" \
    --cpus "${floci_cpus}" --memory "${floci_memory}" \
    -p "${floci_port}:4566" "${floci_image}" >/dev/null

  started_container="${floci_container_name}"

  local attempts=30
  until curl -sf "http://localhost:${floci_port}/_floci/init" >/dev/null 2>&1; do
    attempts=$((attempts - 1))
    if ((attempts <= 0)); then
      echo "Floci did not become ready on port ${floci_port}" >&2
      exit 1
    fi
    sleep 1
  done

  export FLOCI_ENDPOINT="http://localhost:${floci_port}"
}

if [[ -z "${FLOCI_ENDPOINT:-}" ]] && ! docker_available; then
  if apple_container_available; then
    start_floci_via_apple_container
  else
    echo "Neither Docker nor the Apple container CLI is available; cannot run Floci integration tests" >&2
    exit 1
  fi
fi

IFS=' ' read -ra modifiers <<< "${TEST_MODIFIERS:-}"
IFS=' ' read -ra targets <<< "${SLOW_TEST_TARGETS:-}"

buck2_args=(test "${modifiers[@]}" "${targets[@]}" -- --env INTEGRATION_TESTS=1)
if [[ -n "${FLOCI_ENDPOINT:-}" ]]; then
  buck2_args+=(--env "FLOCI_ENDPOINT=${FLOCI_ENDPOINT}")
fi

buck2 "${buck2_args[@]}"
