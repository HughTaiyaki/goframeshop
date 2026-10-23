#!/usr/bin/env sh

set -eu

required_go="1.24.5"
required_gf="2.10.2"

version_ge() {
  awk -v current="$1" -v required="$2" 'BEGIN {
    split(current, c, ".")
    split(required, r, ".")
    for (i = 1; i <= 4; i++) {
      if ((c[i] + 0) > (r[i] + 0)) exit 0
      if ((c[i] + 0) < (r[i] + 0)) exit 1
    }
    exit 0
  }'
}

failures=0

check_command() {
  if command -v "$1" >/dev/null 2>&1; then
    printf '%-22s %s\n' "$1" "$(command -v "$1")"
  else
    printf '%-22s %s\n' "$1" "MISSING"
    failures=$((failures + 1))
  fi
}

for command_name in go gf protoc protoc-gen-go protoc-gen-go-grpc docker; do
  check_command "$command_name"
done

if command -v go >/dev/null 2>&1; then
  go_version="$(go env GOVERSION | sed 's/^go//')"
  if ! version_ge "$go_version" "$required_go"; then
    printf 'Go %s is older than required %s\n' "$go_version" "$required_go"
    failures=$((failures + 1))
  fi
fi

if command -v gf >/dev/null 2>&1; then
  gf_version="$(gf version 2>/dev/null | head -n 1 | sed 's/^v//')"
  if ! version_ge "$gf_version" "$required_gf"; then
    printf 'GoFrame CLI %s is older than required %s\n' "$gf_version" "$required_gf"
    failures=$((failures + 1))
  fi
fi

if command -v docker >/dev/null 2>&1; then
  if ! docker compose version >/dev/null 2>&1; then
    printf '%-22s %s\n' "docker-compose" "MISSING"
    failures=$((failures + 1))
  fi

  if docker info >/dev/null 2>&1; then
    printf '%-22s %s\n' "docker-daemon" "READY"
  else
    printf '%-22s %s\n' "docker-daemon" "NOT RUNNING"
    failures=$((failures + 1))
  fi
fi

if [ "$failures" -ne 0 ]; then
  printf '\nEnvironment check failed with %s issue(s).\n' "$failures"
  exit 1
fi

printf '\nEnvironment check passed.\n'
