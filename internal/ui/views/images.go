package views

import (
	gocontext "context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/actionmenu"
	"github.com/slayer/gcon/internal/ui/components/confirm"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/overlay"
	"github.com/slayer/gcon/internal/ui/symbols"
)

// ImagesView displays Compute Engine disk images in a table format
type ImagesView struct {
	TableClickDelegate
	computeClient *gcp.ComputeClient
	projectID     string
	ctx           *context.ProgramContext // Shared context for dimensions and styles
	table         table.Model
	spinner       spinner.Model
	loading       bool
	err           error
	images        []gcp.Image
	keys          imageKeyMap

	// Action menu state
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Delete confirmation state
	deleteConfirm     *confirm.ConfirmDialog
	showDeleteConfirm bool
	pendingDelete     *gcp.Image // Image pending deletion

	// View dimensions for overlay rendering
	width  int
	height int
}

// imageKeyMap defines image-specific key bindings
type imageKeyMap struct {
	Enter      key.Binding
	Refresh    key.Binding
	ActionMenu key.Binding
	Delete     key.Binding
	CreateDisk key.Binding
}

func defaultImageKeyMap() imageKeyMap {
	return imageKeyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "details"),
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

// Table column definitions
func imageColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 28, Grow: true, Sortable: true},
		{Title: "Created By", Width: 20, Sortable: true},
		{Title: "Location", Width: 12, Sortable: true},
		{Title: "Disk Size", Width: 10, Sortable: true},
		{Title: "Archive Size", Width: 12, Sortable: true},
		{Title: "Family", Width: 18, Sortable: true},
	}
}

// NewImagesView creates a new images view with table display
func NewImagesView(projectID string) *ImagesView {
	title := fmt.Sprintf("Disk Images - %s", projectID)
	t := table.NewWithColumns(imageColumns(), title)

	s := components.NewGCPSpinner()

	v := &ImagesView{
		projectID: projectID,
		table:     t,
		spinner:   s,
		loading:   true,
		keys:      defaultImageKeyMap(),
	}
	v.Table = &v.table
	return v
}

// Init initializes the view and starts loading images
func (v *ImagesView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.initComputeClient(),
	)
}

// initComputeClient creates the compute client then loads images
func (v *ImagesView) initComputeClient() tea.Cmd {
	return func() tea.Msg {
		client, err := gcp.NewComputeClient(gocontext.Background())
		if err != nil {
			return imagesErrorMsg{err: err}
		}
		return imagesClientReadyMsg{client: client}
	}
}

// loadImages fetches images from GCP
func (v *ImagesView) loadImages() tea.Cmd {
	return func() tea.Msg {
		images, err := v.computeClient.ListImages(gocontext.Background(), v.projectID)
		if err != nil {
			return imagesErrorMsg{err: err}
		}
		return imagesLoadedMsg{images: images}
	}
}

// Message types
type imagesClientReadyMsg struct {
	client *gcp.ComputeClient
}

type imagesLoadedMsg struct {
	images []gcp.Image
}

type imagesErrorMsg struct {
	err error
}

// ImageSelectedMsg is sent when an image is selected from the list
type ImageSelectedMsg struct {
	Image gcp.Image
}

// imageStatusIcon returns a symbol indicator for image status
func imageStatusIcon(image gcp.Image) string { //nolint:gocritic // Copying image is acceptable
	if image.IsReady() {
		return symbols.StatusRunning() // Green - ready
	}
	if image.Status == "FAILED" {
		return symbols.StatusStopped() // Red - failed
	}
	// PENDING, DELETING, etc.
	return symbols.StatusTransitioning()
}

// imageToRow converts a GCP image to a table row
func imageToRow(image gcp.Image) table.Row { //nolint:gocritic // Copying image is acceptable
	// Combine status icon with name
	name := imageStatusIcon(image) + " " + image.Name

	// Format sizes
	diskSize := fmt.Sprintf("%d GB", image.DiskSizeGB)
	archiveSize := formatArchiveSize(image.ArchiveSizeBytes)

	// Get location (first storage location)
	location := "-"
	if len(image.StorageLocations) > 0 {
		location = image.StorageLocations[0]
	}

	return table.Row{
		Data: []string{
			name,
			image.CreatedBy,
			location,
			diskSize,
			archiveSize,
			image.Family,
		},
		FilterValue: image.Name + " " + image.Family + " " + image.CreatedBy + " " + location,
		ID:          image.Name,
	}
}

// formatArchiveSize formats archive size in bytes to human-readable format
func formatArchiveSize(bytes int64) string {
	if bytes == 0 {
		return "-"
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
	units := []string{"KB", "MB", "GB", "TB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

// Update handles messages for the images view
//
//nolint:gocognit // Bubble Tea Update pattern - complexity 45
func (v *ImagesView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case imagesClientReadyMsg:
		v.computeClient = msg.client
		return v.loadImages()

	case imagesLoadedMsg:
		v.loading = false
		v.images = msg.images

		// Convert images to table rows
		rows := make([]table.Row, len(msg.images))
		for i, image := range msg.images { //nolint:gocritic // Copying in range acceptable
			rows[i] = imageToRow(image)
		}
		v.table.SetRows(rows)
		return nil

	case imagesErrorMsg:
		v.loading = false
		v.err = msg.err
		return nil

	case table.RowDoubleClickedMsg:
		// Handle double-click on table row - navigate to details
		image := v.findImageByName(msg.RowID)
		if image != nil {
			return func() tea.Msg {
				return ImageSelectedMsg{Image: *image}
			}
		}
		return nil

	case actionmenu.ActionSelectedMsg:
		v.menuOpen = false
		return v.executeAction(msg.Key)

	case actionmenu.ActionMenuClosedMsg:
		v.menuOpen = false
		return nil

	case confirm.ConfirmMsg:
		v.showDeleteConfirm = false
		if v.pendingDelete != nil {
			image := v.pendingDelete
			v.pendingDelete = nil
			return func() tea.Msg {
				return DeleteImageConfirmedMsg{
					ImageName: image.Name,
				}
			}
		}
		return nil

	case confirm.CancelMsg:
		v.showDeleteConfirm = false
		v.pendingDelete = nil
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

		// Don't handle custom keys during loading
		if v.loading {
			return nil
		}

		// Delegate to table when sort menu is open
		if v.table.IsSortMenuOpen() {
			var cmd tea.Cmd
			v.table, cmd = v.table.Update(msg)
			return cmd
		}

		// Let table handle filtering mode
		if v.table.IsFiltering() {
			var cmd tea.Cmd
			v.table, cmd = v.table.Update(msg)
			return cmd
		}

		switch {
		case key.Matches(msg, v.keys.Enter):
			// Navigate to image details on Enter
			if row := v.table.SelectedRow(); row != nil {
				image := v.findImageByName(row.ID)
				if image != nil {
					return func() tea.Msg {
						return ImageSelectedMsg{Image: *image}
					}
				}
			}

		case key.Matches(msg, v.keys.ActionMenu):
			// Open action menu for selected image
			if row := v.table.SelectedRow(); row != nil {
				image := v.findImageByName(row.ID)
				if image != nil {
					v.actionMenu = actionmenu.New("Image Actions", v.buildActions(image))
					v.menuOpen = true
				}
			}
			return nil

		case key.Matches(msg, v.keys.Delete):
			// Direct hotkey for delete
			if row := v.table.SelectedRow(); row != nil {
				image := v.findImageByName(row.ID)
				if image != nil {
					return v.showDeleteConfirmation(image)
				}
			}
			return nil

		case key.Matches(msg, v.keys.CreateDisk):
			// Direct hotkey for create disk from image
			if row := v.table.SelectedRow(); row != nil {
				image := v.findImageByName(row.ID)
				if image != nil && image.IsReady() {
					return func() tea.Msg {
						return DiskCreateFromImageRequestMsg{
							ImageName: image.Name,
							ImageSize: image.DiskSizeGB,
						}
					}
				}
			}
			return nil

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			// Re-initialize client if previous attempt failed
			if v.computeClient == nil {
				return tea.Batch(v.spinner.Tick, v.initComputeClient())
			}
			return tea.Batch(v.spinner.Tick, v.loadImages())
		}
	}

	// Update table for navigation
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

// buildActions creates the action menu items
func (v *ImagesView) buildActions(image *gcp.Image) []actionmenu.Action {
	return []actionmenu.Action{
		{Key: 'c', Label: "Create Disk", Enabled: image.IsReady()},
		{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true},
	}
}

// executeAction performs the action selected from the menu
func (v *ImagesView) executeAction(actionKey rune) tea.Cmd {
	row := v.table.SelectedRow()
	if row == nil {
		return nil
	}

	image := v.findImageByName(row.ID)
	if image == nil {
		return nil
	}

	switch actionKey {
	case 'c':
		// Create disk from image
		if image.IsReady() {
			return func() tea.Msg {
				return DiskCreateFromImageRequestMsg{
					ImageName: image.Name,
					ImageSize: image.DiskSizeGB,
				}
			}
		}
	case 'D':
		return v.showDeleteConfirmation(image)
	}

	return nil
}

// showDeleteConfirmation opens the delete confirmation dialog
func (v *ImagesView) showDeleteConfirmation(image *gcp.Image) tea.Cmd {
	v.deleteConfirm = confirm.New(
		"Delete Image",
		fmt.Sprintf("Are you sure you want to delete image '%s'?", image.Name),
		[]string{
			fmt.Sprintf("Disk Size: %d GB", image.DiskSizeGB),
			fmt.Sprintf("Family: %s", defaultIfEmpty(image.Family, "None")),
			"This action cannot be undone.",
		},
	)
	v.showDeleteConfirm = true
	v.pendingDelete = image
	return nil
}

// IsMenuOpen returns true if the action menu or confirm dialog is currently open
func (v *ImagesView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
}

// findImageByName looks up an image by name
func (v *ImagesView) findImageByName(name string) *gcp.Image {
	for _, image := range v.images { //nolint:gocritic // Copying in range acceptable
		if image.Name == name {
			return &image
		}
	}
	return nil
}

// View renders the images view
func (v *ImagesView) View() string {
	if v.loading && v.computeClient == nil {
		return renderLoading(v.spinner, "Initializing Compute Engine client...")
	}

	if v.loading {
		return renderLoading(v.spinner, "Loading disk images...")
	}

	if v.err != nil {
		return "\n" + components.RenderError(v.err)
	}

	if len(v.images) == 0 {
		return "\n  No custom images found in this project.\n  Press 'esc' to go back."
	}

	// Help text for actions
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: details • .: actions • c: create disk • D: delete • /: filter • r: refresh • esc: back")

	mainContent := v.table.View() + help

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
func (v *ImagesView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// GetComputeClient returns the compute client for reuse in detail views
func (v *ImagesView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

// HasTextInputFocused returns true if the table filter is active.
// Used to prevent global hotkeys (like 'q' for quit) from triggering while typing.
func (v *ImagesView) HasTextInputFocused() bool {
	return v.table.HasTextInputFocused()
}

// SetContext updates the view with shared program context.
// Reads dimensions from the context for consistent sizing.
func (v *ImagesView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.table.SetSize(ctx.ContentWidth, ctx.ContentHeight-2)
}
