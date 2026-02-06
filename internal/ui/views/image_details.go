package views

import (
	gocontext "context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/actionmenu"
	"github.com/slayer/gcon/internal/ui/components/confirm"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/overlay"
	"github.com/slayer/gcon/internal/ui/symbols"
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// imageDetailsLoadedMsg contains the fetched image details
type imageDetailsLoadedMsg struct {
	details *gcp.ImageDetails
}

// imageDetailsErrorMsg indicates an error loading details
type imageDetailsErrorMsg struct {
	err error
}

// ImageDetailsView displays comprehensive disk image information
type ImageDetailsView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	imageName     string
	ctx           *context.ProgramContext // Shared context for dimensions and styles
	details       *gcp.ImageDetails
	viewport      viewport.Model
	spinner       spinner.Model
	loading       bool
	err           error
	width         int
	height        int
	keys          imageDetailsKeyMap
	ready         bool

	// Action menu state
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Delete confirmation state
	deleteConfirm     *confirm.ConfirmDialog
	showDeleteConfirm bool
}

type imageDetailsKeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Refresh    key.Binding
	ActionMenu key.Binding
	Delete     key.Binding
	CreateDisk key.Binding
}

func defaultImageDetailsKeyMap() imageDetailsKeyMap {
	return imageDetailsKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		ActionMenu: key.NewBinding(
			key.WithKeys("."),
			key.WithHelp(".", "actions"),
		),
		Delete: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "delete"),
		),
		CreateDisk: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "create disk"),
		),
	}
}

// NewImageDetailsView creates a new image details view
func NewImageDetailsView(projectID, imageName string, computeClient *gcp.ComputeClient) *ImageDetailsView {
	s := components.NewGCPSpinner()

	return &ImageDetailsView{
		computeClient: computeClient,
		projectID:     projectID,
		imageName:     imageName,
		spinner:       s,
		loading:       true,
		keys:          defaultImageDetailsKeyMap(),
	}
}

// Init initializes the view and starts loading image details
func (v *ImageDetailsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.loadDetails(),
	)
}

// loadDetails fetches image details from GCP
func (v *ImageDetailsView) loadDetails() tea.Cmd {
	return func() tea.Msg {
		details, err := v.computeClient.GetImageDetails(gocontext.Background(), v.projectID, v.imageName)
		if err != nil {
			return imageDetailsErrorMsg{err: err}
		}
		return imageDetailsLoadedMsg{details: details}
	}
}

// Update handles messages for the image details view
//nolint:gocognit // Bubble Tea Update pattern - complexity 45
func (v *ImageDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case imageDetailsLoadedMsg:
		v.loading = false
		v.details = msg.details
		v.updateViewportContent()
		return nil

	case imageDetailsErrorMsg:
		v.loading = false
		v.err = msg.err
		return nil

	case actionmenu.ActionSelectedMsg:
		v.menuOpen = false
		return v.executeAction(msg.Key)

	case actionmenu.ActionMenuClosedMsg:
		v.menuOpen = false
		return nil

	case confirm.ConfirmMsg:
		v.showDeleteConfirm = false
		return func() tea.Msg {
			return DeleteImageConfirmedMsg{
				ImageName: v.imageName,
			}
		}

	case confirm.CancelMsg:
		v.showDeleteConfirm = false
		return nil

	case spinner.TickMsg:
		if v.loading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case tea.KeyMsg:
		// Route to delete confirmation dialog when shown
		if v.showDeleteConfirm && v.deleteConfirm != nil {
			return v.deleteConfirm.Update(msg)
		}

		// Route to action menu when open
		if v.menuOpen && v.actionMenu != nil {
			return v.actionMenu.Update(msg)
		}

		switch {
		case key.Matches(msg, v.keys.ActionMenu):
			if v.details != nil {
				v.actionMenu = actionmenu.New("Image Actions", v.buildActions())
				v.menuOpen = true
			}
			return nil

		case key.Matches(msg, v.keys.Delete):
			// Show delete confirmation dialog
			if v.details != nil {
				return v.showDeleteConfirmation()
			}
			return nil

		case key.Matches(msg, v.keys.CreateDisk):
			// Create disk from this image
			if v.details != nil && v.details.Status == "READY" {
				return func() tea.Msg {
					return DiskCreateFromImageRequestMsg{
						ImageName: v.details.Name,
						ImageSize: v.details.DiskSizeGB,
					}
				}
			}
			return nil

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.loadDetails())
		}
	}

	// Handle viewport scrolling
	if v.ready {
		var cmd tea.Cmd
		v.viewport, cmd = v.viewport.Update(msg)
		return cmd
	}

	return nil
}

// buildActions creates the action menu items
func (v *ImageDetailsView) buildActions() []actionmenu.Action {
	// Check if image is ready for disk creation
	isReady := v.details != nil && v.details.Status == "READY"
	return []actionmenu.Action{
		{Key: 'c', Label: "Create Disk", Enabled: isReady},
		{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true},
		{Key: 'r', Label: "Refresh", Enabled: true},
	}
}

// executeAction performs the action selected from the menu
func (v *ImageDetailsView) executeAction(actionKey rune) tea.Cmd {
	if v.details == nil {
		return nil
	}

	switch actionKey {
	case 'c':
		// Create disk from this image
		if v.details.Status == "READY" {
			return func() tea.Msg {
				return DiskCreateFromImageRequestMsg{
					ImageName: v.details.Name,
					ImageSize: v.details.DiskSizeGB,
				}
			}
		}
	case 'D':
		return v.showDeleteConfirmation()
	case 'r':
		v.loading = true
		v.err = nil
		return tea.Batch(v.spinner.Tick, v.loadDetails())
	}

	return nil
}

// showDeleteConfirmation opens the delete confirmation dialog
func (v *ImageDetailsView) showDeleteConfirmation() tea.Cmd {
	details := []string{}
	if v.details != nil {
		details = append(details, fmt.Sprintf("Disk Size: %d GB", v.details.DiskSizeGB))
		if v.details.Family != "" {
			details = append(details, fmt.Sprintf("Family: %s", v.details.Family))
		}
	}
	details = append(details, "This action cannot be undone.")

	v.deleteConfirm = confirm.New(
		"Delete Image",
		fmt.Sprintf("Are you sure you want to delete image '%s'?", v.imageName),
		details,
	)
	v.showDeleteConfirm = true
	return nil
}

// IsMenuOpen returns true if the action menu or confirm dialog is currently open
func (v *ImageDetailsView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
}

// GetComputeClient returns the compute client for reuse in other views
func (v *ImageDetailsView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

// View renders the image details view
func (v *ImageDetailsView) View() string {
	if v.loading {
		return renderLoading(v.spinner,"Loading image details...")
	}

	if v.err != nil {
		return renderLoading(v.spinner,fmt.Sprintf("Error: %v\n  Press 'esc' to go back", v.err))
	}

	if v.details == nil {
		return renderLoading(v.spinner,"No image details available.\n  Press 'esc' to go back.")
	}

	if !v.ready {
		return renderLoading(v.spinner,"Initializing view...")
	}

	// Help text
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	scrollInfo := scrollStyle.Render(fmt.Sprintf("%.0f%%", v.viewport.ScrollPercent()*100))
	help := helpStyle.Render("\n  ↑/↓: scroll • .: actions • c: create disk • D: delete • r: refresh • esc: back") + " " + scrollInfo

	mainContent := v.viewport.View() + help

	// Overlay action menu if open
	if v.menuOpen && v.actionMenu != nil {
		return v.renderWithOverlay(mainContent, v.actionMenu.View())
	}

	// Overlay delete confirmation if shown
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.renderWithOverlay(mainContent, v.deleteConfirm.View())
	}

	return mainContent
}

// renderWithOverlay overlays a dialog centered on top of the content
func (v *ImageDetailsView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// SetContext updates the view with shared program context.
// Reads dimensions from the context for consistent sizing.
func (v *ImageDetailsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// applySize applies the given dimensions to the viewport
func (v *ImageDetailsView) applySize(width, height int) {
	// Reserve space for footer
	viewportHeight := height - 4
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	if !v.ready {
		v.viewport = viewport.New(width, viewportHeight)
		v.viewport.Style = lipgloss.NewStyle().Padding(0, 2)
		v.ready = true
	} else {
		v.viewport.Width = width
		v.viewport.Height = viewportHeight
	}

	if v.details != nil {
		v.updateViewportContent()
	}
}

// updateViewportContent renders the details content into the viewport
func (v *ImageDetailsView) updateViewportContent() {
	if v.details == nil || !v.ready {
		return
	}

	content := v.renderContent()
	v.viewport.SetContent(content)
}

// renderContent generates the full details content
//nolint:gocognit // Detail rendering with multiple sections - complexity 37
func (v *ImageDetailsView) renderContent() string {
	d := v.details
	var b strings.Builder

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).MarginTop(1)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Width(24)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B6B6B"))

	// Header with status
	statusIcon := imageDetailStatusIcon(d.Status)
	b.WriteString(titleStyle.Render(fmt.Sprintf("Image: %s  %s %s", d.Name, statusIcon, d.Status)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(v.width-4, 60)))
	b.WriteString("\n\n")

	// Basic Information
	b.WriteString(sectionStyle.Render("Basic Information"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Name", d.Name))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Image ID", strconv.FormatUint(d.ID, 10)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Family", defaultIfEmpty(d.Family, "None")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Description", defaultIfEmpty(d.Description, "None")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Status", fmt.Sprintf("%s %s", statusIcon, d.Status)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Created", timeutil.FormatTimestamp(d.CreatedAt)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Created By", d.CreatedBy))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Architecture", d.Architecture))
	b.WriteString("\n")

	// Deprecation status
	if d.Deprecated != nil {
		b.WriteString(sectionStyle.Render("Deprecation"))
		b.WriteString("\n")
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "State", d.Deprecated.State))
		if d.Deprecated.Replacement != "" {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Replacement", d.Deprecated.Replacement))
		}
		if d.Deprecated.Deprecated != "" {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Deprecated Date", timeutil.FormatTimestamp(d.Deprecated.Deprecated)))
		}
		if d.Deprecated.Obsolete != "" {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Obsolete Date", timeutil.FormatTimestamp(d.Deprecated.Obsolete)))
		}
		if d.Deprecated.Deleted != "" {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Deleted Date", timeutil.FormatTimestamp(d.Deprecated.Deleted)))
		}
		b.WriteString("\n")
	}

	// Labels
	if len(d.Labels) > 0 {
		b.WriteString(sectionStyle.Render("Labels"))
		b.WriteString("\n")
		for k, val := range d.Labels {
			b.WriteString(fmt.Sprintf("    %s: %s\n", k, val))
		}
		b.WriteString("\n")
	}

	// Size Information
	b.WriteString(sectionStyle.Render("Size Information"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Disk Size", fmt.Sprintf("%d GB", d.DiskSizeGB)))
	if d.ArchiveSizeB > 0 {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Archive Size", formatBytes(d.ArchiveSizeB)))
	}
	if d.StorageBytes > 0 {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Storage Used", formatBytes(d.StorageBytes)))
	}
	if len(d.StorageLocations) > 0 {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Storage Locations", strings.Join(d.StorageLocations, ", ")))
	}
	b.WriteString("\n")

	// Source
	b.WriteString(sectionStyle.Render("Source"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Type", defaultIfEmpty(d.SourceType, "RAW")))
	if d.SourceDisk != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Disk", d.SourceDisk))
		if d.SourceDiskID != "" {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Disk ID", d.SourceDiskID))
		}
	}
	if d.SourceSnapshot != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Snapshot", d.SourceSnapshot))
	}
	if d.SourceImage != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Image", d.SourceImage))
		if d.SourceImageID != "" {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Image ID", d.SourceImageID))
		}
	}
	b.WriteString("\n")

	// Image Features
	b.WriteString(sectionStyle.Render("Image Features"))
	b.WriteString("\n")
	if len(d.GuestOSFeatures) > 0 {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Guest OS Features", strings.Join(d.GuestOSFeatures, ", ")))
	} else {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Guest OS Features", "None"))
	}
	if d.EnableConfidentialCompute {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Confidential Computing", "Enabled"))
	}
	if d.SatisfiesPzs {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Physical Zone Separation", "Yes"))
	}
	b.WriteString("\n")

	// Licenses
	if len(d.Licenses) > 0 || len(d.LicenseCodes) > 0 {
		b.WriteString(sectionStyle.Render("Licenses"))
		b.WriteString("\n")
		for _, license := range d.Licenses {
			b.WriteString(fmt.Sprintf("    • %s\n", license))
		}
		if len(d.LicenseCodes) > 0 {
			codes := make([]string, len(d.LicenseCodes))
			for i, code := range d.LicenseCodes {
				codes[i] = strconv.FormatInt(code, 10)
			}
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "License Codes", strings.Join(codes, ", ")))
		}
		b.WriteString("\n")
	}

	// Encryption
	b.WriteString(sectionStyle.Render("Encryption"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Image Encryption", d.ImageEncryptionKey))
	if d.SourceDiskEncryptionKey != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Disk Encryption", d.SourceDiskEncryptionKey))
	}
	if d.SourceSnapshotEncryptionKey != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Snapshot Encryption", d.SourceSnapshotEncryptionKey))
	}

	return b.String()
}

// imageDetailStatusIcon returns an appropriate status icon for the image details view
func imageDetailStatusIcon(status string) string {
	if status == "READY" {
		return symbols.StatusRunning() // Green - ready
	}
	if status == "FAILED" {
		return symbols.StatusStopped() // Red - failed
	}
	return symbols.StatusTransitioning() // Yellow - other states
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}

	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

// GetImageName returns the image name for use in breadcrumbs
func (v *ImageDetailsView) GetImageName() string {
	return v.imageName
}
