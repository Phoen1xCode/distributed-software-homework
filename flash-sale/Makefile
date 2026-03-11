
.PHONY: dev/up dev/down dev/restart dev/reset

dev/up:
	docker compose -f docker-compose.dev.yaml up -d
	echo "PostgreSQL is running. Verify with: docker exec flash-sale-postgres pg_isready -U flashsale"

dev/down:
	docker compose -f docker-compose.dev.yaml down

dev/reset:
	docker compose -f docker-compose.dev.yaml down -v
	docker compose -f docker-compose.dev.yaml up -d

dev/restart:
	docker compose -f docker-compose.dev.yaml restart