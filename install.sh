#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_step() {
    echo -e "${PURPLE}[STEP]${NC} $1"
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to detect OS and architecture
detect_system() {
    local os=""
    local arch=""
    
    # Detect OS
    case "$(uname -s)" in
        Linux*)     os="linux" ;;
        Darwin*)    os="darwin" ;;
        CYGWIN*)    os="windows" ;;
        MINGW*)     os="windows" ;;
        MSYS*)      os="windows" ;;
        *)          os="unknown" ;;
    esac
    
    # Detect architecture
    case "$(uname -m)" in
        x86_64)     arch="amd64" ;;
        amd64)      arch="amd64" ;;
        arm64)      arch="arm64" ;;
        aarch64)    arch="arm64" ;;
        armv7l)     arch="armv7" ;;
        armv6l)     arch="armv6" ;;
        *)          arch="unknown" ;;
    esac
    
    echo "$os:$arch"
}

# Function to check if running as root
is_root() {
    [ "$(id -u)" -eq 0 ]
}

# Function to check if sudo is available
has_sudo() {
    command_exists sudo
}

# Function to get installation directory
get_install_dir() {
    if is_root; then
        echo "/usr/local/bin"
    elif [ -w "/usr/local/bin" ]; then
        echo "/usr/local/bin"
    elif [ -w "$HOME/.local/bin" ]; then
        echo "$HOME/.local/bin"
    else
        echo "$HOME/bin"
    fi
}

# Function to ensure installation directory exists
ensure_install_dir() {
    local install_dir="$1"
    
    if [ ! -d "$install_dir" ]; then
        print_status "Creating installation directory: $install_dir"
        mkdir -p "$install_dir"
    fi
    
    # Add to PATH if not already there
    if [[ ":$PATH:" != *":$install_dir:"* ]]; then
        print_warning "Adding $install_dir to PATH"
        echo "export PATH=\"$install_dir:\$PATH\"" >> "$HOME/.bashrc"
        echo "export PATH=\"$install_dir:\$PATH\"" >> "$HOME/.zshrc" 2>/dev/null || true
        print_warning "Please restart your terminal or run: source ~/.bashrc"
    fi
}

# Function to download with progress
download_with_progress() {
    local url="$1"
    local output="$2"
    
    if command_exists curl; then
        curl -L --progress-bar -o "$output" "$url"
    elif command_exists wget; then
        wget --progress=bar:force -O "$output" "$url"
    else
        print_error "Neither curl nor wget is installed. Please install one of them."
        exit 1
    fi
}

# Function to extract archive
extract_archive() {
    local archive="$1"
    local output_dir="$2"
    
    if command_exists unzip; then
        unzip -o "$archive" -d "$output_dir"
    elif command_exists tar; then
        tar -xf "$archive" -C "$output_dir"
    else
        print_error "Neither unzip nor tar is installed. Please install one of them."
        exit 1
    fi
}

# Main installation function
main() {
    print_step "🚀 Starting Apito CLI Installation"
    echo
    
    # Detect system
    print_status "Detecting system..."
    local system_info=$(detect_system)
    local os=$(echo "$system_info" | cut -d: -f1)
    local arch=$(echo "$system_info" | cut -d: -f2)
    
    print_success "Detected OS: $os"
    print_success "Detected Architecture: $arch"
    echo
    
    # Validate system
    if [ "$os" = "unknown" ] || [ "$arch" = "unknown" ]; then
        print_error "Unsupported system: $(uname -s) $(uname -m)"
        exit 1
    fi
    
    # Fetch latest release
    print_step "📦 Fetching latest release..."
    local latest_tag=$(curl -s https://api.github.com/repos/apito-io/cli/releases/latest | grep 'tag_name' | cut -d\" -f4)
    
    if [ -z "$latest_tag" ]; then
        print_error "Failed to fetch the latest release tag."
        exit 1
    fi
    
    print_success "Latest release: $latest_tag"
    echo
    
    # Construct download URL
    local binary_url="https://github.com/apito-io/cli/releases/download/$latest_tag/apito_${latest_tag#v}_${os}_${arch}.tar.gz"
    print_status "Download URL: $binary_url"
    echo
    
    # Setup directories
    local temp_dir=$(mktemp -d)
    local install_dir=$(get_install_dir)
    
    print_status "Temporary directory: $temp_dir"
    print_status "Installation directory: $install_dir"
    echo
    
    # Ensure installation directory exists
    ensure_install_dir "$install_dir"
    
    # Download binary
    print_step "⬇️  Downloading Apito CLI..."
    local archive_file="$temp_dir/apito.tgz"
    download_with_progress "$binary_url" "$archive_file"

    if [ ! -f "$archive_file" ]; then
        print_error "Download failed!"
        rm -rf "$temp_dir"
        exit 1
    fi
    
    print_success "Download completed!"
    echo
    
    # Extract binary
    print_step "📂 Extracting binary..."
    extract_archive "$archive_file" "$temp_dir"
    
    local binary_path="$temp_dir/apito"
    if [ ! -f "$binary_path" ]; then
        print_error "Binary not found in archive!"
        rm -rf "$temp_dir"
        exit 1
    fi
    
    print_success "Extraction completed!"
    echo
    
    # Install binary
    print_step "🔧 Installing binary..."
    local target_path="$install_dir/apito"
    
    # Check if binary already exists
    if [ -f "$target_path" ]; then
        print_warning "Binary already exists. Overwriting..."
    fi
    
    # Copy binary with proper permissions
    if is_root || [ -w "$install_dir" ]; then
        cp "$binary_path" "$target_path"
        chmod +x "$target_path"
    else
        if has_sudo; then
            print_status "Requesting sudo permissions to install binary..."
            sudo cp "$binary_path" "$target_path"
            sudo chmod +x "$target_path"
        else
            print_error "Cannot write to $install_dir and sudo is not available."
            print_error "Please run this script as root or install sudo."
            rm -rf "$temp_dir"
            exit 1
        fi
    fi
    
    print_success "Binary installed successfully!"
    echo
    
    # Cleanup
    print_step "🧹 Cleaning up..."
    rm -rf "$temp_dir"
    print_success "Cleanup completed!"
    echo
    
    # Verify installation
    print_step "✅ Verifying installation..."
    if command -v apito >/dev/null 2>&1; then
        local version=$(apito --version 2>/dev/null || echo "unknown version")
        print_success "Apito CLI installed successfully!"
        print_success "Version: $version"
        print_success "Location: $(which apito)"
        echo
        print_success "🎉 Installation completed! You can now use 'apito' command."
    else
        print_error "Installation verification failed!"
        print_warning "The binary was installed but not found in PATH."
        print_warning "Please restart your terminal or run: source ~/.bashrc"
        exit 1
    fi
}

# Handle script interruption
trap 'print_error "Installation interrupted!"; exit 1' INT TERM

# Run main function
main "$@"
