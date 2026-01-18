# Fancy Header with Colorized App Name and Powerline Breadcrumbs

## Overview

Enhanced the gcon header with a visually appealing rainbow-colored "Google" branding and powerline-styled breadcrumb navigation. The header now displays:

1. **App Name**: "gcon - Google Console Platform TUI" with rainbow-colored "Google" (per-letter coloring: G=blue, o=red, o=yellow, g=blue, l=green, e=red)
2. **Breadcrumb Navigation**: Powerline-styled breadcrumbs positioned immediately after the app name on the left side
   - Colored backgrounds (Project=blue, Category=green, Resources=yellow)
   - Solid arrow separators (`\ue0b0`) with proper foreground/background styling for seamless flow
3. **Single-line Layout**: Compact design with breadcrumbs on the left after app name
4. **Responsive Width Handling**: Automatically adjusts to terminal width with proper truncation

## Implementation Details

### Architecture Changes

The implementation follows the footer component pattern, creating a self-contained header component with its own styling:

1. **Header Component** (`internal/ui/components/header.go`)
   - Self-contained component with HeaderStyles
   - Manages breadcrumb state (project, category, resources)
   - Handles width calculations for powerline symbols
   - Provides truncation for narrow terminals

2. **Symbol System** (`internal/ui/symbols/symbols.go`)
   - Added `HeaderSepRight` and `HeaderSepLeft` to SymbolSet
   - Powerline thin arrows (`\ue0b1`, `\ue0b3`) for all modes
   - ASCII fallback: `>` and `<`

3. **App Integration** (`internal/ui/app.go`, `internal/ui/app_render.go`)
   - Header component initialized in App struct
   - Width updates on window resize
   - Dynamic breadcrumb generation based on current view

### Key Features

#### Rainbow Google Branding

The word "Google" in the app name renders with per-letter coloring using Google's brand colors:

```go
GoogleColors: GoogleColors{
    G:  lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4")).Bold(true), // Blue
    O1: lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335")).Bold(true), // Red
    O2: lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC05")).Bold(true), // Yellow
    G2: lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4")).Bold(true), // Blue
    L:  lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853")).Bold(true), // Green
    E:  lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335")).Bold(true), // Red
}
```

#### Powerline Breadcrumbs

Breadcrumbs use colored backgrounds with powerline separators:

- **Project**: Blue background (#4285F4)
- **Category**: Green background (#34A853) (e.g., "Compute Engine", "Cloud Storage")
- **Resources**: Yellow background (#FBBC05) (e.g., instance names, bucket names)
- **Separators**: Solid powerline arrows (Unicode `\ue0b0`) for a bold, clear visual separation

Example breadcrumb trail:
```
[my-project-123] ❯ [Compute Engine] ❯ [my-instance]
    (blue)           (green)              (yellow)
```

#### Width Handling

The header implements sophisticated width handling to account for Unicode symbols that render wider than lipgloss measures:

```go
func (h *Header) terminalWidth(s string) int {
    base := lipgloss.Width(s)
    extra := 0
    for _, r := range s {
        // Cloud symbol, powerline arrows count as 2-wide
        if r == '☁' || r == '\ue0b1' || r == '\ue0b3' {
            extra++
        }
    }
    return base + extra
}
```

This prevents layout glitches where powerline symbols overflow the available width.

#### ASCII Mode Support

All powerline symbols have ASCII fallbacks for terminals without Unicode support:

- Thin right arrow `\ue0b1` → `>`
- Thin left arrow `\ue0b3` → `<`
- Cloud symbol `☁` → `#`

Enable ASCII mode with: `./gcon --ascii`

### Code Structure

```
internal/ui/components/header.go       # Main header component
internal/ui/components/header_test.go  # Comprehensive tests (21 test cases)
internal/ui/symbols/symbols.go         # Powerline symbol definitions
internal/ui/app.go                     # Header initialization
internal/ui/app_render.go              # Header rendering logic
```

### Testing

Comprehensive test coverage with 21 test cases covering:

- Component initialization and configuration
- Rainbow Google rendering
- Breadcrumb generation (no project, project only, full path, multiple resources)
- Width calculations for Unicode symbols
- Truncation behavior
- ASCII mode fallback
- Style initialization
- Edge cases (empty resources, narrow terminals)

All tests pass:
```bash
$ go test ./internal/ui/components -v -run TestHeader
PASS
ok  	github.com/slayer/gcon/internal/ui/components	0.430s
```

## Visual Examples

### Standard Mode (with powerline symbols)

Breadcrumbs appear immediately after the app name on the left side with proper spacing:

```
☁ gcon - Google Console Platform TUI  [my-project] ❯ [Compute Engine] ❯ [my-instance]
         (rainbow colors)                (blue)          (green)            (yellow)
```

The solid powerline separators (❯) have:
- **Foreground**: Color of the previous segment's background
- **Background**: Color of the next segment's background
- **Style**: Solid/fat arrows (`\ue0b0`) for bold visual impact

This creates a seamless "flow" effect between segments with clear, prominent separators.

### ASCII Mode

```
# gcon - Google Console Platform TUI  [my-project] > [Compute Engine] > [my-instance]
```

### Various Contexts

**Project Selection View:**
```
☁ gcon - Google Console Platform TUI
```

**Instances List:**
```
☁ gcon - Google Console Platform TUI  [my-project] ❯ [Compute Engine]
```

**Instance Details:**
```
☁ gcon - Google Console Platform TUI  [my-project] ❯ [Compute Engine] ❯ [my-instance]
```

**Cloud Storage Objects:**
```
☁ gcon - Google Console Platform TUI  [my-project] ❯ [Cloud Storage] ❯ [my-bucket] ❯ [folder1]
```

## Technical Considerations

### Import Cycle Resolution

Initially, the header component imported `internal/ui` for styles, creating an import cycle. This was resolved by:

1. Moving GoogleColors and breadcrumb styles into the header component
2. Creating `DefaultHeaderStyles()` function
3. Making the header component self-contained (similar to footer)

### Width Calculation Accuracy

Powerline symbols and certain Unicode characters (☁, ❯) render as 2 characters wide in most terminals but lipgloss reports them as 1-wide. The `terminalWidth()` helper function compensates for this discrepancy to prevent layout overflow.

### Breadcrumb State Management

The header doesn't maintain view state directly. Instead, `app_render.go` updates the header state before rendering based on:

- Current view type (ViewInstanceDetails, ViewBuckets, etc.)
- Selected resources (selectedInstance, selectedDisk, etc.)
- Sidebar category (if navigating via sidebar)

This keeps the header component stateless and reusable.

## Performance Impact

Minimal performance impact:
- Header renders once per frame
- Width calculations are O(n) where n is string length
- No additional API calls
- No async operations

## Backward Compatibility

Fully backward compatible:
- No changes to existing view interfaces
- ASCII mode preserved for older terminals
- All existing tests continue to pass

## Future Enhancements

Potential improvements for future iterations:

1. **Configurable Colors**: Allow users to customize breadcrumb colors via config file
2. **Breadcrumb Truncation**: Smart truncation for very long resource names
3. **Breadcrumb Navigation**: Click or keyboard shortcuts to navigate breadcrumb trail
4. **Hover States**: Visual feedback on breadcrumb segments
5. **Custom Separators**: Allow users to choose separator style

## Related Files

- Plan: `/Users/vlad/dev/my/gcon/doc/2026-01-18-fancy-header/plan.md`
- TODO: `/Users/vlad/dev/my/gcon/doc/2026-01-18-fancy-header/TODO.md`
- Implementation: `internal/ui/components/header.go`
- Tests: `internal/ui/components/header_test.go`

## References

- [Powerline Fonts](https://github.com/powerline/fonts)
- [Google Brand Colors](https://brandcolors.net/b/google)
- [Lip Gloss Documentation](https://github.com/charmbracelet/lipgloss)
- Footer Component Pattern: `internal/ui/components/footer.go`
