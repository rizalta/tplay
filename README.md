# tplay

Stream torrents directly to your media player from the terminal.

## Features

- Stream from magnet links without downloading the full file
- Auto-selects single media files, file picker for multi-file torrents
- Supports **mpv**, **VLC**, and **IINA**
- Cache management with TTL cleanup and no-cache mode

## Install

```bash
go install github.com/rizalta/tplay@latest
```

Or build from source:

```bash
git clone https://github.com/rizalta/tplay
cd tplay
go install .
```

## Usage

```
tplay
```

Paste a magnet link and press `Enter`.

## Keybindings

| Key      | Action                   |
|----------|--------------------------|
| `↑ / ↓` | Navigate files           |
| `Enter`  | Select file              |
| `Tab`    | Toggle media / all files |
| `Esc`    | Back / cancel            |
| `Ctrl+S` | Open settings            |
| `Ctrl+C` | Quit                     |

## Configuration

Config is stored at `~/.config/tplay/config.json` and is created automatically on first run.

```json
{
  "player": "mpv",
  "read_ahead": 62,
  "cache": 0,
  "no_cache": false,
  "buffer_percent": 1
}
```

| Field            | Description                                            |
|------------------|--------------------------------------------------------|
| `player`         | Media player to use (`mpv`, `vlc`, `iina`)             |
| `read_ahead`     | Read-ahead buffer size in MB                           |
| `cache`          | Delete torrents older than N days (`0` = keep forever) |
| `no_cache`       | Delete all torrents from this session on exit          |
| `buffer_percent` | Percentage of file to buffer before playback starts    |

Settings can also be changed inside the app with `Ctrl+S`.

## Requirements

- Go 1.21+
- One of: [mpv](https://mpv.io), [VLC](https://www.videolan.org), [IINA](https://iina.io)
