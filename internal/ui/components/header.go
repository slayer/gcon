package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/ui/symbols"
)

// GoogleColors holds styles for rainbow-colored "Google" text
type GoogleColors struct {
	G  lipgloss.Style // Blue #4285F4
	O1 lipgloss.Style // Red #EA4335
	O2 lipgloss.Style // Yellow #FBBC05
	G2 lipgloss.Style // Blue #4285F4
	L  lipgloss.Style // Green #34A853
	E  lipgloss.Style // Red #EA4335
}

// HeaderStyles defines colors for header rendering
type HeaderStyles struct {
	Title               lipgloss.Style
	Muted               lipgloss.Style
	GoogleColors        GoogleColors
	BreadcrumbProject   lipgloss.Style
	BreadcrumbCategory  lipgloss.Style
	BreadcrumbResource  lipgloss.Style
	BreadcrumbSeparator lipgloss.Style
}

// DefaultHeaderStyles returns the default header styles with GCP colors
func DefaultHeaderStyles() HeaderStyles {
	return HeaderStyles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#4285F4")), // Google Blue

		Muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9AA0A6")), // Gray

		GoogleColors: GoogleColors{
			G:  lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4")).Bold(true),
			O1: lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335")).Bold(true),
			O2: lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC05")).Bold(true),
			G2: lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4")).Bold(true),
			L:  lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853")).Bold(true),
			E:  lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335")).Bold(true),
		},

		BreadcrumbProject: lipgloss.NewStyle().
			Background(lipgloss.Color("#4285F4")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1),

		BreadcrumbCategory: lipgloss.NewStyle().
			Background(lipgloss.Color("#34A853")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1),

		BreadcrumbResource: lipgloss.NewStyle().
			Background(lipgloss.Color("#FBBC05")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1),

		BreadcrumbSeparator: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9AA0A6")),
	}
}

// Header displays the app title with rainbow Google and powerline breadcrumbs
type Header struct {
	width  int
	styles HeaderStyles

	// Breadcrumb segments
	projectID string
	category  string
	resources []string
}

// NewHeader creates a new header component
func NewHeader() *Header {
	return &Header{
		styles: DefaultHeaderStyles(),
	}
}

// SetSize sets the header width
func (h *Header) SetSize(width int) {
	h.width = width
}

// SetProject sets the current project ID for breadcrumbs
func (h *Header) SetProject(projectID string) {
	h.projectID = projectID
}

// SetCategory sets the current category (e.g., "Compute Engine") for breadcrumbs
func (h *Header) SetCategory(category string) {
	h.category = category
}

// SetResources sets the resource path for breadcrumbs (e.g., ["my-instance"])
func (h *Header) SetResources(resources []string) {
	h.resources = resources
}

// View renders the header
func (h *Header) View() string {
	if h.width == 0 {
		return ""
	}

	appName := h.renderAppName()
	breadcrumbs := h.renderBreadcrumbs()

	// If no breadcrumbs, just return app name
	if breadcrumbs == "" {
		return appName
	}

	// Place breadcrumbs immediately after app name with some spacing
	spacing := "  " // Two spaces between app name and breadcrumbs
	result := appName + spacing + breadcrumbs

	// Calculate actual terminal width and truncate if needed
	resultWidth := h.terminalWidth(result)
	if resultWidth > h.width {
		// Truncate to fit
		result = h.truncateToWidth(result, h.width)
	}

	return result
}

// renderAppName renders "gcon - Google Console Platform TUI" with rainbow Google
func (h *Header) renderAppName() string {
	cloud := symbols.Cloud()
	gcon := h.styles.Title.Render("gcon")
	dash := h.styles.Muted.Render(" - ")

	// Rainbow "Google"
	g := h.styles.GoogleColors.G.Render("G")
	o1 := h.styles.GoogleColors.O1.Render("o")
	o2 := h.styles.GoogleColors.O2.Render("o")
	g2 := h.styles.GoogleColors.G2.Render("g")
	l := h.styles.GoogleColors.L.Render("l")
	e := h.styles.GoogleColors.E.Render("e")
	google := g + o1 + o2 + g2 + l + e

	rest := h.styles.Title.Render(" Console Platform TUI")

	return cloud + " " + gcon + dash + google + rest
}

// renderBreadcrumbs renders breadcrumb trail with powerline separators
func (h *Header) renderBreadcrumbs() string {
	if h.projectID == "" {
		return ""
	}

	var parts []string
	var lastBg lipgloss.Color

	// Project segment (blue background)
	project := h.styles.BreadcrumbProject.Render(h.projectID)
	parts = append(parts, project)
	lastBg = lipgloss.Color("#4285F4") // Blue

	// Category segment (green background)
	if h.category != "" {
		categoryBg := lipgloss.Color("#34A853") // Green
		// Separator: foreground = previous bg, background = current bg
		sep := lipgloss.NewStyle().
			Foreground(lastBg).
			Background(categoryBg).
			Render(symbols.HeaderSepRight())
		parts = append(parts, sep)

		category := h.styles.BreadcrumbCategory.Render(h.category)
		parts = append(parts, category)
		lastBg = categoryBg
	}

	// Resource segments (yellow background)
	for _, resource := range h.resources {
		if resource == "" {
			continue
		}
		resourceBg := lipgloss.Color("#FBBC05") // Yellow
		// Separator: foreground = previous bg, background = current bg
		sep := lipgloss.NewStyle().
			Foreground(lastBg).
			Background(resourceBg).
			Render(symbols.HeaderSepRight())
		parts = append(parts, sep)

		res := h.styles.BreadcrumbResource.Render(resource)
		parts = append(parts, res)
		lastBg = resourceBg
	}

	return strings.Join(parts, "")
}

// terminalWidth calculates actual terminal width, accounting for wide Unicode symbols
func (h *Header) terminalWidth(s string) int {
	base := lipgloss.Width(s)
	extra := 0
	for _, r := range s {
		// Count wide symbols that render as 2-wide but lipgloss counts as 1-wide
		// Cloud symbol, powerline arrows (both thin and solid), etc.
		if r == '☁' || r == '\ue0b0' || r == '\ue0b1' || r == '\ue0b2' || r == '\ue0b3' {
			extra++
		}
	}
	return base + extra
}

// truncateToWidth truncates a string to fit within the given terminal width.
// It accounts for powerline symbols and ANSI escape sequences.
func (h *Header) truncateToWidth(s string, maxWidth int) string {
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
		if r == '☁' || r == '\ue0b0' || r == '\ue0b1' || r == '\ue0b2' || r == '\ue0b3' {
			runeWidth = 2 // Wide symbols
		}

		if currentWidth+runeWidth > maxWidth {
			// Add ellipsis if we have space
			if currentWidth+3 <= maxWidth {
				result.WriteString("...")
			}
			break
		}

		result.WriteRune(r)
		currentWidth += runeWidth
	}

	return result.String()
}

// Width returns the header width
func (h *Header) Width() int {
	return h.width
}
