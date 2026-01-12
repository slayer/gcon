# Terminal Line Wrapping Issues in Bubble Tea / Lipgloss

## Why This Problem Exists

The core issue is a **standards gap**:

1. **Unicode defines character properties** including "East Asian Width" (Narrow, Wide, Ambiguous, etc.)
2. **go-runewidth** (used by lipgloss) interprets these properties
3. **Terminals decide independently** how wide to render each character
4. **No enforcement mechanism** ensures they agree

Different terminals make different choices, especially for:
- Emoji (🟢 vs ●)
- Symbols in "Ambiguous" width category (☁, ☰, ▶)
- Characters with variation selectors

## Why Bubble Tea's Diffing Doesn't Help

Bubble Tea's renderer:
- Calculates diffs between frames
- Writes only changed portions to terminal
- Uses alternate screen buffer

But it **trusts lipgloss width calculations** when positioning content. If lipgloss says "this line is 108 chars", Bubble Tea believes it. The terminal then wraps the actual 110-char line, causing:

- Header pushed down (wrapped content takes extra row)
- Visual glitches as wrapped content overlaps
- Inconsistent rendering across terminals (VSCode's terminal handles emoji width differently than native terminals)

## More Universal Solutions

### 1. Right Margin Buffer (Simplest)

Never render to exact terminal width. Leave 2-3 char margin:

```go
safeWidth := terminalWidth - 3  // Always leave buffer
```

Pros: Simple, catches most issues
Cons: Wastes some screen space

---

### 2. Unicode Variation Selectors

Force "text presentation" (narrow) with VS15 (`U+FE0E`):

```go
func Cloud() string {
    return "☁\uFE0E"  // Forces text presentation
}
```

Pros: Standard Unicode mechanism
Cons: Not all terminals respect it, go-runewidth may still miscount

---

### 3. Curated Safe Symbol Set

Only use characters with consistent width across terminals:

```
Safe (1-wide everywhere):
  Box drawing: ─ │ ┌ ┐ └ ┘ ├ ┤ ┬ ┴ ┼
  Bullets: •
  ASCII: everything

Unsafe (width varies):
  Emoji: 🟢 📁 ☁️
  Geometric: ● ▶ ☰ ◆
```

Pros: Predictable
Cons: Limited visual options

---

### 4. Runtime Width Probing

Query terminal for actual character width at startup:

```go
func probeCharWidth(ch rune) int {
    // Save cursor position
    fmt.Print("\x1b[s")
    // Print character
    fmt.Print(string(ch))
    // Query cursor position via DECXCPR
    fmt.Print("\x1b[6n")
    // Parse response: ESC [ row ; col R
    // Restore cursor
    fmt.Print("\x1b[u")
    // Return detected width
}
```

Pros: Gets true terminal behavior
Cons: Not all terminals support cursor queries, adds startup latency, complex

---

### 5. Terminal Detection + Profiles

Detect terminal and apply known corrections:

```go
func getWidthCorrection() map[rune]int {
    term := os.Getenv("TERM_PROGRAM")
    switch term {
    case "iTerm.app":
        return itmermWidths
    case "Apple_Terminal":
        return macTermWidths
    case "vscode":
        return vscodeWidths  // Often different!
    }
    // Default conservative
    return defaultWidths
}
```

Pros: Can be very accurate
Cons: Maintenance burden, many terminal variants

---

### 6. Flexible Layout (No Fixed Widths)

Don't pad to exact width. Let content flow naturally:

```go
// Instead of:
lipgloss.Place(width, height, ...)

// Use:
lipgloss.NewStyle().MaxWidth(width).Render(content)
// Don't force padding to exact width
```

Pros: More resilient to width issues
Cons: Can't do precise right-alignment, less polished look

---

## Recommendation

For a TUI app, combine:

1. **Curated symbols** - Use only well-tested characters (e.g., `●` instead of `🟢`)

2. **Right margin buffer** - Don't render to exact terminal width:
   ```go
   const safetyMargin = 2
   renderWidth := terminalWidth - safetyMargin
   ```

3. **ASCII fallback flag** - Provide `--ascii` or `--no-emojis` flag for problematic terminals

4. **Optional: VS15 for ambiguous symbols**:
   ```go
   func Cloud() string {
       return "☁\uFE0E"  // Text presentation selector
   }
   ```

The fundamental issue is that **terminal width calculation is not standardized**, so any solution is either conservative (waste space) or requires runtime detection (complex). The safest universal approach is to use ASCII/box-drawing characters and leave a small right margin.

## Current Implementation

In this project, we implemented:

1. **Symbol counting** (`internal/ui/width.go`) - Counts emojis that terminals render wider than lipgloss measures
2. **SafeWidth calculation** - Reduces target width by emoji count to prevent overflow
3. **MaxWidth constraint** - Uses `lipgloss.Style.MaxWidth()` to truncate content that exceeds available space
4. **ASCII mode** - `--ascii` flag replaces all Unicode symbols with ASCII equivalents
