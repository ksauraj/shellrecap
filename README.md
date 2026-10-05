# shellrecap

Your year in the terminal. shellrecap is an interactive TUI that reads your shell history and turns
it into insights about how you work, plus a Spotify-Wrapped style recap of your year.

## Table of Contents
- [Features](#features)
- [Installation](#installation)
  - [Pre-built Binaries](#pre-built-binaries)
  - [Quick Install Script](#quick-install-script)
  - [Go Install](#go-install)
  - [Manual Installation](#manual-installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [Development](#development)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

## Features
- Shell history analysis for bash, zsh and fish
- Tech stack detection
- Productivity metrics
- Work pattern analysis
- Tool usage statistics
- A yearly recap with ASCII animations and optional AI-written slides

## Installation

### Pre-built Binaries

Download the latest release for your platform:

| Platform | Architecture | Download Link |
|----------|-------------|---------------|
| Linux    | amd64       | [Download](https://github.com/ksauraj/shellrecap/releases/latest/download/shellrecap-linux-amd64) |
| Linux    | arm64       | [Download](https://github.com/ksauraj/shellrecap/releases/latest/download/shellrecap-linux-arm64) |
| macOS    | amd64       | [Download](https://github.com/ksauraj/shellrecap/releases/latest/download/shellrecap-darwin-amd64) |
| macOS    | arm64       | [Download](https://github.com/ksauraj/shellrecap/releases/latest/download/shellrecap-darwin-arm64) |
| Windows  | amd64       | [Download](https://github.com/ksauraj/shellrecap/releases/latest/download/shellrecap-windows-amd64.exe) |
| Windows  | arm64       | [Download](https://github.com/ksauraj/shellrecap/releases/latest/download/shellrecap-windows-arm64.exe) |

### Quick Install Script

#### Linux/macOS (One-line installer)
```bash
curl -L https://raw.githubusercontent.com/ksauraj/shellrecap/master/setup.sh | bash
```

#### Using wget
```bash
wget -qO - https://raw.githubusercontent.com/ksauraj/shellrecap/master/setup.sh | bash
```

### Go Install

```bash
go install github.com/ksauraj/shellrecap/cmd/shellrecap@latest
```

### Manual Installation

```bash
# Linux/macOS
wget https://github.com/ksauraj/shellrecap/releases/latest/download/shellrecap-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')
chmod +x shellrecap-*
./shellrecap-*

# Windows PowerShell
Invoke-WebRequest -Uri "https://github.com/ksauraj/shellrecap/releases/latest/download/shellrecap-windows-amd64.exe" -OutFile "shellrecap.exe"
```

## Configuration

### Build from Source

Requirements:
- Go 1.20 or higher
- A Gemini API key (optional, for the AI-written slides)

```bash
# Clone repository
git clone https://github.com/ksauraj/shellrecap.git
cd shellrecap

# Build, optionally compiling in an API key
make build
make build GEMINI_API_KEY=your_api_key_here

# Or using go build directly
go build -ldflags "-X github.com/ksauraj/shellrecap/internal/gemini.apiKey=YOUR_API_KEY" ./cmd/shellrecap
```

### Gemini (optional)

The Recap view is computed locally from your history. With a Gemini API key it also gets a few
AI-written slides (persona, roast, superpower and forecast). Only aggregate stats such as program
names and counts are sent, never full command lines.

| Variable               | Description                                                      |
|------------------------|------------------------------------------------------------------|
| `GEMINI_API_KEY`       | API key, used when none was compiled in with `-ldflags`          |
| `GEMINI_MODEL`         | Model to use, defaults to `gemini-3.8-flash`                     |
| `SHELLRECAP_CACHE_DIR` | Where AI slides are cached, defaults to `~/.cache/shellrecap` on Linux |

AI slides are cached per year, so Gemini isn't called on every launch. Cached slides are reused
as long as your stats are unchanged, or for up to a week while your command count stays within
10% of when they were generated. Press `r` to re-read your history and regenerate them.

## Usage

### Basic Usage
```bash
shellrecap
```

### Navigation Keys
| Key                       | Action                                         |
|---------------------------|------------------------------------------------|
| `Tab` / `Shift+Tab`       | Next / previous view                           |
| `1`-`6`                   | Jump to a view                                 |
| `←/→`                     | Change slides in Recap, switch views elsewhere |
| `↑/↓`, `PgUp/PgDn`, mouse | Scroll the current view                        |
| `g` / `G`                 | Jump to top / bottom                           |
| `Space`                   | Pause / resume slide autoplay in Recap         |
| `r`                       | Re-read history and regenerate AI slides       |
| `q`                       | Quit application                               |

### Available Views
1. **Overview**: General statistics
2. **Tech Profile**: Technical expertise analysis
3. **Work Patterns**: Productivity patterns
4. **Tool Usage**: Developer tools usage
5. **Recap**: Your year in the terminal: top commands, peak hours, git story, stack, new tools, typos and persona
6. **Timeline**: Your most recent interesting commands

## Development

### Setup Development Environment
```bash
# Clone repository
git clone https://github.com/ksauraj/shellrecap.git
cd shellrecap

# Install dependencies
go mod download

# Run tests
make test

# Build and run
make run
```

## Troubleshooting

### Common Issues

1. **Permission Denied**
```bash
chmod +x shellrecap
```

2. **Binary Not Found**
```bash
export PATH=$PATH:$(pwd)
```

3. **API Key Issues**
Set the key at runtime, or build with it:
```bash
export GEMINI_API_KEY=your_api_key_here
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

Project Link: [https://github.com/ksauraj/shellrecap](https://github.com/ksauraj/shellrecap)
