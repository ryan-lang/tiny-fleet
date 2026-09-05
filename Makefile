.PHONY: all build test test-race deb clean

VERSION ?= 0.1.1
ARCH ?= amd64

all: build

build:
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=$(VERSION)" -o fleet .

test:
	go test -v ./...

test-race:
	go test -race -v ./...

deb:
	./scripts/build-deb.sh

clean:
	rm -f fleet tiny-fleet_*.deb
