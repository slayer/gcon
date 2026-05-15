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

// Content MUST match sidebar's newline count.
// Use the shared renderLoading() helper from views/helpers.go:
//   renderLoading(v.spinner, "Loading instances...")
// The app-level layout handles height matching via lipgloss containers.
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

## EmojiWidthBudget Pattern: Views Behind renderWithSidebar

**Problem**: `renderWithSidebar` applies `MaxWidth(ContentWidth - totalMaxEmojis)` to prevent wide-emoji overflow. Views don't know about this reduction, so they measure their own width as `ContentWidth` and build content that's too wide.

**Root cause**: `lipgloss.Width()` undercounts certain Unicode symbols (▸, ▾, ▶, ●, ◆, etc.) by 1. Terminals render them 2-wide but lipgloss sees them as 1-wide. When a view uses ▸ as a row indicator, every row is effectively 1 char wider than measured.

**Solution**: Pre-compute the sidebar's emoji count and store it in `ProgramContext.EmojiWidthBudget`. Views subtract it in `SetContext()`:

```go
// app.go — syncContext() computes sidebar emoji budget
if a.sidebarActive() {
    a.ctx.EmojiWidthBudget = maxLineEmojiCount(a.sidebar.View())
} else {
    a.ctx.EmojiWidthBudget = 0
}

// views/logs.go — view subtracts budget + its own wide emoji
func (v *LogsView) SetContext(ctx *context.ProgramContext) {
    emojiCorrection := ctx.EmojiWidthBudget + 1 // +1 for own ▸ indicator
    v.width = ctx.ContentWidth - emojiCorrection
}
```

**Rule**: Any view that uses a wide-emoji character (▸, ▾, ▶, ●, etc.) as a row indicator or cursor must:
1. Add `+1` per wide emoji to `emojiCorrection` in `SetContext()`
2. Pass the corrected width to child components (logviewer, table, etc.)

**Why it stays in context**: The sidebar is stable and computed once per layout change. Counting emojis in the view would require passing sidebar state down — the context field is cleaner.

## Overlay + Wide Emoji Interaction

**Problem**: Opening an action menu (`.` key) causes line wrapping to break for a view that already accounts for wide emojis. Lines that were fitting cleanly start wrapping.

**Root cause**: The `overlay` package stitches the dialog over each line of the background view. When a dialog uses ▶ as a cursor indicator, the stitched line has TWO wide emojis (one from the view's row indicator + one from the dialog cursor). `renderWithSidebar` recounts emojis per line and finds 2, reducing `mainWidth` by 2 for ALL lines — 1 more than the view expected.

**Solution**: In the view's `View()` method, shrink the render width by 1 when the overlay is active:

```go
func (v *LogsView) View() string {
    renderWidth := v.width
    if v.menuOpen {
        // ▶ cursor in menu stitches onto lines with ▸, giving 2 wide emojis.
        // renderWithSidebar recounts and reduces mainWidth by 1 more than expected.
        renderWidth = v.width - 1
    }
    v.logViewer.SetSize(renderWidth, v.height-12)
    // ... render
}
```

**Rule**: If a view has both:
- Its own wide-emoji row indicator (▸, ●, etc.)
- An action menu overlay that uses a wide-emoji cursor (▶)

Then the view must subtract 1 from render width when the menu is open.

## Overlay Z-Order: Dialogs Must Render Above Parent Overlays

**Critical**: When a view has nested overlays (e.g., a detail overlay that spawns input/confirm dialogs via 'a'/'d' keys), the `View()` method must check higher-priority dialogs **before** the parent overlay. Otherwise the parent overlay renders first and returns early, hiding the dialog underneath.

The `Update()` key routing order (input → confirm → overlay) is typically correct, but `View()` rendering order must match:

```go
// Wrong — overlay hides dialogs spawned from within it
if v.showOverlay {
    return overlay.Center(main, v.renderOverlay(), w, h)  // returns here!
}
if v.showConfirm && v.confirmDialog != nil {
    return overlay.Center(main, v.confirmDialog.View(), w, h)  // never reached
}

// Correct — higher-priority dialogs render on top
if v.showInput && v.inputDialog != nil {
    return overlay.Center(main, v.inputDialog.View(), w, h)
}
if v.showConfirm && v.confirmDialog != nil {
    return overlay.Center(main, v.confirmDialog.View(), w, h)
}
if v.showOverlay {
    return overlay.Center(main, v.renderOverlay(), w, h)
}
```

**Symptom**: Keys work (state changes correctly, tests pass) but nothing visible happens on screen. If tests pass but runtime appears broken, check the `View()` rendering order.

**Debugging tip**: When state-level tests all pass but the UI doesn't respond, the bug is likely in `View()` (rendering), not `Update()` (state). Add a rendering test that checks `View()` output contains expected dialog content.

## Wrap Tall Tab Bodies in `bubbles/viewport.Model`

Details views with multiple tabs (Overview / Routing / Backends / Observability) often have one tab whose content exceeds the terminal height — e.g. an observability tab with 5+ stacked charts. Without a viewport, anything past the bottom edge of the terminal is clipped and unreachable; users see only the top charts.

The fix is to render the tab body into a string, hand it to a `viewport.Model`, and route scroll keys to it:

```go
type DetailsView struct {
    viewport     viewport.Model
    viewportSize bool // set once SetSize fires
    // ...
}

func (v *DetailsView) SetSize(width, height int) {
    v.width = width
    v.height = height
    if !v.viewportSize {
        v.viewport = viewport.New(width-4, height-4) // leave room for tab bar + status
        v.viewportSize = true
    } else {
        v.viewport.Width = width - 4
        v.viewport.Height = height - 4
    }
}

func (v *DetailsView) View() string {
    var b strings.Builder
    b.WriteString(v.tabs.View())
    b.WriteString("\n\n")

    var body string
    switch v.tabs.ActiveTab().ID {
    case "observability":
        body = v.renderObservability()
    // ...
    }
    if v.viewportSize {
        v.viewport.SetContent(body)
        b.WriteString(v.viewport.View())
    } else {
        b.WriteString(body)
    }
    return b.String()
}
```

**Scroll key routing** — pick which keys go to the viewport based on what the tab itself consumes:

```go
func isViewportScrollKey(m tea.KeyMsg) bool {
    switch m.String() {
    case "j", "k", "up", "down", "pgup", "pgdown", "home", "end":
        return true
    }
    return false
}

// In Update() — after tab-specific handlers, before delegating to tabs:
if isViewportScrollKey(m) {
    var cmd tea.Cmd
    v.viewport, cmd = v.viewport.Update(m)
    return cmd
}
```

If a specific tab (e.g. Backends) consumes `j/k` for its own focus navigation, route those keys to the tab handler first and fall through to the viewport only when the tab doesn't handle them. `PgUp/PgDn/Home/End` are usually safe everywhere.

**Reset scroll on tab change** so each tab opens at the top:

```go
case tabs.TabChangedMsg:
    v.viewport.GotoTop()
    // ...
```

**Symptom**: User reports "I cannot scroll page — some charts are not visible." Tab renders a tall body that ends up taller than the terminal frame.

**Rule**: any tab whose body can plausibly exceed terminal height (multi-chart observability, long forms, long log lists) needs a viewport. List views with their own scroll (tables, log viewer) are exempt — they already manage scrolling internally.
