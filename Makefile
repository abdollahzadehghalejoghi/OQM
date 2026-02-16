.PHONY: build clean install test cross-compile

VERSION ?= 1.0.0
BINARY_NAME = oqm
BUILD_DIR = build

# Build for current platform
build:
	@echo "Building $(BINARY_NAME)..."
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/oqm

# Build for OpenWrt targets
cross-compile:
	@echo "Cross-compiling for OpenWrt targets..."
	@mkdir -p $(BUILD_DIR)

	# MIPS (most common OpenWrt routers)
	GOOS=linux GOARCH=mips GOMIPS=softfloat CGO_ENABLED=0 \
		go build -ldflags="-s -w -X main.version=$(VERSION)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-mips ./cmd/oqm

	# MIPSLE
	GOOS=linux GOARCH=mipsle GOMIPS=softfloat CGO_ENABLED=0 \
		go build -ldflags="-s -w -X main.version=$(VERSION)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-mipsle ./cmd/oqm

	# ARM (Raspberry Pi, etc)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
		go build -ldflags="-s -w -X main.version=$(VERSION)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-arm ./cmd/oqm

	# ARM64
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
		go build -ldflags="-s -w -X main.version=$(VERSION)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-arm64 ./cmd/oqm

	# x86_64 (PC Engines APU, etc)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
		go build -ldflags="-s -w -X main.version=$(VERSION)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-amd64 ./cmd/oqm

	@echo "Cross-compilation complete!"
	@ls -lh $(BUILD_DIR)

# Compress binaries with UPX (optional)
compress: cross-compile
	@echo "Compressing binaries with UPX..."
	@for file in $(BUILD_DIR)/$(BINARY_NAME)-*; do \
		upx --best --lzma $$file 2>/dev/null || echo "UPX not available, skipping compression"; \
	done

# Install locally
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	go clean

# Run tests
test:
	go test -v ./...

# Create release archives
release: cross-compile
	@echo "Creating release archives..."
	@mkdir -p $(BUILD_DIR)/release
	@for arch in mips mipsle arm arm64 amd64; do \
		tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-$$arch.tar.gz \
			-C $(BUILD_DIR) $(BINARY_NAME)-$$arch \
			-C ../openwrt/files/etc/init.d oqm \
			-C ../../../README.md ; \
	done
	@echo "Release archives created in $(BUILD_DIR)/release/"

# Development build with race detector
dev:
	go build -race -o $(BUILD_DIR)/$(BINARY_NAME)-dev ./cmd/oqm

# Format code
fmt:
	go fmt ./...
	goimports -w .

# Lint code
lint:
	golangci-lint run ./...

# Show help
help:
	@echo "Available targets:"
	@echo "  build          - Build for current platform"
	@echo "  cross-compile  - Build for all OpenWrt architectures"
	@echo "  compress       - Cross-compile and compress with UPX"
	@echo "  install        - Install locally"
	@echo "  clean          - Clean build artifacts"
	@echo "  test           - Run tests"
	@echo "  release        - Create release archives"
	@echo "  dev            - Build with race detector"
	@echo "  fmt            - Format code"
	@echo "  lint           - Lint code"
