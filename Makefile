.PHONY: build clean deploy test local

# Build the Lambda function
build:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -o cmd/lambda/bootstrap cmd/lambda/*.go

# Clean build artifacts
clean:
	rm -f cmd/lambda/bootstrap
	rm -rf .aws-sam

# Deploy to AWS
deploy: build
	sam deploy --resolve-s3

# Run tests
test:
	go test -v ./...

# Run locally (requires environment variables)
local:
	go run cmd/lambda/*.go

# Download dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Generate go.sum
mod:
	go mod tidy
