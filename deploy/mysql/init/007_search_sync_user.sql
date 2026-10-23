-- A dedicated, least-privilege account for the search service's binlog
-- reader (not root, as the original tutorial uses):
--
--   REPLICATION SLAVE, REPLICATION CLIENT  read the binlog stream and position
--   SELECT on goods.*                      load the current state of a product
--
-- It cannot write anything. The password is the local-development default,
-- like every other Compose credential; real deployments set their own.
-- Safe to re-run. Apply it with `make db-init`.

CREATE USER IF NOT EXISTS 'search_sync'@'%' IDENTIFIED BY 'change-me';
GRANT REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO 'search_sync'@'%';
GRANT SELECT ON `goods`.* TO 'search_sync'@'%';
