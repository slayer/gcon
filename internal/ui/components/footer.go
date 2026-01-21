package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/ui/mouse"
)

// FooterClickMsg represents a click on a footer region
type FooterClickMsg struct {
	RegionID string
}

// Powerline separator characters (Unicode code points for powerline fonts)
const (
	SepRight          = "\ue0b0" // Powerline right arrow (solid)
	SepRightThin      = "\ue0b1" // Powerline right arrow (thin)
	SepLeft           = "\ue0b2" // Powerline left arrow (solid)
	SepLeftThin       = "\ue0b3" // Powerline left arrow (thin)
	SepDiagonalRight  = "\ue0bc" // Powerline diagonal right (bottom-left to top-right)
	SepDiagonalLeft   = "\ue0ba" // Powerline diagonal left (top-left to bottom-right)
	SepDiagonalRightR = "\ue0be" // Powerline diagonal right reverse
	SepDiagonalLeftR  = "\ue0b8" // Powerline diagonal left reverse
)

// FooterStyles defines colors for each section slot
type FooterStyles struct {
	// Left group styles
	Left1Bg lipgloss.Color
	Left1Fg lipgloss.Color
	Left2Bg lipgloss.Color
	Left2Fg lipgloss.Color
	Left3Bg lipgloss.Color
	Left3Fg lipgloss.Color

	// Center group styles
	CenterBg lipgloss.Color
	CenterFg lipgloss.Color

	// Right group styles
	Right1Bg lipgloss.Color
	Right1Fg lipgloss.Color
	Right2Bg lipgloss.Color
	Right2Fg lipgloss.Color
	Right3Bg lipgloss.Color
	Right3Fg lipgloss.Color

	// Background for spacing between groups
	SpacerBg lipgloss.Color
}

// DefaultFooterStyles returns GCP-inspired footer colors
func DefaultFooterStyles() FooterStyles {
	return FooterStyles{
		// Left group - blue tones for navigation
		Left1Bg: lipgloss.Color("#4285F4"), // Google Blue
		Left1Fg: lipgloss.Color("#FFFFFF"),
		Left2Bg: lipgloss.Color("#5A95F5"), // Lighter blue
		Left2Fg: lipgloss.Color("#FFFFFF"),
		Left3Bg: lipgloss.Color("#303134"), // Dark bg for help text
		Left3Fg: lipgloss.Color("#9AA0A6"), // Muted text

		// Center group - neutral
		CenterBg: lipgloss.Color("#303134"),
		CenterFg: lipgloss.Color("#FFFFFF"),

		// Right group - status colors
		Right1Bg: lipgloss.Color("#303134"), // Dark for project info
		Right1Fg: lipgloss.Color("#9AA0A6"),
		Right2Bg: lipgloss.Color("#303134"), // Will be overridden by task status
		Right2Fg: lipgloss.Color("#FFFFFF"),
		Right3Bg: lipgloss.Color("#303134"),
		Right3Fg: lipgloss.Color("#FFFFFF"),

		// Spacer
		SpacerBg: lipgloss.Color("#202124"),
	}
}

// Footer displays a multi-section status bar with powerline separators
type Footer struct {
	width  int
	styles FooterStyles

	// Fixed slots - nil means slot is hidden, empty string shows just separator
	Left1   *string
	Left2   *string
	Left3   *string
	Center1 *string
	Center2 *string
	Center3 *string
	Right1  *string
	Right2  *string
	Right3  *string

	// Pre-rendered sections (already styled, for custom styling)
	CenterRendered   string
	CenterRenderedBg lipgloss.Color // Background color for separator styling
	Right1Rendered   string
	Right1RenderedBg lipgloss.Color // Background color for separator styling
	Right2Rendered   string
	Right2RenderedBg lipgloss.Color
	Right3Rendered   string
	Right3RenderedBg lipgloss.Color

	regionManager *mouse.RegionManager
}

// NewFooter creates a new footer with default styles
func NewFooter() *Footer {
	return &Footer{
		styles:        DefaultFooterStyles(),
		regionManager: mouse.NewRegionManager(),
	}
}

// SetWidth sets the footer width
func (f *Footer) SetWidth(width int) {
	f.width = width
}

// SetStyles sets custom footer styles
func (f *Footer) SetStyles(styles FooterStyles) {
	f.styles = styles
}

// Helper to set a slot value
func strPtr(s string) *string {
	return &s
}

// SetLeft1 sets the first left slot
func (f *Footer) SetLeft1(s string) { f.Left1 = strPtr(s) }

// SetLeft2 sets the second left slot
func (f *Footer) SetLeft2(s string) { f.Left2 = strPtr(s) }

// SetLeft3 sets the third left slot
func (f *Footer) SetLeft3(s string) { f.Left3 = strPtr(s) }

// SetCenter1 sets the first center slot
func (f *Footer) SetCenter1(s string) { f.Center1 = strPtr(s) }

// SetCenter2 sets the second center slot
func (f *Footer) SetCenter2(s string) { f.Center2 = strPtr(s) }

// SetCenter3 sets the third center slot
func (f *Footer) SetCenter3(s string) { f.Center3 = strPtr(s) }

// SetRight1 sets the first right slot
func (f *Footer) SetRight1(s string) { f.Right1 = strPtr(s) }

// SetRight2 sets the second right slot
func (f *Footer) SetRight2(s string) { f.Right2 = strPtr(s) }

// SetRight3 sets the third right slot
func (f *Footer) SetRight3(s string) { f.Right3 = strPtr(s) }

// ClearLeft1 hides the first left slot
func (f *Footer) ClearLeft1() { f.Left1 = nil }

// ClearLeft2 hides the second left slot
func (f *Footer) ClearLeft2() { f.Left2 = nil }

// ClearLeft3 hides the third left slot
func (f *Footer) ClearLeft3() { f.Left3 = nil }

// ClearCenter clears all center slots
func (f *Footer) ClearCenter() {
	f.Center1 = nil
	f.Center2 = nil
	f.Center3 = nil
	f.CenterRendered = ""
	f.CenterRenderedBg = ""
}

// SetCenterStyled sets pre-rendered content for center (bypasses default styling)
// The bg color is used for powerline separator styling
func (f *Footer) SetCenterStyled(rendered string, bg lipgloss.Color) {
	f.CenterRendered = rendered
	f.CenterRenderedBg = bg
}

// ClearRight1 hides the first right slot
func (f *Footer) ClearRight1() { f.Right1 = nil }

// SetRight1Styled sets pre-rendered content for right1 (bypasses default styling)
// The bg color is used for powerline separator styling
func (f *Footer) SetRight1Styled(rendered string, bg lipgloss.Color) {
	f.Right1Rendered = rendered
	f.Right1RenderedBg = bg
}

// ClearRight1Styled clears the pre-rendered right1 content
func (f *Footer) ClearRight1Styled() {
	f.Right1Rendered = ""
	f.Right1RenderedBg = ""
}

// ClearRight2 hides the second right slot
func (f *Footer) ClearRight2() { f.Right2 = nil }

// ClearRight3 hides the third right slot
func (f *Footer) ClearRight3() { f.Right3 = nil }

// SetRight2Styled sets pre-rendered content for right2 (bypasses default styling)
// The bg color is used for powerline separator styling
func (f *Footer) SetRight2Styled(rendered string, bg lipgloss.Color) {
	f.Right2Rendered = rendered
	f.Right2RenderedBg = bg
}

// SetRight3Styled sets pre-rendered content for right3 (bypasses default styling)
// The bg color is used for powerline separator styling
func (f *Footer) SetRight3Styled(rendered string, bg lipgloss.Color) {
	f.Right3Rendered = rendered
	f.Right3RenderedBg = bg
}

// ClearRight2Styled clears the pre-rendered right2 content
func (f *Footer) ClearRight2Styled() {
	f.Right2Rendered = ""
	f.Right2RenderedBg = ""
}

// ClearRight3Styled clears the pre-rendered right3 content
func (f *Footer) ClearRight3Styled() {
	f.Right3Rendered = ""
	f.Right3RenderedBg = ""
}

// View renders the footer
func (f *Footer) View() string {
	if f.width == 0 {
		return ""
	}

	// Render each group
	leftGroup := f.renderLeftGroup()
	centerGroup := f.renderCenterGroup()
	rightGroup := f.renderRightGroup()

	// Calculate widths accounting for powerline symbols that render wider than lipgloss measures
	leftWidth := f.terminalWidth(leftGroup)
	centerWidth := f.terminalWidth(centerGroup)
	rightWidth := f.terminalWidth(rightGroup)

	usedWidth := leftWidth + centerWidth + rightWidth

	// If content exceeds width, drop center first, then truncate
	if usedWidth > f.width && centerWidth > 0 {
		centerGroup = ""
		centerWidth = 0
		usedWidth = leftWidth + rightWidth
	}

	spacingTotal := f.width - usedWidth

	// Distribute spacing: if center exists, space on both sides; otherwise all to right
	var leftSpacing, rightSpacing int
	if centerWidth > 0 {
		leftSpacing = spacingTotal / 2
		rightSpacing = spacingTotal - leftSpacing
	} else {
		leftSpacing = spacingTotal
		rightSpacing = 0
	}

	// Ensure non-negative (content exceeds width)
	if leftSpacing < 0 {
		leftSpacing = 0
	}
	if rightSpacing < 0 {
		rightSpacing = 0
	}

	// Build spacer strings with background color
	spacerStyle := lipgloss.NewStyle().Background(f.styles.SpacerBg)
	leftSpacer := spacerStyle.Render(strings.Repeat(" ", leftSpacing))
	rightSpacer := spacerStyle.Render(strings.Repeat(" ", rightSpacing))

	// Join everything
	var parts []string
	if leftWidth > 0 {
		parts = append(parts, leftGroup)
	}
	if leftSpacing > 0 {
		parts = append(parts, leftSpacer)
	}
	if centerWidth > 0 {
		parts = append(parts, centerGroup)
	}
	if rightSpacing > 0 {
		parts = append(parts, rightSpacer)
	}
	if rightWidth > 0 {
		parts = append(parts, rightGroup)
	}

	result := lipgloss.JoinHorizontal(lipgloss.Top, parts...)

	// Final safety: truncate to width if still exceeding
	// This can happen when left+right groups alone exceed the width
	if f.terminalWidth(result) > f.width {
		result = f.truncateToWidth(result, f.width)
	}

	return result
}

// UpdateRegions implements Clickable interface
func (f *Footer) UpdateRegions(offsetX, offsetY int) {
	f.regionManager.Clear()

	if f.width == 0 {
		return
	}

	// Re-calculate layouts to find positions
	leftGroup := f.renderLeftGroup()
	centerGroup := f.renderCenterGroup()
	rightGroup := f.renderRightGroup()

	leftWidth := f.terminalWidth(leftGroup)
	centerWidth := f.terminalWidth(centerGroup)
	rightWidth := f.terminalWidth(rightGroup)

	usedWidth := leftWidth + centerWidth + rightWidth

	// Same logic as View()
	hasCenter := true
	if usedWidth > f.width && centerWidth > 0 {
		hasCenter = false
		centerWidth = 0
		usedWidth = leftWidth + rightWidth
	}

	spacingTotal := f.width - usedWidth
	var leftSpacing int
	if hasCenter && centerWidth > 0 {
		leftSpacing = spacingTotal / 2
	} else {
		leftSpacing = spacingTotal
	}
	if leftSpacing < 0 {
		leftSpacing = 0
	}

	// Calculate Right Group Start X
	var rightGroupStartX int
	if hasCenter {
		rightSpacing := spacingTotal - leftSpacing
		if rightSpacing < 0 {
			rightSpacing = 0
		}
		rightGroupStartX = offsetX + leftWidth + leftSpacing + centerWidth + rightSpacing
	} else {
		rightGroupStartX = offsetX + leftWidth + leftSpacing
	}

	// If Right3 is present, it's the LAST item in Right Group.
	if f.Right3 != nil || f.Right3Rendered != "" {
		// Calculate width of Right3 block
		var content string

		if f.Right3Rendered != "" {
			content = f.Right3Rendered
		} else if f.Right3 != nil {
			style := lipgloss.NewStyle().
				Background(f.styles.Right3Bg).
				Foreground(f.styles.Right3Fg).
				Padding(0, 1)
			content = style.Render(*f.Right3)
		}

		r3Width := f.terminalWidth(content)

		// The separator is before r3StartX.
		// Just content width.

		rightGroupEndX := rightGroupStartX + rightWidth
		r3StartX := rightGroupEndX - r3Width

		// Register region
		f.regionManager.Add("right3", mouse.Rect{
			X:      r3StartX,
			Y:      offsetY,
			Width:  r3Width,
			Height: 1, // Footer is 1 line high
		}, nil)
	}
}

// GetRegions implements Clickable interface
func (f *Footer) GetRegions() []mouse.Region {
	return f.regionManager.GetRegions()
}

// HandleRegionClick implements Clickable interface
func (f *Footer) HandleRegionClick(regionID string) tea.Cmd {
	return func() tea.Msg {
		return FooterClickMsg{RegionID: regionID}
	}
}

// terminalWidth calculates actual terminal width, accounting for powerline symbols
// that are rendered as 2-wide but lipgloss counts as 1-wide
func (f *Footer) terminalWidth(s string) int {
	base := lipgloss.Width(s)
	extra := 0
	for _, r := range s {
		// Powerline symbols render as 2-wide in most terminals
		if r == '\ue0b0' || r == '\ue0b1' || r == '\ue0b2' || r == '\ue0b3' ||
			r == '\ue0bc' || r == '\ue0ba' || r == '\ue0be' || r == '\ue0b8' {
			extra++
		}
	}
	return base + extra
}

// truncateToWidth truncates a string to fit within the given terminal width.
// It accounts for powerline symbols and ANSI escape sequences.
func (f *Footer) truncateToWidth(s string, maxWidth int) string {
	currentWidth := 0
	var result strings.Builder
	inEscape := false

	for _, r := range s {
		// Track ANSI escape sequences (don't count them in width)
		if r == '\x1b' {
			inEscape = true
			result.WriteRune(r)
			continue
		}
		if inEscape {
			result.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}

		// Calculate width of this rune
		runeWidth := 1
		if r == '\ue0b0' || r == '\ue0b1' || r == '\ue0b2' || r == '\ue0b3' ||
			r == '\ue0bc' || r == '\ue0ba' || r == '\ue0be' || r == '\ue0b8' {
			runeWidth = 2 // Powerline symbols render as 2-wide
		}

		if currentWidth+runeWidth > maxWidth {
			break
		}

		result.WriteRune(r)
		currentWidth += runeWidth
	}

	return result.String()
}

// renderLeftGroup renders left1 | left2 | left3 with powerline separators
func (f *Footer) renderLeftGroup() string {
	var parts []string
	var lastBg lipgloss.Color

	// Left1
	if f.Left1 != nil {
		style := lipgloss.NewStyle().
			Background(f.styles.Left1Bg).
			Foreground(f.styles.Left1Fg).
			Padding(0, 1)
		parts = append(parts, style.Render(*f.Left1))
		lastBg = f.styles.Left1Bg
	}

	// Left2
	if f.Left2 != nil {
		// Add powerline separator from previous section
		if lastBg != "" {
			sep := lipgloss.NewStyle().
				Foreground(lastBg).
				Background(f.styles.Left2Bg).
				Render(SepRight)
			parts = append(parts, sep)
		}
		style := lipgloss.NewStyle().
			Background(f.styles.Left2Bg).
			Foreground(f.styles.Left2Fg).
			Padding(0, 1)
		parts = append(parts, style.Render(*f.Left2))
		lastBg = f.styles.Left2Bg
	}

	// Left3
	if f.Left3 != nil {
		// Add powerline separator from previous section
		if lastBg != "" {
			sep := lipgloss.NewStyle().
				Foreground(lastBg).
				Background(f.styles.Left3Bg).
				Render(SepRight)
			parts = append(parts, sep)
		}
		style := lipgloss.NewStyle().
			Background(f.styles.Left3Bg).
			Foreground(f.styles.Left3Fg).
			Padding(0, 1)
		parts = append(parts, style.Render(*f.Left3))
		lastBg = f.styles.Left3Bg
	}

	// Final separator to spacer
	if lastBg != "" {
		sep := lipgloss.NewStyle().
			Foreground(lastBg).
			Background(f.styles.SpacerBg).
			Render(SepRight)
		parts = append(parts, sep)
	}

	return strings.Join(parts, "")
}

// renderCenterGroup renders center1 | center2 | center3 or pre-rendered content
func (f *Footer) renderCenterGroup() string {
	var parts []string

	// Check if we have pre-rendered content
	if f.CenterRendered != "" {
		// Use custom background color for separators
		centerBg := f.styles.CenterBg
		if f.CenterRenderedBg != "" {
			centerBg = f.CenterRenderedBg
		}

		// Separator from spacer
		sep := lipgloss.NewStyle().
			Foreground(f.styles.SpacerBg).
			Background(centerBg).
			Render(SepRight)
		parts = append(parts, sep)

		// Add pre-rendered content
		parts = append(parts, f.CenterRendered)

		// Final separator to spacer
		sep = lipgloss.NewStyle().
			Foreground(centerBg).
			Background(f.styles.SpacerBg).
			Render(SepRight)
		parts = append(parts, sep)

		return strings.Join(parts, "")
	}

	// Otherwise render normal center slots
	hasParts := f.Center1 != nil || f.Center2 != nil || f.Center3 != nil
	if hasParts {
		sep := lipgloss.NewStyle().
			Foreground(f.styles.SpacerBg).
			Background(f.styles.CenterBg).
			Render(SepRight)
		parts = append(parts, sep)
	}

	style := lipgloss.NewStyle().
		Background(f.styles.CenterBg).
		Foreground(f.styles.CenterFg).
		Padding(0, 1)

	var centerParts []string
	if f.Center1 != nil {
		centerParts = append(centerParts, *f.Center1)
	}
	if f.Center2 != nil {
		centerParts = append(centerParts, *f.Center2)
	}
	if f.Center3 != nil {
		centerParts = append(centerParts, *f.Center3)
	}

	if len(centerParts) > 0 {
		// Join center parts with diagonal separators for visual interest
		content := strings.Join(centerParts, " "+SepDiagonalRight+" ")
		parts = append(parts, style.Render(content))

		// Final separator to spacer
		sep := lipgloss.NewStyle().
			Foreground(f.styles.CenterBg).
			Background(f.styles.SpacerBg).
			Render(SepRight)
		parts = append(parts, sep)
	}

	return strings.Join(parts, "")
}

// renderRightGroup renders right sections with powerline separators (right to left style)
func (f *Footer) renderRightGroup() string {
	var parts []string

	// Collect active right sections in reverse order for proper powerline direction
	type section struct {
		content  string
		bg       lipgloss.Color
		fg       lipgloss.Color
		rendered string // pre-rendered content (already styled)
	}

	var sections []section

	if f.Right1 != nil || f.Right1Rendered != "" {
		bg := f.styles.Right1Bg
		if f.Right1RenderedBg != "" {
			bg = f.Right1RenderedBg // Use custom bg for separator styling
		}
		s := section{bg: bg, fg: f.styles.Right1Fg, rendered: f.Right1Rendered}
		if f.Right1 != nil {
			s.content = *f.Right1
		}
		sections = append(sections, s)
	}
	if f.Right2 != nil || f.Right2Rendered != "" {
		bg := f.styles.Right2Bg
		if f.Right2RenderedBg != "" {
			bg = f.Right2RenderedBg
		}
		s := section{bg: bg, fg: f.styles.Right2Fg, rendered: f.Right2Rendered}
		if f.Right2 != nil {
			s.content = *f.Right2
		}
		sections = append(sections, s)
	}
	if f.Right3 != nil || f.Right3Rendered != "" {
		bg := f.styles.Right3Bg
		if f.Right3RenderedBg != "" {
			bg = f.Right3RenderedBg
		}
		s := section{bg: bg, fg: f.styles.Right3Fg, rendered: f.Right3Rendered}
		if f.Right3 != nil {
			s.content = *f.Right3
		}
		sections = append(sections, s)
	}

	if len(sections) == 0 {
		return ""
	}

	// First separator from spacer (using left-pointing arrow for right group)
	sep := lipgloss.NewStyle().
		Foreground(sections[0].bg).
		Background(f.styles.SpacerBg).
		Render(SepLeft)
	parts = append(parts, sep)

	for i, sec := range sections {
		// Render content
		if sec.rendered != "" {
			// Use pre-rendered content (for task status with custom colors)
			parts = append(parts, sec.rendered)
		} else {
			style := lipgloss.NewStyle().
				Background(sec.bg).
				Foreground(sec.fg).
				Padding(0, 1)
			parts = append(parts, style.Render(sec.content))
		}

		// Add separator to next section (if not last)
		if i < len(sections)-1 {
			nextBg := sections[i+1].bg
			sep := lipgloss.NewStyle().
				Foreground(nextBg).
				Background(sec.bg).
				Render(SepLeft)
			parts = append(parts, sep)
		}
	}

	return strings.Join(parts, "")
}
