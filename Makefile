.PHONY: all build test generate clean

all: generate build test

build:
	go build ./...

test:
	go test ./...

generate:
	go generate ./...

clean:
	rm -f examples/basic/service_gowtgen.go
	rm -f examples/full/endpoint_gowtgen.go
