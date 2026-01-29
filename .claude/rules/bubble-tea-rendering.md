---
description: Lipgloss and Bubble Tea rendering guidelines
globs:
  - "internal/ui/**/*.go"
---

# Bubble Tea Rendering Guidelines

## lipgloss Height() and Newline Counting

**Critical**: `lipgloss.Height(n)` renders n lines, which equals **n-1 newlines**.

When using `lipgloss.JoinHorizontal()` (e.g., sidebar + content), both sides must have the **same newline count** or the layout breaks and causes visual glitches like headers disappearing.

```go
// Sidebar uses lipgloss Height(n) which outputs n-1 newlines
container := styles.Container.Width(width).Height(s.height)
return container.Render(content)  // outputs height-1 newlines

// Content MUST match sidebar's newline count
func (v *View) renderLoading(msg string) string {
    content := fmt.Sprintf("\n  %s %s\n", v.spinner.View(), msg)
    // Match sidebar: height-1 newlines (NOT height!)
    targetNewlines := v.height - 1
    if targetNewlines < 10 {
        targetNewlines = 10
    }
    currentNewlines := strings.Count(content, "\n")
    for i := currentNewlines; i < targetNewlines; i++ {
        content += "\n"
    }
    return content
}
```

## Testing Render Heights

Always add tests to verify sidebar and content output the same newline count:

```go
func TestRenderingHeightConsistency(t *testing.T) {
    // Setup app with sidebar active
    sidebarView := app.sidebar.View()
    contentView := app.renderCurrentView()

    sidebarNewlines := strings.Count(sidebarView, "\n")
    contentNewlines := strings.Count(contentView, "\n")

    assert.Equal(t, sidebarNewlines, contentNewlines,
        "Sidebar and content must have same newline count")
}
```

## Avoid tea.ClearScreen

**Never use `tea.ClearScreen`** - it clears the entire terminal including app header/chrome, not just the content area. Instead, ensure consistent rendering heights.

## Hide Duplicate UI Elements

When using bubbles/list component, hide its built-in title with `l.SetShowTitle(false)` if the app header already shows context. This prevents duplicate titles.

## Unicode Symbol Width Issues

**Critical**: `lipgloss.Width()` miscounts certain Unicode symbols as 1-wide when terminals render them as 2-wide. Affected symbols include: ☁, ☰, ▶, ▸, ◀, ●

**Solutions**:
1. Use `SafeWidth(terminalWidth, content)` helper to reduce width by emoji count per line
2. Prefer 1-char Unicode symbols with lipgloss colors over emoji circles:
   - Instead of 🟢 🔴 🟡 → use ● with `lipgloss.Color("#34A853")` etc.
3. Centralize symbols in `internal/ui/symbols` package with ASCII fallback support (`--ascii` flag)
4. **For click regions and width calculations**: Always use `lipgloss.Width()` instead of `len()` to handle multi-byte UTF-8 characters correctly

```go
// symbols/symbols.go - colored 1-char status indicators
var colorGreen = lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))

func StatusRunning() string {
    if asciiMode {
        return colorGreen.Render("[OK]")
    }
    return colorGreen.Render("●")  // 1-char wide, colored
}

// Width calculations for click regions
// Wrong - counts bytes, not visual width
width := len(text) + padding

// Correct - counts visual width, handles UTF-8
width := lipgloss.Width(text) + padding
```

## Component Width Reporting

When components have borders, the `Width()` method must include border width in the reported value:

```go
func (s *Sidebar) Width() int {
    // Add 1 for the right border that the container style adds
    if s.collapsed {
        return CollapsedWidth + 1
    }
    return ExpandedWidth + 1
}
```

Layout calculations depend on accurate width reporting. If a component reports width=26 but renders as 27 (due to border), content will overflow by 1 character.

## lipgloss Style Testing

`lipgloss.Style.String()` returns an empty string - it doesn't serialize style properties. To test that styles are properly initialized, render content instead:

```go
// Wrong - String() returns empty
assert.NotEmpty(t, style.String())

// Correct - test by rendering
rendered := style.Render("test")
assert.NotEmpty(t, rendered)
```

## Component Width Caching

For components that recalculate layouts (like tables with flexible columns), cache the last width to avoid recalculation on every render:

```go
type Model struct {
    lastWidth       int
    columnsComputed bool
}

func (m *Model) SetSize(width, height int) {
    // Only recalculate if width changed
    if width != m.lastWidth || !m.columnsComputed {
        m.adjustColumnWidths(width)
        m.lastWidth = width
        m.columnsComputed = true
    }
}
```

## Transparent Backgrounds with lipgloss

**Critical**: `UnsetBackground()` on lipgloss styles doesn't always work when the style is wrapped by a Container with `.Width()`, because the Container fills remaining space with its background.

**Solutions**:

1. Use `Inline(true)` to prevent background inheritance (preferred when you still want foreground styling):
```go
// Correct - Inline(true) prevents container background bleeding
helpStyle := lipgloss.NewStyle().Foreground(colorMuted).Faint(true)
b.WriteString(helpStyle.Inline(true).Render("Select a zone"))
return m.styles.Container.Width(m.width).Render(b.String())
```

2. Use plain strings for truly unstyled text:
```go
// Also correct - plain text, no lipgloss styling at all
b.WriteString("help text")
return m.styles.Container.Width(m.width).Render(b.String())
```

This applies to any text that should appear "inline" without a background box inside a styled container.

## Styling Bubbles TextInput Components

**Critical**: Never wrap `textInput.View()` output with styled boxes - it breaks cursor positioning and placeholder display.

**Solution**: Set background colors directly on the textinput's internal style properties:

```go
// Wrong - breaks cursor and display
boxStyle := lipgloss.NewStyle().Background(bgColor)
return boxStyle.Render(f.textInput.View())

// Correct - set styles on the component itself
f.textInput.TextStyle = lipgloss.NewStyle().
    Foreground(textColor).
    Background(bgColor)
f.textInput.PlaceholderStyle = lipgloss.NewStyle().
    Foreground(placeholderColor).
    Background(bgColor)
f.textInput.PromptStyle = lipgloss.NewStyle().
    Background(bgColor)
f.textInput.Cursor.TextStyle = lipgloss.NewStyle().
    Background(bgColor)
return f.textInput.View()
```

Update these styles in `Focus()` and `Blur()` methods to change appearance based on focus state.
