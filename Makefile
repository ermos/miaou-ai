BINARY       := miaou-ai
DIST         := dist
PI_ARCH      ?= arm64
GO_IMAGE     := golang:1.25-bookworm
PIPER_VERSION := 2023.11.14-2
# piper's release naming (aarch64) only matches PI_ARCH=arm64; that's the default and
# the only one wired up here, add an armv7l mapping if you ever target a 32-bit Pi OS.
PIPER_URL    := https://github.com/rhasspy/piper/releases/download/$(PIPER_VERSION)/piper_linux_aarch64.tar.gz

.PHONY: build-pi fetch-piper package clean

# Downloads the native Piper TTS engine (C++ binary, no Python) for Linux/arm64
# into assets/piper/, alongside the .onnx voice model already there.
fetch-piper:
	@if [ -x assets/piper/piper ]; then echo "piper already present, skipping"; exit 0; fi
	mkdir -p assets/piper
	curl -sL $(PIPER_URL) | tar -xz -C assets/piper --strip-components=1

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
package: fetch-piper build-pi
	rm -rf $(DIST)/package
	mkdir -p $(DIST)/package
	cp $(DIST)/$(BINARY) $(DIST)/package/
	cp -r assets $(DIST)/package/
	cp PERSONALITY.md $(DIST)/package/
	tar -czf $(DIST)/$(BINARY)-pi-$(PI_ARCH).tar.gz -C $(DIST)/package .
	@echo "Package ready: $(DIST)/$(BINARY)-pi-$(PI_ARCH).tar.gz"

clean:
	rm -rf $(DIST)
