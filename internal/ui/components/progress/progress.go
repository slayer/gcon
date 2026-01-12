package progress

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
)

// Colors matching GCP theme
var (
	borderColor    = lipgloss.Color("#4285F4")
	progressColor  = lipgloss.Color("#34A853")
	textColor      = lipgloss.Color("#E8EAED")
	mutedTextColor = lipgloss.Color("#9AA0A6")
)

// Progress represents a progress bar display
type Progress struct {
	Title            string
	CurrentFile      string
	CurrentFileNum   int
	TotalFiles       int
	BytesTransferred int64
	TotalBytes       int64
	width            int
}

// New creates a new progress display
func New() *Progress {
	return &Progress{
		width: 50,
	}
}

// SetSize sets the width of the progress bar
func (p *Progress) SetSize(width int) {
	if width > 20 {
		p.width = width - 10 // Account for borders
	}
}

// SetProgress updates the progress state
func (p *Progress) SetProgress(title, currentFile string, currentNum, totalFiles int, transferred, total int64) {
	p.Title = title
	p.CurrentFile = currentFile
	p.CurrentFileNum = currentNum
	p.TotalFiles = totalFiles
	p.BytesTransferred = transferred
	p.TotalBytes = total
}

// View renders the progress bar
func (p *Progress) View() string {
	// Calculate percentage
	var percent float64
	if p.TotalBytes > 0 {
		percent = float64(p.BytesTransferred) / float64(p.TotalBytes) * 100
	}

	// Build the progress bar
	barWidth := p.width - 20 // Account for percentage and size text
	if barWidth < 10 {
		barWidth = 10
	}

	filled := int(float64(barWidth) * percent / 100)
	if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	// Styles
	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(textColor)

	fileStyle := lipgloss.NewStyle().
		Foreground(mutedTextColor)

	barStyle := lipgloss.NewStyle().
		Foreground(progressColor)

	percentStyle := lipgloss.NewStyle().
		Foreground(textColor).
		Bold(true)

	sizeStyle := lipgloss.NewStyle().
		Foreground(mutedTextColor)

	// Build content
	var lines []string

	// Title line
	if p.TotalFiles > 1 {
		lines = append(lines, titleStyle.Render(fmt.Sprintf("%s: %d of %d files", p.Title, p.CurrentFileNum, p.TotalFiles)))
	} else {
		lines = append(lines, titleStyle.Render(p.Title))
	}

	// Current file line
	if p.CurrentFile != "" {
		displayFile := p.CurrentFile
		maxLen := p.width - 15
		if len(displayFile) > maxLen {
			displayFile = "..." + displayFile[len(displayFile)-maxLen+3:]
		}
		lines = append(lines, fileStyle.Render(displayFile))
	}

	// Progress bar line
	progressLine := fmt.Sprintf("%s  %s  %s",
		barStyle.Render(bar),
		percentStyle.Render(fmt.Sprintf("%3.0f%%", percent)),
		sizeStyle.Render(fmt.Sprintf("%s/%s", gcp.FormatSize(p.BytesTransferred), gcp.FormatSize(p.TotalBytes))),
	)
	lines = append(lines, progressLine)

	content := strings.Join(lines, "\n")
	return borderStyle.Render(content)
}

// ProgressUpdate is a message sent to update progress
type ProgressUpdate struct {
	BytesTransferred int64
	TotalBytes       int64
	CurrentFile      string
	CurrentFileNum   int
	TotalFiles       int
}

// ProgressDone is sent when the operation completes
type ProgressDone struct {
	Error error
}
