# Glow

Render markdown on the CLI, with _pizzazz_!

> [!NOTE]
> This is a fork of [Glow](https://github.com/charmbracelet/glow) by
> [Charm](https://charm.sh). It adds inline markdown editing, built with
> [Claude Code](https://claude.com/claude-code). All the credit for Glow
> itself goes to Charm. See [Editing (this fork)](#editing-this-fork).
>
> To install this fork, build it from source:
> `git clone -b inline-editing https://github.com/pannekoek141/glow && cd glow && go build`

<p align="center">
    <img src="https://stuff.charm.sh/glow/glow-banner-github.gif" alt="Glow Logo">
    <a href="https://github.com/charmbracelet/glow/releases"><img src="https://img.shields.io/github/release/charmbracelet/glow.svg" alt="Latest Release"></a>
    <a href="https://pkg.go.dev/github.com/charmbracelet/glow?tab=doc"><img src="https://godoc.org/github.com/golang/gddo?status.svg" alt="GoDoc"></a>
    <a href="https://github.com/charmbracelet/glow/actions"><img src="https://github.com/charmbracelet/glow/workflows/build/badge.svg" alt="Build Status"></a>
    <a href="https://goreportcard.com/report/github.com/charmbracelet/glow"><img src="https://goreportcard.com/badge/charmbracelet/glow" alt="Go ReportCard"></a>
</p>

<p align="center">
    <img src="https://github.com/user-attachments/assets/c2246366-f84b-4847-b431-32a61ca07b74" width="800" alt="Glow UI Demo">
</p>

## What is it?

Glow is a terminal based markdown reader designed from the ground up to bring
out the beauty—and power—of the CLI.

Use it to discover markdown files, read documentation directly on the command
line. Glow will find local markdown files in subdirectories or a local
Git repository.

## Installation

### Package Manager

```bash
# macOS or Linux
brew install glow
```

```bash
# macOS (with MacPorts)
sudo port install glow
```

```bash
# Arch Linux (btw)
pacman -S glow
```

```bash
# Void Linux
xbps-install -S glow
```

```bash
# Nix shell
nix-shell -p glow --command glow
```

```bash
# FreeBSD
pkg install glow
```

```bash
# Solus
eopkg install glow
```

```bash
# Windows (with Chocolatey, Scoop, or Winget)
choco install glow
scoop install glow
winget install charmbracelet.glow
```

```bash
# Android (with termux)
pkg install glow
```

```bash
# Ubuntu (Snapcraft)
sudo snap install glow
```

```bash
# Debian/Ubuntu
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://repo.charm.sh/apt/gpg.key | sudo gpg --dearmor -o /etc/apt/keyrings/charm.gpg
echo "deb [signed-by=/etc/apt/keyrings/charm.gpg] https://repo.charm.sh/apt/ * *" | sudo tee /etc/apt/sources.list.d/charm.list
sudo apt update && sudo apt install glow
```

```bash
# Fedora/RHEL
echo '[charm]
name=Charm
baseurl=https://repo.charm.sh/yum/
enabled=1
gpgcheck=1
gpgkey=https://repo.charm.sh/yum/gpg.key' | sudo tee /etc/yum.repos.d/charm.repo
sudo yum install glow
```

Or download a binary from the [releases][releases] page. MacOS, Linux, Windows,
FreeBSD and OpenBSD binaries are available, as well as Debian, RPM, and Alpine
packages. ARM builds are also available for macOS, Linux, FreeBSD and OpenBSD.

### Go

Or just install it with `go`:

```bash
go install charm.land/glow/v3@latest
```

### Build (requires Go 1.21+)

```bash
git clone https://github.com/charmbracelet/glow.git
cd glow
go build
```

[releases]: https://github.com/charmbracelet/glow/releases

## The TUI

Simply run `glow` without arguments to start the textual user interface and
browse local. Glow will find local markdown files in the
current directory and below or, if you’re in a Git repository, Glow will search
the repo.

Markdown files can be read with Glow's high-performance pager. Most of the
keystrokes you know from `less` are the same, but you can press `?` to list
the hotkeys.

### Editing (this fork)

This fork adds Obsidian-style inline editing to the TUI.

- Press `e` in the pager or on the file list to edit. Every block stays
  rendered except the one under the cursor, which shows raw markdown.
  Changes autosave. Press `esc` to stop editing, or `E` to open `$EDITOR`.
- Double-click text to start editing right there. While editing, a click
  moves the cursor.
- Type `/` at the start of a line (or after a space) for a menu of markdown
  elements: headings, lists, to-dos, quotes, code blocks, tables, dividers,
  bold, italic, links and more.
- Click a to-do's `[ ]` to check or uncheck it.
- While viewing, drag with the mouse to select text. It's copied to the
  clipboard when you let go.
- Undo and redo with `cmd+z` / `cmd+shift+z` (or `ctrl+z` / `ctrl+y`).
  In Ghostty, add `keybind = super+z=unbind` and
  `keybind = super+shift+z=unbind` to your config so the keys reach Glow.
- Arrow keys at a block's edge move to the next block. Backspace at the start
  of a block joins it with the one above.

On the file list, press `n` for a new file (folders are created for you) and
`backspace` to move the selected file to the Trash.

Also new: tables get a full box border, and `style: auto` follows the
terminal's light/dark mode live.

## The CLI

In addition to a TUI, Glow has a CLI for working with Markdown. To format a
document use a markdown source as the primary argument:

```bash
# Read from file
glow README.md

# Read from stdin
echo "[Glow](https://github.com/charmbracelet/glow)" | glow -

# Fetch README from GitHub / GitLab
glow github.com/charmbracelet/glow

# Fetch markdown from HTTP
glow https://host.tld/file.md
```

### Word Wrapping

The `-w` flag lets you set a maximum width at which the output will be wrapped:

```bash
glow -w 60
```

### Paging

CLI output can be displayed in your preferred pager with the `-p` flag. This defaults
to the ANSI-aware `less -r` if `$PAGER` is not explicitly set.

### Styles

You can choose a style with the `-s` flag. When no flag is provided `glow` tries
to detect your terminal's current background color and automatically picks
either the `dark` or the `light` style for you.

```bash
glow -s [dark|light]
```

Alternatively you can also supply a custom JSON stylesheet:

```bash
glow -s mystyle.json
```

For additional usage details see:

```bash
glow --help
```

Check out the [Glamour Style Section](https://github.com/charmbracelet/glamour/blob/master/styles/gallery/README.md)
to find more styles. Or [make your own](https://github.com/charmbracelet/glamour/tree/master/styles)!

## The Config File

If you find yourself supplying the same flags to `glow` all the time, it's
probably a good idea to create a config file. Run `glow config`, which will open
it in your favorite $EDITOR. Alternatively you can manually put a file named
`glow.yml` in the default config path of you platform. If you're not sure where
that is, please refer to `glow --help`.

Here's an example config:

```yaml
# style name or JSON path (default "auto")
style: "light"
# mouse wheel support (TUI-mode only)
mouse: true
# use pager to display markdown
pager: true
# at which column should we word wrap?
width: 80
# show all files, including hidden and ignored.
all: false
# show line numbers (TUI-mode only)
showLineNumbers: false
# preserve newlines in the output
preserveNewLines: false
```

## Contributing

See [contributing][contribute].

[contribute]: https://github.com/charmbracelet/glow/contribute

## Feedback

We’d love to hear your thoughts on this project. Feel free to drop us a note!

- [Twitter](https://twitter.com/charmcli)
- [The Fediverse](https://mastodon.social/@charmcli)
- [Discord](https://charm.sh/chat)

## License

[MIT](https://github.com/charmbracelet/glow/raw/master/LICENSE)

---

Part of [Charm](https://charm.sh).

<a href="https://charm.sh/"><img alt="The Charm logo" src="https://stuff.charm.sh/charm-badge.jpg" width="400"></a>

Charm热爱开源 • Charm loves open source
