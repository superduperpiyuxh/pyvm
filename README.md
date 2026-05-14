# pyvm

A fast, zero-dependency Python version manager written in Go.

PyVM lets you install and switch between Python versions instantly. It uses pre-compiled binaries, so you don't need a system Python or a compiler to get started. It includes a clean CLI and a visual TUI for managing your environment.

## Features

- **No dependencies:** Works on a fresh OS without needing Python or `gcc`.
- **Pre-compiled:** Uses optimized binaries for fast installations.
- **TUI + CLI:** Manage versions visually or via standard commands.
- **Cross-platform:** Native support for Linux, macOS, and Windows.
- **Theming:** Customizable UI colors.

## Installation

### 1. Build from source
```bash
go build -o pyvm main.go
```

### 2. Configure your PATH
Add the shim directory to your shell config (e.g., `.zshrc` or `.bashrc`):

```bash
export PATH="$HOME/.pyvm/shim:$PATH"
```

## Usage

### TUI Mode
Run `pyvm` without arguments to open the interactive manager.

- `i` to install
- `u` to use/activate
- `d` to delete
- `Tab` to switch between Available, Installed, and Themes.

### CLI Commands
| Command | Action |
| :--- | :--- |
| `pyvm install 3.12` | Install a version |
| `pyvm use 3.12` | Switch active version |
| `pyvm ls` | List installed versions |
| `pyvm rm 3.12` | Uninstall a version |
| `pyvm which python` | Show path to the active binary |
| `pyvm color` | Change the UI theme |

## Project Structure
- `~/.pyvm/versions`: Where Python builds live.
- `~/.pyvm/shim`: Executables that route to your active version.
- `~/.pyvm/config.json`: Your preferences and theme settings.

## License
MIT

---
Created by [piyuxhh](https://github.com/piyuxhh)
