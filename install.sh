#!/bin/sh

# Determine the operating system and architecture
OS="$(uname -s)"
ARCH="$(uname -m)"
BINARY_URL=""
TEMP_DIR="/tmp/apito_cli"

# Display OS information
echo "Detected OS: $OS"
echo "Detected Architecture: $ARCH"

# Fetch the latest release tag from GitHub
echo "Fetching the latest Apito CLI release..."
LATEST_TAG=$(curl -s https://api.github.com/repos/apito-io/cli/releases/latest | grep 'tag_name' | cut -d\" -f4)

if [ -z "$LATEST_TAG" ]; then
    echo "Failed to fetch the latest release tag."
    exit 1
fi

echo "Latest release tag: $LATEST_TAG"

# Construct the download URL based on OS and architecture
if [ "$OS" = "Linux" ]; then
    if [ "$ARCH" = "x86_64" ]; then
        BINARY_URL="https://github.com/apito-io/cli/releases/download/$LATEST_TAG/apito-linux-amd64.zip"
    elif [ "$ARCH" = "aarch64" ]; then
        BINARY_URL="https://github.com/apito-io/cli/releases/download/$LATEST_TAG/apito-linux-arm64.zip"
    else
        echo "Unsupported architecture: $ARCH"
        exit 1
    fi
elif [ "$OS" = "Darwin" ]; then
    if [ "$ARCH" = "x86_64" ]; then
        BINARY_URL="https://github.com/apito-io/cli/releases/download/$LATEST_TAG/apito-darwin-amd64.zip"
    elif [ "$ARCH" = "arm64" ]; then
        BINARY_URL="https://github.com/apito-io/cli/releases/download/$LATEST_TAG/apito-darwin-arm64.zip"
    else
        echo "Unsupported architecture: $ARCH"
        exit 1
    fi
else
    echo "Unsupported OS: $OS"
    exit 1
fi

# Display the download URL
echo "Downloading Apito CLI from: $BINARY_URL"

# Create a temporary directory for the download
mkdir -p $TEMP_DIR

# Download the binary zip with progress display
curl -L --progress-bar -o "$TEMP_DIR/apito.zip" "$BINARY_URL"

# Unzip the downloaded file
echo "Unzipping the Apito CLI..."
unzip -o "$TEMP_DIR/apito.zip" -d "$TEMP_DIR"

# Move the binary to /usr/local/bin and make it executable
mv "$TEMP_DIR/apito" /usr/local/bin/apito
chmod +x /usr/local/bin/apito

# Clean up the temporary directory
rm -rf $TEMP_DIR

# Verify installation
if command -v apito > /dev/null; then
    echo "Apito CLI installed successfully!"
else
    echo "Installation failed!"
    exit 1
fi
