# anylyrics

A tiny terminal tool that turns a pasted lyric file into plain text and copies
it to the Wayland clipboard.

Supported input formats are auto-detected:

| Format | Description |
|---|---|
| **LRC** | Classic line-synced `[mm:ss.xx]text` |
| **LRC+** | Word/syllable-timed format ([spec](https://gitlab.com/Lalaggi/semitone/-/raw/main/docs/LRCPLUS_SPEC_V3.md)) |
| **ELRC** | Extended LRC with inline `<mm:ss.xx>` word tags and optional `v2:` voice prefixes |
| **TTML** | Word-timed XML (e.g. Apple Music) |
| plain text | Passed through unchanged |

## Build & run

```sh
go build -o anylyrics .
./anylyrics
```

## Usage

1. Paste the full lyric file (or press `ctrl+v` to pull it from the clipboard).
2. Press `ctrl+enter` (or plain `enter`) to convert.
3. The plaintext lyrics are shown.

| Key / action | Effect |
|---|---|
| `enter` | copy to clipboard and quit |
| click | copy to clipboard (stays open) |
| `c` | copy to clipboard (stays open) |
| `↑` `↓` / wheel | scroll |
| `q` / `esc` | quit |

Clipboard access uses `wl-copy` / `wl-paste` (Wayland).

## Tests

```sh
go test ./...
```
