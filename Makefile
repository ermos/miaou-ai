BINARY       := miaou-ai
DIST         := dist
PI_ARCH      ?= arm64
GO_IMAGE     := golang:1.25-bookworm
PIPER_VERSION := 2023.11.14-2
# piper's release naming (aarch64) only matches PI_ARCH=arm64; that's the default and
# the only one wired up here, add an armv7l mapping if you ever target a 32-bit Pi OS.
PIPER_URL    := https://github.com/rhasspy/piper/releases/download/$(PIPER_VERSION)/piper_linux_aarch64.tar.gz
WHISPER_MODEL ?= tiny-q5_1

.PHONY: build-pi fetch-piper build-whisper package clean

# Downloads the native Piper TTS engine (C++ binary, no Python) for Linux/arm64
# into assets/piper/, alongside the .onnx voice model already there.
fetch-piper:
	@if [ -x assets/piper/piper ]; then echo "piper already present, skipping"; exit 0; fi
	mkdir -p assets/piper
	curl -sL $(PIPER_URL) | tar -xz -C assets/piper --strip-components=1

# whisper.cpp ships no prebuilt binaries, so unlike fetch-piper this builds
# from source — run it ON the target machine (native compile; cross-compiling
# C++ for arm64 from macOS via Docker/QEMU is impractically slow for a project
# this size). Multilingual model (no .en suffix): whisper.cpp detects the
# spoken language itself, so one model handles English and French alike.
build-whisper:
	@if [ -x assets/whisper/whisper-cli ]; then echo "whisper-cli already present, skipping"; exit 0; fi
	rm -rf /tmp/whisper-build
	git clone --depth 1 https://github.com/ggml-org/whisper.cpp /tmp/whisper-build
	# Static build: whisper-cli otherwise links against libwhisper.so/libggml*.so
	# built into scattered subdirectories, which then have to ship (and be found
	# via LD_LIBRARY_PATH) alongside it. One self-contained binary is simpler.
	cmake -B /tmp/whisper-build/build -S /tmp/whisper-build -DBUILD_SHARED_LIBS=OFF
	cmake --build /tmp/whisper-build/build -j --config Release
	mkdir -p assets/whisper
	cp /tmp/whisper-build/build/bin/whisper-cli assets/whisper/whisper-cli
	sh /tmp/whisper-build/models/download-ggml-model.sh $(WHISPER_MODEL) /tmp/whisper-build/models
	cp /tmp/whisper-build/models/ggml-$(WHISPER_MODEL).bin assets/whisper/
	rm -rf /tmp/whisper-build

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
