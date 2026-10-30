# tplay

Stream torrents directly to your media player from the terminal.

## Features

- Stream from magnet links without downloading the full file
- Auto-selects single media files, file picker for multi-file torrents
- Resume playback from where you left off
- Name-sorted file list with live filtering
- Powered by **mpv** with real-time IPC tracking
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

Press `/` to filter the file list; `Enter` applies it and `Esc` clears it. Torrents without media files show an empty list — press `Tab` to show all files.

## Keybindings

| Key      | Action                   |
|----------|--------------------------|
| `↑ / ↓` | Navigate files           |
| `Enter`  | Select file              |
| `/`      | Filter files             |
| `Tab`    | Toggle media / all files |
| `Esc`    | Back / cancel            |
| `Ctrl+S` | Open settings            |
| `Ctrl+C` | Quit                     |

## Configuration

Config is stored at `~/.config/tplay/config.json` and is created automatically on first run.

```json
{
  "read_ahead": 62,
  "cache": 0,
  "no_cache": false,
  "buffer_percent": 1,
  "resume_playback": true
}
```

| Field             | Description                                            |
|-------------------|--------------------------------------------------------|
| `read_ahead`      | Read-ahead buffer size in MB                           |
| `cache`           | Delete torrents older than N days (`0` = keep forever) |
| `no_cache`        | Delete all torrents from this session on exit          |
| `buffer_percent`  | Percentage of file to buffer before playback starts    |
| `resume_playback` | Resume video playback from last position               |

Settings can also be changed inside the app with `Ctrl+S`.

## Requirements

- Go 1.21+
- [mpv](https://mpv.io) (macOS and Linux)
