.PHONY: test build demo vet check

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

build:
	mkdir -p bin
	CGO_ENABLED=1 go build -trimpath -o bin/finder ./cmd/finder

demo:
	go run ./cmd/finder demo

check: test vet build
