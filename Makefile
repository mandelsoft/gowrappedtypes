.PHONY: all build test generate clean

all: generate build test

build:
	go build ./...

test:
	go test ./...

generate:
	go generate ./...

clean:
	find . -name '*_gowtgen.go' -delete
