# Makefile

# Change this to match your Go package name
APP_NAME=lazer

# Target architecture (ARMv7 for Raspberry Pi 3/4)
GOARCH=arm
GOOS=linux
GOARM=7

# SSH and SCP details
PI_USER=ubuntu
PI_HOST=192.168.1.252
PI_PATH=/home/ubuntu/

# Build directory
BUILD_DIR=build

all: build

build:
	@echo "Building $(APP_NAME) for Raspberry Pi..."
	GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) go build -o $(BUILD_DIR)/$(APP_NAME) .

scp: build
	@echo "Copying binary to Raspberry Pi..."
	scp $(BUILD_DIR)/$(APP_NAME) $(PI_USER)@$(PI_HOST):$(PI_PATH)

clean:
	@echo "Cleaning build directory..."
	rm -rf $(BUILD_DIR)

.PHONY: all build scp clean
