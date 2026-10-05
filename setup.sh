#!/bin/bash

# Function to fetch the download URL for the latest release
fetch_download_url() {
  local os=$(uname -s | tr '[:upper:]' '[:lower:]')
  local arch=$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/; s/armv7l/arm/') # Map common architectures

  # Debug output to stderr
  echo "Detected OS: $os, Architecture: $arch" >&2

  # Fetch the latest release JSON and extract the download URL
  local url=$(curl -s https://api.github.com/repos/ksauraj/shellrecap/releases/latest | grep -oP '"browser_download_url": "\K[^"]+' | grep "$os" | grep -E "$arch|armv7l|aarch64")

  if [[ -z "$url" ]]; then
    echo "Error: No binary found for OS: $os, Architecture: $arch." >&2
    exit 1
  fi

  # Sanitize the URL by removing any trailing whitespace or special characters
  url=$(echo "$url" | tr -d '\r')

  # Debug output to stderr
  echo "Download URL: $url" >&2

  # Return the URL to stdout
  echo "$url"
}

# Function to download and run the binary
download_and_run() {
  local url=$1
  local binary_name="shellrecap"

  echo "Downloading binary from URL: $url" >&2
  if ! curl -L -o "$binary_name" "$url"; then
    echo "Error: Failed to download the binary. Please check your internet connection and try again." >&2
    exit 1
  fi

  echo "Making the binary executable..." >&2
  chmod +x "$binary_name"

  echo "Running the binary..." >&2
  ./"$binary_name"
}

# Main script execution
download_url=$(fetch_download_url)
if [[ -n "$download_url" ]]; then
  download_and_run "$download_url"
else
  echo "Error: Unable to fetch download URL." >&2
  exit 1
fi
