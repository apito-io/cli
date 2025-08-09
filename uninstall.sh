#!/bin/bash

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}[WARNING]${NC} This will remove the Apito CLI binary from your system."
echo -n "Proceed? [y/N]: "
read -r confirm
if [[ ! "$confirm" =~ ^([yY]|[yY][eE][sS])$ ]]; then
  echo "Aborted."
  exit 0
fi

apito_path=$(command -v apito 2>/dev/null || true)
if [ -z "$apito_path" ]; then
  echo -e "${RED}[ERROR]${NC} apito not found in PATH."
  exit 1
fi

if rm -f "$apito_path" 2>/dev/null; then
  echo -e "${GREEN}[SUCCESS]${NC} Removed $apito_path"
  exit 0
fi

echo -e "${YELLOW}[WARNING]${NC} Failed to remove without sudo. Trying with sudo..."
if sudo rm -f "$apito_path"; then
  echo -e "${GREEN}[SUCCESS]${NC} Removed $apito_path"
  exit 0
fi

echo -e "${RED}[ERROR]${NC} Failed to remove $apito_path"
exit 1

