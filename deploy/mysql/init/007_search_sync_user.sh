#!/bin/sh
# A dedicated, least-privilege account for the search service's binlog
# reader (not root, as the original tutorial uses):
#
#   REPLICATION SLAVE, REPLICATION CLIENT  read the binlog stream and position
#   SELECT on goods.*                      load the current state of a product
#
# It cannot write anything. A shell script rather than .sql so that its
# password comes from the environment (SEARCH_SYNC_PASSWORD in .env, passed
# to the mysql container by compose) instead of being written here.
#
# Run by the image's entrypoint on first start, and by `make db-init`
# afterwards. Safe to re-run: ALTER USER keeps the password in step with .env.
set -eu

: "${SEARCH_SYNC_PASSWORD:?SEARCH_SYNC_PASSWORD is not set}"
password=$(printf '%s' "$SEARCH_SYNC_PASSWORD" | sed "s/'/''/g")

mysql -uroot -p"$MYSQL_ROOT_PASSWORD" <<SQL
CREATE USER IF NOT EXISTS 'search_sync'@'%' IDENTIFIED BY '$password';
ALTER USER 'search_sync'@'%' IDENTIFIED BY '$password';
GRANT REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO 'search_sync'@'%';
GRANT SELECT ON \`goods\`.* TO 'search_sync'@'%';
SQL
