.PHONY: all build test test-race deb darwin dist clean checksums

VERSION ?= 0.2.0
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

darwin:
	mkdir -p dist/staging-darwin-amd64 dist/staging-darwin-arm64 dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X main.Version=$(VERSION)" -o dist/staging-darwin-amd64/fleet .
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X main.Version=$(VERSION)" -o dist/staging-darwin-arm64/fleet .
	cp io.github.ryan-lang.tiny-fleet.plist README.md QUICKSTART.md dist/staging-darwin-amd64/
	cp io.github.ryan-lang.tiny-fleet.plist README.md QUICKSTART.md dist/staging-darwin-arm64/
	(cd dist/staging-darwin-amd64 && tar -czf ../tiny-fleet_$(VERSION)_darwin_amd64.tar.gz fleet io.github.ryan-lang.tiny-fleet.plist README.md QUICKSTART.md)
	(cd dist/staging-darwin-arm64 && tar -czf ../tiny-fleet_$(VERSION)_darwin_arm64.tar.gz fleet io.github.ryan-lang.tiny-fleet.plist README.md QUICKSTART.md)
	rm -rf dist/staging-darwin-amd64 dist/staging-darwin-arm64

dist: deb darwin
	mkdir -p dist
	cp tiny-fleet_$(VERSION)_amd64.deb dist/
	tar -czf dist/tiny-fleet_$(VERSION)_linux_amd64.tar.gz fleet fleet.service README.md QUICKSTART.md
	$(MAKE) checksums

checksums:
	cd dist && sha256sum tiny-fleet_* > checksums.txt

clean:
	rm -rf fleet tiny-fleet_*.deb dist
