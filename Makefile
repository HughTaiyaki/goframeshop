include hack/hack-cli.mk

.PHONY: env-check
env-check:
	@./scripts/check-env.sh

.PHONY: infra-up
infra-up:
	@docker compose up -d mysql etcd redis elasticsearch rabbitmq otel-collector jaeger prometheus loki

.PHONY: infra-down
infra-down:
	@docker compose down

# Applies every init script to a running MySQL. The scripts are idempotent, so
# this is how an existing volume picks up a newly added schema without the data
# loss of infra-reset.
# Creates missing local configs from the .example files, with the passwords
# from .env. Never overwrites a file that exists.
.PHONY: dev-config
dev-config:
	@./scripts/dev-config.sh

.PHONY: db-init
db-init:
	@for f in deploy/mysql/init/*.sql deploy/mysql/init/*.sh; do echo "$$f"; done | sort | while read -r f; do \
		echo "applying $$f"; \
		case "$$f" in \
		*.sql) docker compose exec -T mysql sh -c 'mysql -uroot -p"$$MYSQL_ROOT_PASSWORD"' < "$$f" || exit 1 ;; \
		*.sh)  docker compose exec -T mysql sh -s < "$$f" || exit 1 ;; \
		esac; \
	done

.PHONY: infra-reset
infra-reset:
	@docker compose down -v

.PHONY: infra-ps
infra-ps:
	@docker compose ps

.PHONY: infra-logs
infra-logs:
	@docker compose logs -f mysql etcd redis elasticsearch rabbitmq otel-collector jaeger prometheus loki

.PHONY: verify
verify:
	@go mod verify
	@go test ./...

# Fault experiments (task 0021, design §10): builds and starts user, goods,
# order and the payment simulator, injects faults, checks the invariants and
# writes reports/fault/<time>/report.md. Needs `make infra-up` and the
# service ports free (stop locally running copies first).
.PHONY: fault-trade
fault-trade:
	@GOFRAMESHOP_FAULT=1 go test ./tests/fault/ -run TestFaultTrade -count=1 -timeout 20m -v

# Prints a new Ed25519 key pair. The private key belongs to app/user only; the
# public key goes to every service and gateway that verifies tokens.
.PHONY: jwt-keys
jwt-keys:
	@go run ./scripts/jwtkeys
