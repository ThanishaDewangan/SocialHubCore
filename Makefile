.PHONY: dev test lint seed build clean docker-up docker-down

dev:
	go run cmd/api/main.go

worker:
	go run cmd/worker/main.go

test:
	go test -v ./...

lint:
	go fmt ./...
	go vet ./...

seed:
	@echo "Seeding database with test data..."
	@go run scripts/seed.go

build:
	go build -o bin/api cmd/api/main.go
	go build -o bin/worker cmd/worker/main.go

clean:
	rm -rf bin/

docker-up:
	docker-compose up --build

docker-down:
	docker-compose down -v

tidy:
	go mod tidy
