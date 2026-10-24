#!/bin/sh
# Creates every missing local config from its committed .example, filling in
# the ${...} passwords from .env:
#
#   app/*/hack/config.yaml              GoFrame CLI (gf gen dao)
#   app/*/manifest/config/config.yaml   each service; keys still to be added
#
# Existing files are never touched. Usage: make dev-config
set -eu
cd "$(dirname "$0")/.."

if [ ! -f .env ]; then
	echo ".env is missing: copy .env.example to .env and choose the passwords" >&2
	exit 1
fi
set -a
. ./.env
set +a

for example in app/*/hack/config.yaml.example app/*/manifest/config/config.yaml.example; do
	target=${example%.example}
	if [ -e "$target" ]; then
		continue
	fi
	# perl rather than sed: a password may contain /, & or |.
	perl -pe 's/\$\{(MYSQL_PASSWORD|SEARCH_SYNC_PASSWORD|REDIS_PASSWORD|RABBITMQ_PASSWORD)\}/defined $ENV{$1} && $ENV{$1} ne "" ? $ENV{$1} : die "$1 is not set in .env\n"/ge' \
		"$example" > "$target.tmp"
	mv "$target.tmp" "$target"
	chmod 600 "$target"
	echo "created $target"
done
