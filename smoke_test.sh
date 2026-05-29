#!/bin/bash
# Smoke test a carbon-relay-ng image:
# - start the container
# - confirm each listener logs that it is up
# - send a metric and confirm it reaches the routing table
set -euo pipefail

image="${1:?usage: smoke_test.sh <image>}"
container="crng-smoke-$$"
line_port=12003  # host port mapped to the relay's 2003 line listener
http_port=18081  # host port mapped to the relay's 8081 admin server

cleanup() { docker rm -f "$container" >/dev/null 2>&1 || true; }
trap cleanup EXIT

die() {
    echo "smoke test FAILED for $image: $1" >&2
    docker logs "$container" 2>&1 || true
    exit 1
}

# Retry a command quietly until it succeeds, giving up after ~30s.
retry() {
    for _ in $(seq 30); do
        "$@" >/dev/null 2>&1 && return 0
        sleep 1
    done
    return 1
}

docker run -d --name "$container" -e CRNG_LOG_LEVEL=info \
    -p "127.0.0.1:${line_port}:2003" -p "127.0.0.1:${http_port}:8081" \
    "$image" >/dev/null

# 1. Each listener announces itself in the logs.
logged() { docker logs "$container" 2>&1 | grep -qF "$1"; }
for listener in \
    "listening on 0.0.0.0:2003/tcp" \
    "listening on 0.0.0.0:2003/udp" \
    "admin TCP listener starting on 0.0.0.0:2004" \
    "admin HTTP listener starting on 0.0.0.0:8081"; do
    retry logged "$listener" || die "listener never came up: $listener"
done

# 2. A metric sent over the line protocol increments the "direction=in" counter in the routing table
in_count() {
    curl -fsS "http://127.0.0.1:${http_port}/debug/vars2" \
        | grep -oE '"[^"]*\.direction_is_in"[: ]+[0-9]+' | grep -oE '[0-9]+$'
}
printf 'carbon.relay.ng.smoke.test 1 %s\n' "$(date +%s)" \
    > "/dev/tcp/127.0.0.1/${line_port}" || die "could not reach the line listener"
ingested() { [ "$(in_count)" -gt 0 ]; }
retry ingested || die "metric was sent but never counted"

echo "--- container logs ---"
docker logs "$container" 2>&1 || true
echo "smoke test passed for $image"
