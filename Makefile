# 1. RUN SIMPLE_BANK USING POSTGRES CONTAINER AND `make server`
# 1.1 run postgres
postgres:
	docker run --name postgres -p 5432:5432 -e POSTGRES_USER=root -e POSTGRES_PASSWORD=secret -d postgres:13.12

# 1.2 create db
# 	make createdb

# 1.3 migrate simple_bank db
#	make migrateup

# 1.4 run server
# 	make server

# 2. RUN SIMPLE_BANK USING DOCKER NETWORK TO CONNECT 2 STAND-ALONE CONTAINERS 
# 2.1 create network
network:
	docker network create bank-network

# 2.2 run postgres container with network 
postgreswithnetwork:
	docker run --name postgres --network bank-network -p 5432:5432 -e POSTGRES_USER=root -e POSTGRES_PASSWORD=secret -d postgres:13.12

# 2.3 create db
# 	make createdb

# 2.4 migrate simple_bank db
#	make migrateup

# 2.5 build app into docker image
build:
	docker build -t simplebank:latest .

# 2.6 run app with network 
appwithnetwork:
	docker run --name simplebank --network bank-network -p 8080:8080 -e GIN_MODE=release -e DB_SOURCE="postgresql://root:secret@postgres:5432/simple_bank?sslmode=disable" simplebank:latest

# (optional) connect postgres container with network if not yet
connectdb:
	docker network connect bank-network postgres

# 3. RUN SIMPLE_BANK USING DOCKER COMPOSE
# 3.1 docker compose up
composeup:
	docker compose up

# (optional) delete/clear docker compose
#	docker compose down
#	docker rmi <image>

# -------------------(general)------------------------

# create simple_bank database on postgres container
createdb:
	docker exec -it postgres createdb --username=root --owner=root simple_bank

# drop simple_bank database on postgres container
dropdb:
	docker exec -it postgres dropdb simple_bank

# create a new db migration
# soda generate -c ./db/database.yml -p ./db/migrations sql <migration_name>

# migrate simple_bank database from app to postgres container
migrateup:
	soda migrate -p ./db/migrations -c ./db/database.yml

# migrate all down simple_bank database from app to postgres container
migratedownall:
	soda migrate down -p ./db/migrations -c ./db/database.yml --step 4

# migrate 1 down simple_bank database from app to postgres container
migratedown1:
	soda migrate down -p ./db/migrations -c ./db/database.yml --step 1

# generate a new migration
new_migration:
	soda generate sql -p ./db/migrations -c ./db/database.yml $(name)

# build database document
db_docs:
	dbdocs build doc/db.dbml

# build database schema
db_schema:
	dbml2sql --postgres -o doc/schema.sql doc/db.dbml

# generate queries to golang code
sqlc:
	docker run --rm -v "${CURDIR}:/src" -w /src sqlc/sqlc:1.20.0 generate

# run test
test:
	go test -v -cover -short ./...
	# go test -v -cover -coverprofile cover.out -outputdir ./covers/ ./...
	# go tool cover -html ./covers/cover.out -o ./covers/cover.html

server:
	go run main.go

# generate gomock for testing
mock:
	mockgen -package mockdb -destination db/mock/store.go github.com/thanhquy1105/simplebank/db/sqlc Store

proto:

# For mac
# rm -f pb/*.go
# rm -f doc/swagger/*.swagger.json

# For windows
	del pb\*.go
	del doc\swagger\*.swagger.json

	protoc --proto_path=proto --go_out=pb --go_opt=paths=source_relative \
	--go-grpc_out=pb --go-grpc_opt=paths=source_relative \
	--grpc-gateway_out=pb --grpc-gateway_opt=paths=source_relative \
	--openapiv2_out=doc/swagger --openapiv2_opt=allow_merge=true,merge_file_name=simple_bank \
	proto/*.proto

evans:
	evans --host localhost -p 9090 -r repl

redis:
	docker run --name redis -p 6379:6379 -d redis:7.2.1

.PHONY: postgres createdb dropdb migrateup migratedown sqlc test proto evans redis db_docs db_schema new_migration