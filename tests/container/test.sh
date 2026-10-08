#!/usr/bin/env bash
# Container test: the image carries a working binary, and a service image which copies it
# in turns healthy, or unhealthy where the probe asks the wrong port.
#
# Usage: tests/container/test.sh [IMAGE]   (default: container-healthcheck:test, built here)
set -euo pipefail

image="${1:-container-healthcheck:test}"
sample="container-healthcheck-sample:test"
root="$(cd "$(dirname "$0")/../.." && pwd)"
containers=()

cleanup() {
    if [ "${#containers[@]}" -gt 0 ]; then
        docker rm -f "${containers[@]}" >/dev/null 2>&1 || true
    fi
}
trap cleanup EXIT

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

# The verdict of the healthcheck, after its first decision
health_of() {
    local container="$1" status
    for _ in $(seq 1 60); do
        status="$(docker inspect --format '{{.State.Health.Status}}' "${container}")"
        if [ "${status}" = "healthy" ] || [ "${status}" = "unhealthy" ]; then
            echo "${status}"
            return
        fi
        sleep 1
    done
    echo "undecided"
}

if [ "$#" -eq 0 ]; then
    docker build --tag "${image}" "${root}"
fi

version="$(docker run --rm "${image}" --version)"
[ -n "${version}" ] || fail "the image prints no version"
echo "ok: the image runs, version ${version}"

docker build --tag "${sample}" --build-arg "HEALTHCHECK_IMAGE=${image}" \
    --file "${root}/tests/container/Dockerfile.sample" "${root}/tests/container"

healthy="$(docker run --detach "${sample}")"
containers+=("${healthy}")
[ "$(health_of "${healthy}")" = "healthy" ] || fail "the sample service is not healthy"
echo "ok: a service image with the binary turns healthy"

wrong_port="$(docker run --detach --env HEALTHCHECK_PORT=8081 "${sample}")"
containers+=("${wrong_port}")
[ "$(health_of "${wrong_port}")" = "unhealthy" ] || fail "a probe of the wrong port does not turn the service unhealthy"
docker inspect --format '{{range .State.Health.Log}}{{.Output}}{{end}}' "${wrong_port}" | grep -q "connection refused" \
    || fail "the health log does not name the reason"
echo "ok: a probe of the wrong port turns it unhealthy, with the reason in the health log"
