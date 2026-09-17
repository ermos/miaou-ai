BINARY       := miaou-ai
DIST         := dist
PI_ARCH      ?= arm64
GO_IMAGE     := golang:1.25-bookworm

.PHONY: build-pi package clean

build-pi:
	mkdir -p $(DIST)
	docker run --rm --platform=linux/$(PI_ARCH) \
		-v $(CURDIR):/src -w /src \
		-e CGO_ENABLED=1 -e GOOS=linux -e GOARCH=$(PI_ARCH) \
		-e GOCACHE=/tmp/gocache \
		$(GO_IMAGE) \
		sh -c "apt-get update -qq && apt-get install -y -qq gcc libc6-dev libx11-dev libxcursor-dev libxrandr-dev libxinerama-dev libxi-dev libxxf86vm-dev libgl1-mesa-dev pkg-config >/dev/null && go build -o $(DIST)/$(BINARY) ."

# Bundles the binary with everything it loads at runtime (assets/, PERSONALITY.md)
# into a tarball ready to scp to the Pi. .env is deliberately left out, copy it
# yourself so secrets never sit in a build artifact.
package: build-pi
	rm -rf $(DIST)/package
	mkdir -p $(DIST)/package
	cp $(DIST)/$(BINARY) $(DIST)/package/
	cp -r assets $(DIST)/package/
	cp PERSONALITY.md $(DIST)/package/
	tar -czf $(DIST)/$(BINARY)-pi-$(PI_ARCH).tar.gz -C $(DIST)/package .
	@echo "Package ready: $(DIST)/$(BINARY)-pi-$(PI_ARCH).tar.gz"

clean:
	rm -rf $(DIST)
