package views

import (
	gocontext "context"
	"fmt"
	"regexp"
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
	"github.com/slayer/gcon/internal/ui/components/links"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/focus"
	"github.com/slayer/gcon/internal/ui/overlay"
	"github.com/slayer/gcon/internal/ui/symbols"
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// snapshotDetailsLoadedMsg contains the fetched snapshot details
type snapshotDetailsLoadedMsg struct {
	details *gcp.SnapshotDetails
}

// snapshotDetailsErrorMsg indicates an error loading details
type snapshotDetailsErrorMsg struct {
	err error
}

// SnapshotDiskSelectedMsg is sent when a disk link is selected in snapshot details
type SnapshotDiskSelectedMsg struct {
	DiskName string
	Zone     string
}

// Focus region IDs for snapshot details view
const (
	snapshotRegionIDLinks    = "links"
	snapshotRegionIDViewport = "viewport"
)

// SnapshotDetailsView displays comprehensive snapshot information
type SnapshotDetailsView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	snapshotName  string
	ctx           *context.ProgramContext // Shared context for dimensions and styles
	details       *gcp.SnapshotDetails
	viewport      viewport.Model
	spinner       spinner.Model
	loading       bool
	err           error
	width         int
	height        int
	keys          snapshotDetailsKeyMap
	ready         bool
	diskLink      *links.Links   // Navigable link to source disk
	focusMgr      *focus.Manager // Focus management for routing keys between regions

	// Action menu state
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Delete confirmation state
	deleteConfirm     *confirm.ConfirmDialog
	showDeleteConfirm bool
}

type snapshotDetailsKeyMap struct {
	Up          key.Binding
	Down        key.Binding
	Refresh     key.Binding
	ActionMenu  key.Binding
	Delete      key.Binding
	CreateDisk  key.Binding
	CreateImage key.Binding
}

func defaultSnapshotDetailsKeyMap() snapshotDetailsKeyMap {
	return snapshotDetailsKeyMap{
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
		CreateImage: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "create image"),
		),
	}
}

// NewSnapshotDetailsView creates a new snapshot details view
func NewSnapshotDetailsView(projectID, snapshotName string, computeClient *gcp.ComputeClient) *SnapshotDetailsView {
	s := components.NewGCPSpinner()

	// Initialize focus manager - links region starts disabled until disk info loads
	fm := focus.NewManager()
	fm.SetRegions([]focus.Region{
		focus.NewDisabledRegion(snapshotRegionIDLinks, focus.RegionLinks, "Source Disk"),
		focus.NewRegion(snapshotRegionIDViewport, focus.RegionViewport, "Content"),
	})

	return &SnapshotDetailsView{
		computeClient: computeClient,
		projectID:     projectID,
		snapshotName:  snapshotName,
		spinner:       s,
		loading:       true,
		keys:          defaultSnapshotDetailsKeyMap(),
		diskLink:      links.New(),
		focusMgr:      fm,
	}
}

// Init initializes the view and starts loading snapshot details
func (v *SnapshotDetailsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.loadDetails(),
	)
}

// loadDetails fetches snapshot details from GCP
func (v *SnapshotDetailsView) loadDetails() tea.Cmd {
	return func() tea.Msg {
		details, err := v.computeClient.GetSnapshotDetails(gocontext.Background(), v.projectID, v.snapshotName)
		if err != nil {
			return snapshotDetailsErrorMsg{err: err}
		}
		return snapshotDetailsLoadedMsg{details: details}
	}
}

// Update handles messages for the snapshot details view
//nolint:gocognit // Bubble Tea Update pattern - complexity 45
func (v *SnapshotDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case snapshotDetailsLoadedMsg:
		v.loading = false
		v.details = msg.details
		v.updateViewportContent()
		return nil

	case snapshotDetailsErrorMsg:
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
			return DeleteSnapshotConfirmedMsg{
				SnapshotName: v.snapshotName,
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

	case links.LinkSelectedMsg:
		// Handle disk link selection - navigate to disk details
		if msg.Link.Type == "disk" {
			// Extract zone and disk name from the link data
			if diskInfo, ok := msg.Link.Data.(diskLinkData); ok {
				if diskInfo.DiskName != "" && diskInfo.Zone != "" {
					return func() tea.Msg {
						return SnapshotDiskSelectedMsg(diskInfo)
					}
				}
			}
		}
		return nil

	case focus.FocusChangedMsg:
		// Focus changed between regions - update rendering
		v.updateViewportContent()
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

		// Handle Tab/Shift+Tab for cycling between focus regions
		if focusMsg := v.focusMgr.HandleKey(msg); focusMsg != nil {
			v.updateViewportContent()
			return func() tea.Msg { return focusMsg }
		}

		// Route keys based on currently focused region
		switch v.focusMgr.ActiveType() {
		case focus.RegionLinks:
			// When links region is focused, j/k navigate links
			if v.diskLink.HasItems() && links.HandleKey(msg) {
				cmd := v.diskLink.Update(msg)
				v.updateViewportContent()
				return cmd
			}

		case focus.RegionViewport:
			// When viewport region is focused, j/k scroll content
			if v.ready {
				var cmd tea.Cmd
				v.viewport, cmd = v.viewport.Update(msg)
				return cmd
			}
		}

		// View-specific action keys (work regardless of focus)
		switch {
		case key.Matches(msg, v.keys.ActionMenu):
			if v.details != nil {
				v.actionMenu = actionmenu.New("Snapshot Actions", v.buildActions())
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
			// Create disk from this snapshot
			if v.details != nil && v.details.Status == "READY" {
				return func() tea.Msg {
					return DiskCreateFromSnapshotRequestMsg{
						SnapshotName: v.details.Name,
						SnapshotSize: v.details.DiskSizeGB,
					}
				}
			}
			return nil

		case key.Matches(msg, v.keys.CreateImage):
			// Create image from this snapshot
			if v.details != nil && v.details.Status == "READY" {
				return func() tea.Msg {
					return ImageCreateFromSnapshotRequestMsg{
						SnapshotName: v.details.Name,
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

	return nil
}

// buildActions creates the action menu items
func (v *SnapshotDetailsView) buildActions() []actionmenu.Action {
	// Check if snapshot is ready for disk/image creation
	isReady := v.details != nil && v.details.Status == "READY"
	return []actionmenu.Action{
		{Key: 'c', Label: "Create Disk", Enabled: isReady},
		{Key: 'i', Label: "Create Image", Enabled: isReady},
		{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true},
		{Key: 'r', Label: "Refresh", Enabled: true},
	}
}

// executeAction performs the action selected from the menu
func (v *SnapshotDetailsView) executeAction(actionKey rune) tea.Cmd {
	if v.details == nil {
		return nil
	}

	switch actionKey {
	case 'c':
		// Create disk from this snapshot
		if v.details.Status == "READY" {
			return func() tea.Msg {
				return DiskCreateFromSnapshotRequestMsg{
					SnapshotName: v.details.Name,
					SnapshotSize: v.details.DiskSizeGB,
				}
			}
		}
	case 'i':
		// Create image from this snapshot
		if v.details.Status == "READY" {
			return func() tea.Msg {
				return ImageCreateFromSnapshotRequestMsg{
					SnapshotName: v.details.Name,
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
func (v *SnapshotDetailsView) showDeleteConfirmation() tea.Cmd {
	details := []string{}
	if v.details != nil {
		details = append(details, fmt.Sprintf("Size: %d GB", v.details.DiskSizeGB))
		if v.details.SourceDisk != "" {
			details = append(details, fmt.Sprintf("Source Disk: %s", v.details.SourceDisk))
		}
	}
	details = append(details, "This action cannot be undone.")

	v.deleteConfirm = confirm.New(
		"Delete Snapshot",
		fmt.Sprintf("Are you sure you want to delete snapshot '%s'?", v.snapshotName),
		details,
	)
	v.showDeleteConfirm = true
	return nil
}

// IsMenuOpen returns true if the action menu or confirm dialog is currently open
func (v *SnapshotDetailsView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
}

// View renders the snapshot details view
func (v *SnapshotDetailsView) View() string {
	if v.loading {
		return renderLoading(v.spinner,"Loading snapshot details...")
	}

	if v.err != nil {
		return renderLoading(v.spinner,fmt.Sprintf("Error: %v\n  Press 'esc' to go back", v.err))
	}

	if v.details == nil {
		return renderLoading(v.spinner,"No snapshot details available.\n  Press 'esc' to go back.")
	}

	if !v.ready {
		return renderLoading(v.spinner,"Initializing view...")
	}

	// Help text - context-sensitive based on focused region
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	scrollInfo := scrollStyle.Render(fmt.Sprintf("%.0f%%", v.viewport.ScrollPercent()*100))

	helpText := v.buildHelpText()
	help := helpStyle.Render(helpText) + " " + scrollInfo

	// Wrap viewport with focus accent
	mainContent := focus.RenderAccent(v.viewport.View(), v.focusMgr.IsActive(snapshotRegionIDViewport)) + help

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
func (v *SnapshotDetailsView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// SetContext updates the view with shared program context.
// Reads dimensions from the context for consistent sizing.
func (v *SnapshotDetailsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// applySize applies the given dimensions to the viewport
func (v *SnapshotDetailsView) applySize(width, height int) {
	// Reserve space for footer
	viewportHeight := height - 4
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	// Reserve 1 char for focus accent bar
	viewportWidth := width - 1
	if viewportWidth < 1 {
		viewportWidth = 1
	}

	if !v.ready {
		v.viewport = viewport.New(viewportWidth, viewportHeight)
		v.viewport.Style = lipgloss.NewStyle().Padding(0, 2)
		v.ready = true
	} else {
		v.viewport.Width = viewportWidth
		v.viewport.Height = viewportHeight
	}

	if v.details != nil {
		v.updateViewportContent()
	}
}

// updateViewportContent renders the details content into the viewport
func (v *SnapshotDetailsView) updateViewportContent() {
	if v.details == nil || !v.ready {
		return
	}

	// Populate disk link from snapshot details
	v.populateDiskLink()

	// Update links focus state based on focus manager
	v.diskLink.SetRegionFocused(v.focusMgr.IsActive(snapshotRegionIDLinks))

	content := v.renderContent()
	v.viewport.SetContent(content)
}

// renderContent generates the full details content
func (v *SnapshotDetailsView) renderContent() string {
	d := v.details
	var b strings.Builder

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).MarginTop(1)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Width(24)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B6B6B"))

	// Header with status
	statusIcon := snapshotDetailStatusIcon(d.Status)
	b.WriteString(titleStyle.Render(fmt.Sprintf("Snapshot: %s  %s %s", d.Name, statusIcon, d.Status)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(v.width-4, 60)))
	b.WriteString("\n\n")

	// Basic Information
	b.WriteString(sectionStyle.Render("Basic Information"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Name", d.Name))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Snapshot ID", strconv.FormatUint(d.ID, 10)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Description", defaultIfEmpty(d.Description, "None")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Status", fmt.Sprintf("%s %s", statusIcon, d.Status)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Created", timeutil.FormatTimestamp(d.CreatedAt)))
	if d.AutoCreated {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Auto-Created", "Yes"))
	}
	b.WriteString("\n")

	// Labels
	if len(d.Labels) > 0 {
		b.WriteString(labelStyle.Render("Labels"))
		b.WriteString("\n")
		for k, val := range d.Labels {
			b.WriteString(fmt.Sprintf("    %s: %s\n", k, val))
		}
	} else {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Labels", "None"))
	}
	b.WriteString("\n")

	// Source Information
	b.WriteString(sectionStyle.Render("Source"))
	b.WriteString("\n")

	// Render source disk as navigable link if available
	if v.diskLink.HasItems() {
		diskName := defaultIfEmpty(d.SourceDisk, "Unknown")
		// Render label with proper formatting
		label := labelStyle.Render("Source Disk:")
		// Get link rendering (includes cursor and highlighting)
		linkRendered := v.diskLink.RenderRow(0, diskName)
		// Combine label and link on same line
		b.WriteString(label + " " + linkRendered + "\n")
	} else {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Disk", defaultIfEmpty(d.SourceDisk, "Unknown")))
	}

	if d.SourceDiskZone != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Disk Zone", d.SourceDiskZone))
	}
	b.WriteString("\n")

	// Size and Storage
	b.WriteString(sectionStyle.Render("Size and Storage"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Disk Size", fmt.Sprintf("%d GB", d.DiskSizeGB)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Storage Used", fmt.Sprintf("%d GB (%.1f GB)", d.StorageBytesGb, float64(d.StorageBytes)/1024/1024/1024)))
	if d.SnapshotType != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Snapshot Type", d.SnapshotType))
	}
	b.WriteString("\n")

	// Storage Locations
	if len(d.StorageLocations) > 0 {
		b.WriteString(sectionStyle.Render("Storage Locations"))
		b.WriteString("\n")
		if len(d.StorageLocations) == 1 {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Location", d.StorageLocations[0]))
		} else {
			b.WriteString(labelStyle.Render("Locations:"))
			b.WriteString("\n")
			for _, loc := range d.StorageLocations {
				b.WriteString(fmt.Sprintf("  • %s\n", loc))
			}
		}
		b.WriteString("\n")
	}

	// Encryption
	b.WriteString(sectionStyle.Render("Encryption"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Snapshot Encryption", defaultIfEmpty(d.SnapshotEncryptionKey, "Google-managed")))
	if d.SourceDiskEncryption != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Disk Encryption", d.SourceDiskEncryption))
	}
	b.WriteString("\n")

	// Additional Information
	if d.ChainName != "" || d.SatisfiesPZS {
		b.WriteString(sectionStyle.Render("Additional Information"))
		b.WriteString("\n")
		if d.ChainName != "" {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Chain Name", d.ChainName))
		}
		if d.SatisfiesPZS {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Satisfies PZS", "Yes"))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// snapshotDetailStatusIcon returns an appropriate status icon for the snapshot details view
func snapshotDetailStatusIcon(status string) string {
	switch status {
	case "READY":
		return symbols.StatusRunning() // Green - ready to use
	case "CREATING", "UPLOADING":
		return symbols.StatusTransitioning() // Yellow - in progress
	case "FAILED":
		return symbols.StatusStopped() // Red - failed
	case "DELETING":
		return symbols.StatusTransitioning() // Yellow - being deleted
	default:
		return symbols.StatusUnknown() // Unknown status
	}
}

// GetSnapshotName returns the snapshot name for use in breadcrumbs
func (v *SnapshotDetailsView) GetSnapshotName() string {
	return v.snapshotName
}

// GetComputeClient returns the compute client for reuse in other detail views
func (v *SnapshotDetailsView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

// diskLinkData holds disk information for navigation
type diskLinkData struct {
	DiskName string
	Zone     string
}

var snapshotDiskSourceRegex = regexp.MustCompile(`projects/[^/]+/zones/([^/]+)/disks/([^/]+)`)

// extractDiskInfoFromSnapshotSource parses a source disk URL and returns disk name and zone
// Source format: projects/{project}/zones/{zone}/disks/{diskName}
func extractDiskInfoFromSnapshotSource(source string) (diskName, zone string) {
	matches := snapshotDiskSourceRegex.FindStringSubmatch(source)
	if len(matches) == 3 {
		// matches[0] is the full string, [1] is zone, [2] is diskName
		return matches[2], matches[1]
	}
	return "", ""
}

// populateDiskLink creates a link item from the snapshot's source disk if available
// and enables/disables the links focus region accordingly
func (v *SnapshotDetailsView) populateDiskLink() {
	if v.details == nil || v.details.SourceDiskID == "" {
		v.diskLink.SetItems(nil)
		v.focusMgr.DisableRegion(snapshotRegionIDLinks)
		return
	}

	// Extract disk name and zone from source disk URL
	diskName, zone := extractDiskInfoFromSnapshotSource(v.details.SourceDiskID)
	if diskName == "" || zone == "" {
		v.diskLink.SetItems(nil)
		v.focusMgr.DisableRegion(snapshotRegionIDLinks)
		return
	}

	// Create single link item for the source disk
	items := []links.Link{
		{
			ID:    diskName,
			Label: diskName, // Will be formatted in renderContent
			Type:  "disk",
			Data: diskLinkData{
				DiskName: diskName,
				Zone:     zone,
			},
		},
	}
	v.diskLink.SetItems(items)
	v.focusMgr.EnableRegion(snapshotRegionIDLinks)
}

// buildHelpText generates context-sensitive help text based on the focused region
func (v *SnapshotDetailsView) buildHelpText() string {
	bindings := focus.HelpForRegion(v.focusMgr.ActiveType(), v.getRegionLabel())
	helpStr := focus.FormatHelp(bindings)
	badge := focus.FormatRegionBadge(v.focusMgr.Active())
	if badge != "" {
		return "\n  " + badge + " • " + helpStr + " • .: actions • D: delete • r: refresh"
	}
	return "\n  " + helpStr + " • .: actions • D: delete • r: refresh"
}

// getRegionLabel returns a descriptive label for the current focus context
func (v *SnapshotDetailsView) getRegionLabel() string {
	if v.focusMgr.ActiveType() == focus.RegionLinks {
		return "disk"
	}
	return ""
}
