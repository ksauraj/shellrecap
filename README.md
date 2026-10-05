# K8au Shell Analyzer

An interactive TUI tool to analyze your shell history and provide insights about your command-line usage patterns.

## Table of Contents
- [Features](#features)
- [Installation](#installation)
  - [Pre-built Binaries](#pre-built-binaries)
  - [Quick Install Script](#quick-install-script)
  - [Manual Installation](#manual-installation)
  - [Package Managers](#package-managers)
- [Configuration](#configuration)
- [Usage](#usage)
- [Development](#development)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

## Features
- Shell history analysis
- Tech stack detection
- Productivity metrics
- Work pattern analysis
- Tool usage statistics
- Spotify-Wrapped style recap of your year in the terminal, with ASCII animations and optional AI-written slides

## Installation

### Pre-built Binaries

Download the latest release for your platform:

| Platform | Architecture | Download Link |
|----------|-------------|---------------|
| Linux    | amd64       | [Download](https://github.com/ksauraj/k8au-shell-analyzer/releases/latest/download/k8au-shell-analyser-linux-amd64) |
| Linux    | arm64       | [Download](https://github.com/ksauraj/k8au-shell-analyzer/releases/latest/download/k8au-shell-analyser-linux-arm64) |
| macOS    | amd64       | [Download](https://github.com/ksauraj/k8au-shell-analyzer/releases/latest/download/k8au-shell-analyser-darwin-amd64) |
| macOS    | arm64       | [Download](https://github.com/ksauraj/k8au-shell-analyzer/releases/latest/download/k8au-shell-analyser-darwin-arm64) |
| Windows  | amd64       | [Download](https://github.com/ksauraj/k8au-shell-analyzer/releases/latest/download/k8au-shell-analyser-windows-amd64.exe) |
| Windows  | arm64       | [Download](https://github.com/ksauraj/k8au-shell-analyzer/releases/latest/download/k8au-shell-analyser-windows-arm64.exe) |

### Quick Install Script

#### Linux/macOS (One-line installer)
```bash
curl -L https://raw.githubusercontent.com/ksauraj/k8au-shell-analyzer/master/setup.sh | bash
```

#### Using wget
```bash
wget -qO - https://raw.githubusercontent.com/ksauraj/k8au-shell-analyzer/master/setup.sh | bash
```

### Manual Installation

```bash
# Linux/macOS
wget https://github.com/ksauraj/k8au-shell-analyzer/releases/latest/download/k8au-shell-analyser-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m)
chmod +x k8au-shell-analyser-*
./k8au-shell-analyser-*

# Windows PowerShell
Invoke-WebRequest -Uri "https://github.com/ksauraj/k8au-shell-analyzer/releases/latest/download/k8au-shell-analyser-windows-amd64.exe" -OutFile "k8au-shell-analyser.exe"
```

## Configuration

### Build from Source

Requirements:
- Go 1.20 or higher
- Gemini API Key

```bash
# Clone repository
git clone https://github.com/ksauraj/k8au-shell-analyzer.git
cd k8au-shell-analyzer

# Build with API key
make build GEMINI_API_KEY=your_api_key_here

# Or using go build directly
go build -ldflags "-X github.com/ksauraj/k8au-shell-analyzer/internal/gemini.apiKey=YOUR_API_KEY" ./cmd/k8au-shell-analyzer
```

### Gemini (optional)

The Wrapped view is computed locally from your history. With a Gemini API key it also gets a few
AI-written slides (persona, roast, superpower and forecast). Only aggregate stats such as program
names and counts are sent, never full command lines.

| Variable         | Description                                                          |
|------------------|----------------------------------------------------------------------|
| `GEMINI_API_KEY` | API key, used when none was compiled in with `-ldflags`              |
| `GEMINI_MODEL`   | Model to use, defaults to `gemini-3.8-flash`                         |
| `K8AU_CACHE_DIR` | Where AI slides are cached, defaults to `~/.cache/k8au-shell-analyzer` on Linux |

AI slides are cached per year, so Gemini isn't called on every launch. Cached slides are reused
as long as your stats are unchanged, or for up to a week while your command count stays within
10% of when they were generated. Press `r` to re-read your history and regenerate them.

## Usage

### Basic Usage
```bash
./k8au-shell-analyser
```

### Navigation Keys
| Key                       | Action                                           |
|---------------------------|--------------------------------------------------|
| `Tab` / `Shift+Tab`       | Next / previous view                             |
| `1`-`6`                   | Jump to a view                                   |
| `←/→`                     | Change slides in Wrapped, switch views elsewhere |
| `↑/↓`, `PgUp/PgDn`, mouse | Scroll the current view                          |
| `g` / `G`                 | Jump to top / bottom                             |
| `Space`                   | Pause / resume slide autoplay in Wrapped         |
| `r`                       | Re-read history and regenerate AI slides         |
| `q`                       | Quit application                                 |

### Available Views
1. **Overview**: General statistics
2. **Tech Profile**: Technical expertise analysis
3. **Work Patterns**: Productivity patterns
4. **Tool Usage**: Developer tools usage
5. **Wrapped**: Your year in the terminal: top commands, peak hours, git story, stack, new tools, typos and persona
6. **Timeline**: Your most recent interesting commands

## Development

### Setup Development Environment
```bash
# Clone repository
git clone https://github.com/ksauraj/k8au-shell-analyzer.git
cd k8au-shell-analyzer

# Install dependencies
go mod download

# Run tests
go test ./...

# Run with hot reload (using air)
air
```

## Troubleshooting

### Common Issues

1. **Permission Denied**
```bash
chmod +x k8au-shell-analyser
```

2. **Binary Not Found**
```bash
export PATH=$PATH:$(pwd)
```

3. **API Key Issues**
Ensure you're building with the correct Gemini API key:
```bash
make build GEMINI_API_KEY=your_api_key_here
```

## Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

Distributed under the MIT License. See `LICENSE` for more information.

## Author

**Ksauraj** - [GitHub](https://github.com/ksauraj)

Project Link: [https://github.com/ksauraj/k8au-shell-analyzer](https://github.com/ksauraj/k8au-shell-analyzer)
