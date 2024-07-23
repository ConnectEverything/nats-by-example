GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

# Requires: go install github.com/githubnemo/CompileDaemon@master
# for the multi-build support.
watch:
	CompileDaemon \
		-color=true \
		-pattern="(.+\.go|.+\.html|.+\.css|.+\.svg|.+\.yaml|.+\.cast)$$" \
		-exclude-dir="html" \
		-exclude-dir="docker" \
		-exclude-dir="dist" \
		-exclude-dir=".git" \
		-build="make build" \
		-build="nbe build" \
		-command="nbe serve" \
		-graceful-kill

build:
	mkdir -p dist/$(GOOS)-$(GOARCH)
	go build -o dist/$(GOOS)-$(GOARCH)/nbe$(DIST_EXT) ./cmd/nbe

zip:
	cd dist/$(GOOS)-$(GOARCH) && zip ../$(GOOS)-$(GOARCH).zip nbe$(DIST_EXT)

dist:
	GOOS=linux GOARCH=amd64 make build zip
	GOOS=linux GOARCH=arm64 make build zip
	GOOS=darwin GOARCH=amd64 make build zip
	GOOS=darwin GOARCH=arm64 make build zip
	GOOS=windows GOARCH=amd64 DIST_EXT=".exe" make build zip
	GOOS=windows GOARCH=arm64 DIST_EXT=".exe" make build zip

.PHONY: dist
