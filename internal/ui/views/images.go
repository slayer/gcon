package views

import (
	gocontext "context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	btable "github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/mouse"
	"github.com/slayer/gcon/internal/ui/symbols"
)

// ImagesView displays Compute Engine disk images in a table format
type ImagesView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	ctx           *context.ProgramContext // Shared context for dimensions and styles
	table         table.Model
	spinner       spinner.Model
	loading       bool
	err           error
	images        []gcp.Image
	keys          imageKeyMap
}

// imageKeyMap defines image-specific key bindings
type imageKeyMap struct {
	Enter   key.Binding
	Refresh key.Binding
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
	}
}

// Table column definitions
func imageColumns() []btable.Column {
	return []btable.Column{
		{Title: "Name", Width: 28},
		{Title: "Created By", Width: 20},
		{Title: "Location", Width: 12},
		{Title: "Disk Size", Width: 10},
		{Title: "Archive Size", Width: 12},
		{Title: "Family", Width: 18},
	}
}

// NewImagesView creates a new images view with table display
func NewImagesView(projectID string) *ImagesView {
	title := fmt.Sprintf("Disk Images - %s", projectID)
	t := table.New(imageColumns(), title)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	return &ImagesView{
		projectID: projectID,
		table:     t,
		spinner:   s,
		loading:   true,
		keys:      defaultImageKeyMap(),
	}
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

	case spinner.TickMsg:
		if v.loading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case tea.KeyMsg:
		// Don't handle custom keys during loading
		if v.loading {
			return nil
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

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.loadImages())
		}
	}

	// Update table for navigation
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
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
		return v.renderLoading("Initializing Compute Engine client...")
	}

	if v.loading {
		return v.renderLoading("Loading disk images...")
	}

	if v.err != nil {
		return "\n" + components.RenderError(v.err)
	}

	if len(v.images) == 0 {
		return "\n  No custom images found in this project.\n  Press 'esc' to go back."
	}

	// Help text for actions
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: details • /: filter • r: refresh • esc: back")

	return v.table.View() + help
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
	v.table.SetSize(ctx.ContentWidth, ctx.ContentHeight-6)
}

// renderLoading renders a loading message
func (v *ImagesView) renderLoading(msg string) string {
	return fmt.Sprintf("\n  %s %s\n", v.spinner.View(), msg)
}

// UpdateRegions delegates to the table component.
// Implements the components.Clickable interface.
func (v *ImagesView) UpdateRegions(offsetX, offsetY int) {
	if clickable, ok := interface{}(&v.table).(components.Clickable); ok {
		clickable.UpdateRegions(offsetX, offsetY)
	}
}

// GetRegions delegates to the table component.
// Implements the components.Clickable interface.
func (v *ImagesView) GetRegions() []mouse.Region {
	if clickable, ok := interface{}(&v.table).(components.Clickable); ok {
		return clickable.GetRegions()
	}
	return nil
}

// HandleRegionClick delegates to the table component.
// Implements the components.Clickable interface.
func (v *ImagesView) HandleRegionClick(regionID string) tea.Cmd {
	if clickable, ok := interface{}(&v.table).(components.Clickable); ok {
		return clickable.HandleRegionClick(regionID)
	}
	return nil
}
