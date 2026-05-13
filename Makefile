.PHONY: test race build run docker-build docker-up docker-down clean

test:
	go test ./...

race:
	go test -race ./...

build:
	go build -o bin/gostream-broker ./cmd/gostream-broker
	go build -o bin/gostream ./cmd/gostream

run:
	GOSTREAM_DATA_DIR=data go run ./cmd/gostream-broker

docker-build:
	docker build -t gostream:local .

docker-up:
	docker compose -f deployments/docker-compose.yml up --build

docker-down:
	docker compose -f deployments/docker-compose.yml down -v

clean:
	rm -rf bin data

