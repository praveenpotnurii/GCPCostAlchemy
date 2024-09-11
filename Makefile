# Makefile for GCP Cost Alchemy project

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=gcpcostalchemy
BINARY_UNIX=$(BINARY_NAME)_unix

# Main package path
MAIN_PACKAGE=.

all: test build

build:
	$(GOBUILD) -o $(BINARY_NAME) -v $(MAIN_PACKAGE)

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)

run:
	$(GOBUILD) -o $(BINARY_NAME) -v $(MAIN_PACKAGE)
	./$(BINARY_NAME)

deps:
	$(GOGET) github.com/joho/godotenv
	$(GOGET) google.golang.org/api/compute/v1
	$(GOGET) google.golang.org/api/recommender/v1
	$(GOGET) google.golang.org/api/option

# Cross compilation
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v $(MAIN_PACKAGE)

docker-build:
	docker build -t $(BINARY_NAME):latest .

# View logs
logs:
	tail -f app.log

# Run with logging
run-with-logs:
	$(GOBUILD) -o $(BINARY_NAME) -v $(MAIN_PACKAGE)
	./$(BINARY_NAME) & tail -f app.log


.PHONY: all build test clean run deps build-linux docker-build logs run-with-logs