# goread

A lightweight, keyboard-driven RSS/Atom newsreader for the terminal, built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

Feeds are fetched in parallel on startup. Read state and your feed list persist between sessions.

---

## Installation

```bash
git clone <your-repo>
cd goread
go build ./...
```

Requires Go 1.21 or later.

---

## Usage

```bash
./goread
```

On first launch the feed list will be empty. Press `a` to add your first RSS or Atom feed URL.

---

## Keyboard Controls

### Feed List

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate feeds |
| `Enter` | Open feed and view its articles |
| `a` | Add a new feed (enter URL, then `Enter`) |
| `d` / `x` | Delete the selected feed |
| `r` | Refresh all feeds |
| `q` | Quit |

### Article List

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate articles |
| `Enter` | Read the selected article |
| `o` | Open article in the system browser |
| `r` | Refresh this feed |
| `Esc` / `q` | Back to feed list |

### Article View

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll line by line |
| `PgUp` / `PgDn` | Scroll page by page |
| `o` | Open article in the system browser |
| `Esc` / `q` | Back to article list |

### Add Feed

| Key | Action |
|-----|--------|
| `Enter` | Confirm and fetch the feed |
| `Esc` | Cancel |

---

## How Non-Text Content Is Handled

RSS and Atom feeds are XML documents but often contain non-text elements:

| Content Type | How goread handles it |
|---|---|
| **HTML markup** | Stripped to plain text; block tags (`<p>`, `<br>`, `<div>`, etc.) become newlines |
| **HTML entities** | Unescaped (`&amp;` → `&`, `&nbsp;` → space, etc.) |
| **Images** | Silently skipped; press `o` to open the full article in your browser |
| **Audio / Video** | Enclosures are ignored; open in browser to access media |
| **Atom feeds** | Parsed identically to RSS via [gofeed](https://github.com/mmcdole/gofeed) |

---

## Data Storage

Feed URLs, cached titles, and per-article read state are stored as JSON at:

| OS | Path |
|---|---|
| Windows | `%APPDATA%\goread\data.json` |
| macOS | `~/Library/Application Support/goread/data.json` |
| Linux | `~/.config/goread/data.json` |

---

## Program Logic

```mermaid
flowchart TD
    A([Launch]) --> B[Load data.json\nfeed URLs + read state]
    B --> C{Feeds saved?}
    C -- No --> D[Feed List\nempty]
    C -- Yes --> E[Fetch all feeds\nin parallel]
    E --> F[Feed List\nwith unread counts]
    D --> F

    F --> G{Keypress}

    G -- a --> H[Add Feed view\ntext input]
    H -- Enter --> I[Append URL\nsave data.json]
    I --> J[Fetch new feed]
    J --> F
    H -- Esc --> F

    G -- d / x --> K[Remove feed\nfrom list + data.json]
    K --> F

    G -- r --> E

    G -- Enter --> L[Article List\nfor selected feed]
    L --> M{Keypress}

    M -- r --> N[Re-fetch\nthis feed]
    N --> L

    M -- o --> O[Open URL\nin system browser]
    O --> L

    M -- Enter --> P[Article View\nstripped + wrapped text]
    P --> Q[Mark article read\nsave data.json]
    Q --> R{Keypress}

    R -- o --> S[Open URL\nin system browser]
    S --> P

    R -- Esc / q --> L
    M -- Esc / q --> F
    G -- q --> T([Quit])
```

---

## Dependencies

| Package | Purpose |
|---|---|
| [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) | TUI framework (Elm-architecture event loop) |
| [charmbracelet/bubbles](https://github.com/charmbracelet/bubbles) | List, viewport, and text input components |
| [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) | Terminal styling and colors |
| [mmcdole/gofeed](https://github.com/mmcdole/gofeed) | RSS and Atom feed parsing |
