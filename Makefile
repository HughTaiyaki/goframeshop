include hack/hack-cli.mk

.PHONY: env-check
env-check:
	@./scripts/check-env.sh

.PHONY: infra-up
infra-up:
	@docker compose up -d mysql etcd redis elasticsearch rabbitmq otel-collector jaeger

.PHONY: infra-down
infra-down:
	@docker compose down

# Applies every init script to a running MySQL. The scripts are idempotent, so
# this is how an existing volume picks up a newly added schema without the data
# loss of infra-reset.
.PHONY: db-init
db-init:
	@for f in deploy/mysql/init/*.sql; do \
		echo "applying $$f"; \
		docker compose exec -T mysql sh -c 'mysql -uroot -p"$$MYSQL_ROOT_PASSWORD"' < "$$f" || exit 1; \
	done

.PHONY: infra-reset
infra-reset:
	@docker compose down -v

.PHONY: infra-ps
infra-ps:
	@docker compose ps

.PHONY: infra-logs
infra-logs:
	@docker compose logs -f mysql etcd redis elasticsearch rabbitmq otel-collector jaeger

.PHONY: verify
verify:
	@go mod verify
	@go test ./...

# Prints a new Ed25519 key pair. The private key belongs to app/user only; the
# public key goes to every service and gateway that verifies tokens.
.PHONY: jwt-keys
jwt-keys:
	@go run ./scripts/jwtkeys
