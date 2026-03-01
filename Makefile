SHELL := /bin/bash

BINARY := webhook-service
BUILD_DIR := bin
IMAGE := gitea-jenkins-webhook
GO_FILES := $(shell find . -name '*.go' -not -path "./vendor/*")

.PHONY: all build test test-unit test-integration lint fmt tidy clean docker-build docker-run docker-compose ci

all: build

fmt:
	gofmt -w $(GO_FILES)

tidy:
	go mod tidy

lint:
	go vet ./...

# Юнит-тесты с покрытием, интеграционные тесты, отчёт в консоль и coverage.html
test:
	go test -race -tags=integration -covermode=atomic -coverprofile=coverage.out ./internal/... ./pkg/...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html

build:
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/webhook-service

ci: tidy lint test build

clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html

run-server:
	go run ./cmd/webhook-service run -config ./config.yaml --debug

run-check:
	go run ./cmd/webhook-service check -config ./config.yaml

run-check-debug:
	go run ./cmd/webhook-service check -config ./config.yaml --debug

run-build: build
	./bin/webhook-service -config ./config.yaml

docker-build:
	docker build --network host -t $(IMAGE) .

docker-run: docker-build
	docker run --rm -p 8081:8081 -v $(PWD)/config.example.yaml:/etc/webhook/config.yaml $(IMAGE)

docker-compose:
	docker compose up -d --build
