package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/config"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/symbols"
)

// syncFooter updates all footer slots based on current application state
func (a *App) syncFooter() {
	// Left1: Navigation hint (esc back/quit)
	switch a.currentView {
	case ViewInstanceDetails, ViewMetadata, ViewProjectMetadata, ViewDiskDetails, ViewSnapshotDetails, ViewImageDetails, ViewObjects, ViewInstanceEditor, ViewFirewallDetails:
		a.footer.SetLeft1("esc back")
	case ViewProjects, ViewInstances, ViewDisks, ViewSnapshots, ViewImages, ViewBuckets, ViewNetworks, ViewFirewall:
		a.footer.SetLeft1("esc quit")
	default:
		a.footer.ClearLeft1()
	}

	// Left2: Sidebar focus hint (only when sidebar is active)
	if a.sidebarActive() {
		if a.focusedPanel == FocusSidebar {
			a.footer.SetLeft2("] content")
		} else {
			a.footer.SetLeft2("[ sidebar")
		}
	} else {
		a.footer.ClearLeft2()
	}

	// Left3: Help shortcuts
	a.footer.SetLeft3(": cmd • ? help • q quit")

	// Center: Task status (pre-rendered with custom styles)
	taskStatus, taskBg := a.renderTaskStatus()
	if taskStatus != "" {
		a.footer.SetCenterStyled(taskStatus, taskBg)
	} else {
		a.footer.ClearCenter()
	}

	// Right1: GCloud configuration (only if not "default")
	if a.configProfile != "" && a.configProfile != "default" {
		// Generate color from configuration name
		bg := colorFromString(a.configProfile)
		configStyle := lipgloss.NewStyle().
			Background(bg).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1)
		a.footer.SetRight1Styled(configStyle.Render(fmt.Sprintf("[%s]", a.configProfile)), bg)
	} else {
		a.footer.ClearRight1Styled()
	}

	// Right2: Authenticated identity (email of user or service account) with type icon
	if a.authenticatedIdentity != "" {
		// Get identity type icon
		var icon string
		switch a.identityType {
		case config.IdentityUser:
			icon = symbols.IdentityUser()
		case config.IdentityServiceAccount:
			icon = symbols.IdentityService()
		default:
			icon = ""
		}

		// Truncate long emails to fit in footer (account for icon + space)
		truncated := truncateEmail(a.authenticatedIdentity, 23)

		// Generate color from identity (email/service account name)
		bgColor := colorFromString(a.authenticatedIdentity)

		identityStyle := lipgloss.NewStyle().
			Background(bgColor).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1)

		// Render icon + email
		var content string
		if icon != "" {
			content = icon + " " + truncated
		} else {
			content = truncated
		}

		a.footer.SetRight2Styled(identityStyle.Render(content), bgColor)
	} else {
		a.footer.ClearRight2Styled()
	}

	// Right3: Project info (if selected) with color based on project ID
	if a.selectedProject != nil {
		bg := colorFromString(a.selectedProject.ID)
		projectStyle := lipgloss.NewStyle().
			Background(bg).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1)
		a.footer.SetRight3Styled(projectStyle.Render(a.selectedProject.ID), bg)
	} else {
		a.footer.ClearRight3Styled()
	}
}

// colorFromString generates a consistent color based on string hash.
// Uses HSL color space for visually distinct, saturated colors.
func colorFromString(s string) lipgloss.Color {
	// Simple hash using FNV-1a algorithm
	var hash uint32 = 2166136261
	for i := range len(s) {
		hash ^= uint32(s[i])
		hash *= 16777619
	}

	// Generate hue from hash (0-360), keep saturation and lightness fixed
	// for readable text on white foreground
	hue := float64(hash%360) / 360.0
	sat := 0.65 // Good saturation for vibrant colors
	lum := 0.45 // Darker for white text readability

	// HSL to RGB conversion
	var r, g, b float64
	if sat == 0 {
		r, g, b = lum, lum, lum
	} else {
		var q float64
		if lum < 0.5 {
			q = lum * (1 + sat)
		} else {
			q = lum + sat - lum*sat
		}
		p := 2*lum - q
		r = hueToRGB(p, q, hue+1.0/3.0)
		g = hueToRGB(p, q, hue)
		b = hueToRGB(p, q, hue-1.0/3.0)
	}

	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X",
		int(r*255), int(g*255), int(b*255)))
}

// hueToRGB is a helper for HSL to RGB conversion
func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}
	if t < 1.0/6.0 {
		return p + (q-p)*6*t
	}
	if t < 1.0/2.0 {
		return q
	}
	if t < 2.0/3.0 {
		return p + (q-p)*(2.0/3.0-t)*6
	}
	return p
}

// Task status styles with colored backgrounds
var (
	taskRunningStyle = lipgloss.NewStyle().
				Background(ColorPrimary).
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1)

	taskSuccessStyle = lipgloss.NewStyle().
				Background(ColorSecondary).
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1)

	taskErrorStyle = lipgloss.NewStyle().
			Background(ColorError).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1)
)

// renderTaskStatus returns styled task status text and background color, or empty string if no tasks
func (a *App) renderTaskStatus() (string, lipgloss.Color) {
	if len(a.ctx.Tasks) == 0 {
		return "", ""
	}

	// Find the most relevant task to display (prefer running, then most recent)
	var displayTask *context.Task
	var runningCount int

	for _, task := range a.ctx.Tasks {
		t := task
		if t.State == context.TaskRunning {
			runningCount++
		}
		if displayTask == nil {
			displayTask = &t
			continue
		}
		// Prefer running tasks over finished/error
		if t.State == context.TaskRunning && displayTask.State != context.TaskRunning {
			displayTask = &t
		}
	}

	if displayTask == nil {
		return "", ""
	}

	var status string
	var bg lipgloss.Color
	switch displayTask.State {
	case context.TaskRunning:
		text := "⠋ " + displayTask.Description
		if runningCount > 1 {
			text += fmt.Sprintf(" (+%d)", runningCount-1)
		}
		status = taskRunningStyle.Render(text)
		bg = ColorPrimary
	case context.TaskFinished:
		status = taskSuccessStyle.Render("✓ " + displayTask.Description)
		bg = ColorSecondary
	case context.TaskError:
		errMsg := displayTask.Description
		if displayTask.Error != nil {
			errMsg = displayTask.Error.Error()
		}
		status = taskErrorStyle.Render("✗ " + errMsg)
		bg = ColorError
	}

	return status, bg
}

// truncateEmail truncates an email to fit maxWidth, preserving start and domain.
// Uses smart truncation that keeps the beginning of the username and the domain visible.
func truncateEmail(email string, maxWidth int) string {
	if len(email) <= maxWidth {
		return email
	}

	// Need at least 10 chars for meaningful truncation (e.g., "ab...xy.com")
	if maxWidth < 10 {
		if maxWidth <= len(email) {
			return email[:maxWidth]
		}
		return email
	}

	// Split at @
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		// No @ found, simple truncation
		if maxWidth > 3 && len(email) > maxWidth {
			return email[:maxWidth-3] + "..."
		}
		return email[:maxWidth]
	}

	username := parts[0]
	domain := parts[1]

	// Calculate available space: maxWidth - 3 (for "...")
	availableSpace := maxWidth - 3

	// Try to preserve reasonable parts of both username and domain
	// Allocate space: favor showing more of username, but keep domain readable
	domainLen := len(domain)

	// If domain is short enough to fit with some username, keep full domain
	if domainLen <= availableSpace/2 && domainLen < availableSpace {
		// Keep full domain, truncate username
		usernameLen := availableSpace - domainLen
		if usernameLen > len(username) {
			usernameLen = len(username)
		}
		if usernameLen < 1 {
			usernameLen = 1
		}
		return username[:usernameLen] + "..." + domain
	}

	// Both are long, split space roughly 60/40 (username/domain)
	usernameLen := (availableSpace * 6) / 10
	domainLen = availableSpace - usernameLen

	// Ensure we don't exceed actual lengths
	if usernameLen > len(username) {
		usernameLen = len(username)
	}
	if domainLen > len(domain) {
		domainLen = len(domain)
	}

	// Ensure minimum lengths
	if usernameLen < 1 {
		usernameLen = 1
	}
	if domainLen < 1 {
		domainLen = 1
	}

	// Truncate and format
	return username[:usernameLen] + "..." + domain[len(domain)-domainLen:]
}
