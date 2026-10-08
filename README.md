# shellrecap

Your year in the terminal. shellrecap is an interactive TUI that reads your shell history and turns
it into insights about how you work, plus a Spotify-Wrapped style recap of your year.

![shellrecap preview: the splash screen, the dashboard tabs and the Recap slides](assets/preview.gif)

## Table of Contents
- [Features](#features)
- [Installation](#installation)
  - [Pre-built Binaries](#pre-built-binaries)
  - [Quick Install Script](#quick-install-script)
  - [Go Install](#go-install)
  - [Manual Installation](#manual-installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [Themes](#themes)
- [Share Your Recap](#share-your-recap)
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
- Themes that match your terminal, whether it's dark, pitch black, light or Ubuntu purple
- Share your recap as animated GIFs and pictures in one keypress

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

Downloads the right binary for your system into the current directory and starts it.

```bash
curl -fsSL https://raw.githubusercontent.com/ksauraj/shellrecap/master/setup.sh | bash
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
- A Gemini and/or Groq API key (optional, for the AI-written slides)

```bash
# Clone repository
git clone https://github.com/ksauraj/shellrecap.git
cd shellrecap

# Build, optionally compiling in API keys
make build
make build GEMINI_API_KEY=your_gemini_key GROQ_API_KEY=your_groq_key

# Or using go build directly
go build -ldflags "-X github.com/ksauraj/shellrecap/internal/ai.geminiAPIKey=YOUR_GEMINI_KEY \
  -X github.com/ksauraj/shellrecap/internal/ai.groqAPIKey=YOUR_GROQ_KEY" ./cmd/shellrecap
```

### AI slides (optional)

The Recap view is computed locally from your history. With an API key it also gets a few
AI-written slides (persona, roast, superpower and forecast). Only aggregate stats such as program
names and counts are sent, never full command lines.

shellrecap asks **Gemini** first and falls back to **Groq** if Gemini fails or has no key. Each
provider has a backup model for when its preferred one is overloaded. Both work on their free tiers.

| Provider | Models, in order                                | Free tier limits (per account)                         |
|----------|-------------------------------------------------|--------------------------------------------------------|
| Gemini   | `gemini-3.8-flash`, then `gemini-3.5-flash-lite` | See [Gemini API rate limits](https://ai.google.dev/gemini-api/docs/rate-limits) |
| Groq     | `qwen/qwen3.8-27b`, then `openai/gpt-oss-120b`   | 30 requests/min, 1,000 requests/day, 8K tokens/min, 200K tokens/day |

| Variable               | Description                                                      |
|------------------------|------------------------------------------------------------------|
| `GEMINI_API_KEY`       | Gemini API key, used when none was compiled in with `-ldflags`   |
| `GROQ_API_KEY`         | Groq API key, used when none was compiled in with `-ldflags`     |
| `GEMINI_MODEL`         | Gemini model to use when `--model` isn't given                   |
| `GROQ_MODEL`           | Groq model to use when `--model` isn't given                     |
| `SHELLRECAP_CACHE_DIR` | Where AI slides are cached, defaults to `~/.cache/shellrecap` on Linux |
| `SHELLRECAP_SHARE_DIR` | Where share images are saved, defaults to `~/Pictures/shellrecap` |
| `SHELLRECAP_CONFIG_DIR` | Where your theme and share choices are saved, defaults to `~/.config/shellrecap` on Linux |
| `SHELLRECAP_THEME`     | Color theme, like `--theme`                                      |
| `SHELLRECAP_BACKGROUND` | Your terminal's background color, like `#300a24`, for the `auto` theme |

AI slides are cached per year and per model, so the AI isn't called on every launch. Cached
slides are reused as long as your stats are unchanged, or for up to a week while your command
count stays within 10% of when they were generated. Press `r` to re-read your history and
regenerate them. Each AI slide says which model wrote it and how long it took.

## Usage

### Basic Usage
```bash
shellrecap
```

### Command Line Options
| Flag                  | Description                                                        |
|-----------------------|--------------------------------------------------------------------|
| `--provider <name>`   | `auto` (default: Gemini, falling back to Groq), `gemini` or `groq` |
| `--model <model>`     | Model to use with `--provider`, e.g. `openai/gpt-oss-20b`          |
| `--no-cache`          | Always ask the AI for fresh slides                                 |
| `--theme <name>`      | Color theme: `auto`, `dark`, `black`, `light` or `terminal`        |
| `--version`           | Print the version                                                  |

```bash
# Compare providers and models
shellrecap --provider groq --no-cache
shellrecap --provider groq --model openai/gpt-oss-20b --no-cache
shellrecap --provider gemini --model gemini-3.5-flash-lite --no-cache
```

### Navigation Keys
| Key                       | Action                                         |
|---------------------------|------------------------------------------------|
| `Tab` / `Shift+Tab`       | Next / previous view                           |
| `1`-`5`                   | Jump to a view                                 |
| `←/→`                     | Change slides in Recap, switch views elsewhere |
| `↑/↓`, `PgUp/PgDn`, mouse | Scroll the current view                        |
| `g` / `G`                 | Jump to top / bottom                           |
| `Space`                   | Pause / resume slide autoplay in Recap         |
| `s`                       | Share your recap                               |
| `t`                       | Switch to the next color theme                 |
| `r`                       | Re-read history and regenerate AI slides       |
| `q`                       | Quit application                               |

### Available Views
1. **Overview**: General statistics
2. **Tech Profile**: Technical expertise analysis
3. **Work Patterns**: Productivity patterns
4. **Tool Usage**: Developer tools usage
5. **Recap**: Your year in the terminal: top commands, peak hours, git story, stack, new tools, typos and persona

## Themes

shellrecap picks its colors to suit your terminal. Press `t` to try the next theme; your choice is
remembered.

| Theme      | What it looks like                                                                  |
|------------|-------------------------------------------------------------------------------------|
| `auto`     | The default. Reads your terminal's background color and blends every grey from it, so it fits any color scheme, like Ubuntu's purple. Accents are adjusted until they're easy to read. |
| `dark`     | Muted accents on neutral greys                                                      |
| `black`    | A little more contrast, for pitch-black terminals and OLED screens                  |
| `light`    | Deeper accents on pale greys, for light terminals                                   |
| `terminal` | Your terminal's own 16 colors. Used by `auto` when the terminal can't show more     |

Most terminals report their background color. If yours doesn't and `auto` looks off, tell it the
color, or pick a theme:

```bash
SHELLRECAP_BACKGROUND='#300a24' shellrecap   # the auto theme, for this background
shellrecap --theme light                     # or SHELLRECAP_THEME=light
```

## Share Your Recap

Press `s` in the app, or run `shellrecap share`, and your recap is saved as GIFs and pictures:

| File                                   | What it is                                           | Best for                     |
|----------------------------------------|------------------------------------------------------|------------------------------|
| `shellrecap-2026-recap.gif`            | Every Recap slide, animated, under 1 MB              | X, Discord, Reddit, Slack    |
| `shellrecap-2026-tour.gif`             | Every tab, then the Recap slides, animated           | X, Discord, Reddit, Slack    |
| `shellrecap-2026-poster.png`           | A one-image summary poster                           | Instagram, LinkedIn, Bluesky |
| `shellrecap-2026-overview.png` and the other tabs | A picture of each tab                     | Posts with several pictures  |
| `shellrecap-2026-slide-01.png` and on  | A picture of each Recap slide (off by default)       | Carousels                    |

They're saved to `~/Pictures/shellrecap` and the poster is copied to your clipboard. Pick X,
Bluesky or LinkedIn in the share menu to open a new post with a caption ready, then paste or drag
in the image. Nothing is uploaded until you post it yourself. The images show program names,
subcommands like `git commit` and counts, never full command lines or your alias definitions.

Press `c` in the share menu to customize what's created:

| Option        | Choices                                                                         |
|---------------|---------------------------------------------------------------------------------|
| Images        | Any of the Recap GIF, tour GIF, poster, tab pictures and slide pictures         |
| AI slides     | Include the AI-written slides or leave them out                                 |
| GIF size      | `standard` (1080x1080 Recap, 1280x960 tour) or `small` (720x720, 960x720)       |
| Picture shape | `portrait` (1080x1350), `story` (1080x1920), `square` (1080x1080) or `landscape` (1920x1080) |
| Image theme   | `dark`, `black` (pitch black) or `light`                                        |

Your choices are saved in `~/.config/shellrecap/config.json` on Linux and used by
`shellrecap share` too. Set `SHELLRECAP_CONFIG_DIR` to keep them somewhere else.

On Linux, copying to the clipboard needs `wl-copy` (Wayland) or `xclip` (X11). Without them, or
over SSH, the images are still saved.

```bash
shellrecap share                          # save the images and print links to post them
shellrecap share --out ~/Desktop          # save them somewhere else
shellrecap share --only recap-gif,poster  # just these: recap-gif, tour-gif, poster, tabs, slides
shellrecap share --shape story --theme light --gif-size small
shellrecap share --no-ai                  # leave out the AI-written slides
```

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
Set the keys at runtime, or build with them:
```bash
export GEMINI_API_KEY=your_gemini_key GROQ_API_KEY=your_groq_key
make build GEMINI_API_KEY=your_gemini_key GROQ_API_KEY=your_groq_key
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
