postgres:
	docker run --name postgres -p 5432:5432 -e POSTGRES_USER=root -e POSTGRES_PASSWORD=secret -d postgres

createdb:
	docker exec -it postgres createdb --username=root --owner=root simple_bank

dropdb:
	docker exec -it postgres dropdb simple_bank

migrateup:
	soda migrate -p ./db/migrations -c ./db/database.yml

migratedown:
	soda migrate down -p ./db/migrations -c ./db/database.yml

.PHONY: postgres createdb dropdb migrateup migratedown