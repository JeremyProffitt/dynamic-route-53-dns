.PHONY: build test clean deploy local validate lint

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bootstrap ./cmd/lambda/main.go

test:
	go test -v -race ./...

clean:
	rm -f bootstrap

deploy:
	sam deploy

local:
	sam local start-api

validate:
	sam validate

lint:
	golangci-lint run
