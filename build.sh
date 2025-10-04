#!/bin/bash

# Define the target platforms and architectures
PLATFORMS=("windows/amd64" "windows/386" "darwin/amd64" "linux/amd64" "linux/386" "linux/arm" "linux/arm64")

# Create the build directory
mkdir -p build

# Iterate through the platforms and build the binary
for platform in "${PLATFORMS[@]}"
do
    # Split the platform string into OS and architecture
    GOOS=$(echo $platform | cut -d'/' -f1)
    GOARCH=$(echo $platform | cut -d'/' -f2)

    # Set the output filename
    OUTPUT_NAME="build/filedot-dl_${GOOS}_${GOARCH}"
    if [ $GOOS = "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi

    # Build the binary
    echo "Building for ${GOOS}/${GOARCH}..."
    env GOOS=$GOOS GOARCH=$GOARCH go build -o $OUTPUT_NAME .
done

echo "Build complete."
