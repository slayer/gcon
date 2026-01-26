package views

import (
	gocontext "context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
	"github.com/slayer/gcon/internal/ui/components/actionmenu"
	"github.com/slayer/gcon/internal/ui/components/confirm"
	"github.com/slayer/gcon/internal/ui/components/tabs"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/focus"
	"github.com/slayer/gcon/internal/ui/overlay"
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// Max preview size: 500KB
const maxPreviewBytes = 500 * 1024

// Tab IDs for object details view
const (
	objectTabIDDetails = "details"
	objectTabIDPreview = "preview"
)

// Focus region IDs for object details view
const (
	objectRegionIDTabs     = "tabs"
	objectRegionIDViewport = "viewport"
)

// Message types for object details view
type objectMetadataLoadedMsg struct {
	metadata *gcp.ObjectMetadata
}

type objectMetadataErrorMsg struct {
	err error
}

type objectPreviewLoadedMsg struct {
	content   []byte
	truncated bool
}

type objectPreviewErrorMsg struct {
	err error
}

type objectDownloadCompleteMsg struct {
	path string
	err  error
}

type objectDeleteCompleteMsg struct {
	err error
}

type objectOpenCompleteMsg struct {
	err error
}

// ObjectAction indicates what action to perform when opening object details
type ObjectAction string

const (
	ObjectActionView    ObjectAction = ""        // Default: view details
	ObjectActionPreview ObjectAction = "preview" // Open preview tab
	ObjectActionOpen    ObjectAction = "open"    // Download and open with default app
)

// ObjectSelectedMsg signals that an object was selected for viewing details
type ObjectSelectedMsg struct {
	Object gcp.StorageObject
	Action ObjectAction // Optional action to perform (preview, open)
}

// ObjectDeletedMsg signals that an object was deleted
type ObjectDeletedMsg struct {
	ObjectName string
}

// objectDetailsKeyMap defines object details key bindings
type objectDetailsKeyMap struct {
	Preview    key.Binding
	Open       key.Binding
	Download   key.Binding
	Delete     key.Binding
	ActionMenu key.Binding
	Refresh    key.Binding
}

func defaultObjectDetailsKeyMap() objectDetailsKeyMap {
	return objectDetailsKeyMap{
		Preview: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "preview"),
		),
		Open: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open"),
		),
		Download: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "download"),
		),
		Delete: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "delete"),
		),
		ActionMenu: key.NewBinding(
			key.WithKeys("."),
			key.WithHelp(".", "actions"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
	}
}

// ObjectDetailsView displays comprehensive GCS object information
type ObjectDetailsView struct {
	storageClient *gcp.StorageClient
	bucketName    string
	objectName    string
	displayName   string
	ctx           *context.ProgramContext
	metadata      *gcp.ObjectMetadata
	spinner       spinner.Model
	loading       bool
	err           error
	width         int
	height        int
	keys          objectDetailsKeyMap
	ready         bool

	// Tab navigation
	tabs         *tabs.Tabs
	tabViewports []viewport.Model

	// Focus management
	focusMgr *focus.Manager

	// Action menu
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Preview state
	previewContent   []byte
	previewTruncated bool
	previewLoading   bool
	previewError     error

	// Delete confirmation
	showDeleteConfirm bool
	deleteConfirm     *confirm.ConfirmDialog
	deleting          bool

	// Pending action to perform after metadata loads
	pendingAction ObjectAction
}

// NewObjectDetailsView creates a new object details view
func NewObjectDetailsView(bucketName, objectName, displayName string, storageClient *gcp.StorageClient, action ObjectAction) *ObjectDetailsView {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	// Initialize tabs
	tabsComponent := tabs.New([]tabs.Tab{
		{ID: objectTabIDDetails, Label: "Details"},
		{ID: objectTabIDPreview, Label: "Preview"},
	})

	// Initialize focus manager
	fm := focus.NewManager()
	fm.SetRegions([]focus.Region{
		focus.NewRegion(objectRegionIDTabs, focus.RegionTabs, "Tabs"),
		focus.NewRegion(objectRegionIDViewport, focus.RegionViewport, "Content"),
	})

	return &ObjectDetailsView{
		storageClient: storageClient,
		bucketName:    bucketName,
		objectName:    objectName,
		displayName:   displayName,
		spinner:       s,
		loading:       true,
		keys:          defaultObjectDetailsKeyMap(),
		tabs:          tabsComponent,
		tabViewports:  make([]viewport.Model, 2),
		focusMgr:      fm,
		pendingAction: action,
	}
}

// Init initializes the view and starts loading metadata
func (v *ObjectDetailsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.loadMetadata(),
	)
}

// loadMetadata fetches object metadata from GCS
func (v *ObjectDetailsView) loadMetadata() tea.Cmd {
	return func() tea.Msg {
		metadata, err := v.storageClient.GetObjectMetadata(
			gocontext.Background(),
			v.bucketName,
			v.objectName,
		)
		if err != nil {
			return objectMetadataErrorMsg{err: err}
		}
		return objectMetadataLoadedMsg{metadata: metadata}
	}
}

// loadPreview fetches object content for preview
// Takes objectSize as parameter to avoid race condition with v.metadata access
func (v *ObjectDetailsView) loadPreview(objectSize int64) tea.Cmd {
	return func() tea.Msg {
		content, err := v.storageClient.GetObjectContent(
			gocontext.Background(),
			v.bucketName,
			v.objectName,
			maxPreviewBytes,
		)
		if err != nil {
			return objectPreviewErrorMsg{err: err}
		}

		truncated := objectSize > maxPreviewBytes
		return objectPreviewLoadedMsg{content: content, truncated: truncated}
	}
}

// Update handles messages for the object details view
//nolint:gocognit // Bubble Tea Update pattern - complexity 69
func (v *ObjectDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case objectMetadataLoadedMsg:
		v.loading = false
		v.metadata = msg.metadata
		v.updateViewportContent()

		// Handle pending action after metadata loads
		if v.pendingAction != "" {
			action := v.pendingAction
			v.pendingAction = "" // Clear to prevent re-execution
			switch action {
			case ObjectActionPreview:
				// Switch to preview tab and load preview
				if v.isPreviewable() {
					v.tabs.SetActiveByID(objectTabIDPreview)
					v.previewLoading = true
					return v.loadPreview(msg.metadata.Size)
				}
			case ObjectActionOpen:
				// Immediately open with default app
				return v.openObject()
			}
		}
		return nil

	case objectMetadataErrorMsg:
		v.loading = false
		v.err = msg.err
		return nil

	case objectPreviewLoadedMsg:
		v.previewLoading = false
		v.previewContent = msg.content
		v.previewTruncated = msg.truncated
		v.updateViewportContent()
		return nil

	case objectPreviewErrorMsg:
		v.previewLoading = false
		v.previewError = msg.err
		v.updateViewportContent()
		return nil

	case objectDownloadCompleteMsg:
		if msg.err != nil {
			v.err = msg.err
		}
		return nil

	case objectOpenCompleteMsg:
		if msg.err != nil {
			v.err = msg.err
		}
		return nil

	case objectDeleteCompleteMsg:
		v.deleting = false
		if msg.err != nil {
			v.err = msg.err
			return nil
		}
		// Signal that object was deleted so parent can navigate back
		return func() tea.Msg {
			return ObjectDeletedMsg{ObjectName: v.objectName}
		}

	case spinner.TickMsg:
		if v.loading || v.previewLoading || v.deleting {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case tabs.TabChangedMsg:
		v.updateViewportContent()
		// Load preview when switching to preview tab
		if msg.TabID == objectTabIDPreview && v.previewContent == nil && !v.previewLoading {
			if v.isPreviewable() && v.metadata != nil {
				v.previewLoading = true
				return v.loadPreview(v.metadata.Size)
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
		if v.showDeleteConfirm {
			v.showDeleteConfirm = false
			v.deleteConfirm = nil
			v.deleting = true
			return tea.Batch(v.spinner.Tick, v.deleteObject())
		}
		return nil

	case confirm.CancelMsg:
		if v.showDeleteConfirm {
			v.showDeleteConfirm = false
			v.deleteConfirm = nil
		}
		return nil

	case focus.FocusChangedMsg:
		v.updateViewportContent()
		return nil

	case tea.KeyMsg:
		// Handle delete confirmation dialog
		if v.showDeleteConfirm && v.deleteConfirm != nil {
			return v.deleteConfirm.Update(msg)
		}

		// Route keys to action menu when open
		if v.menuOpen {
			return v.actionMenu.Update(msg)
		}

		// Handle Tab/Shift+Tab for cycling between focus regions
		if focusMsg := v.focusMgr.HandleKey(msg); focusMsg != nil {
			v.updateViewportContent()
			return func() tea.Msg { return focusMsg }
		}

		// Route keys based on focused region
		switch v.focusMgr.ActiveType() {
		case focus.RegionTabs:
			if tabs.HandleKey(msg) {
				return v.tabs.Update(msg)
			}

		case focus.RegionViewport:
			activeIdx := v.tabs.ActiveIndex()
			if activeIdx >= 0 && activeIdx < len(v.tabViewports) {
				var cmd tea.Cmd
				v.tabViewports[activeIdx], cmd = v.tabViewports[activeIdx].Update(msg)
				return cmd
			}
		}

		// View-specific action keys
		switch {
		case key.Matches(msg, v.keys.ActionMenu):
			if v.metadata != nil {
				v.actionMenu = actionmenu.New("Object Actions", v.buildActions())
				v.menuOpen = true
			}
			return nil

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			v.previewContent = nil
			v.previewError = nil
			return tea.Batch(v.spinner.Tick, v.loadMetadata())

		case key.Matches(msg, v.keys.Preview):
			if v.metadata != nil && v.isPreviewable() {
				v.tabs.SetActiveByID(objectTabIDPreview)
				if v.previewContent == nil && !v.previewLoading {
					v.previewLoading = true
					return v.loadPreview(v.metadata.Size)
				}
			}
			return nil

		case key.Matches(msg, v.keys.Open):
			if v.metadata != nil {
				return v.openObject()
			}
			return nil

		case key.Matches(msg, v.keys.Download):
			if v.metadata != nil {
				return v.downloadObject()
			}
			return nil

		case key.Matches(msg, v.keys.Delete):
			if v.metadata != nil {
				v.showDeleteConfirm = true
				v.deleteConfirm = v.createDeleteConfirmDialog()
				return nil
			}
			return nil
		}
	}

	return nil
}

// buildActions creates the action menu items
func (v *ObjectDetailsView) buildActions() []actionmenu.Action {
	previewable := v.isPreviewable()

	return []actionmenu.Action{
		{Key: 'v', Label: "Preview", Enabled: previewable},
		{Key: 'o', Label: "Open", Enabled: true},
		{Key: 'd', Label: "Download", Enabled: true},
		{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true},
		{Key: 'r', Label: "Refresh", Enabled: true},
	}
}

// executeAction performs the action selected from the menu
func (v *ObjectDetailsView) executeAction(actionKey rune) tea.Cmd {
	switch actionKey {
	case 'v':
		if v.isPreviewable() && v.metadata != nil {
			v.tabs.SetActiveByID(objectTabIDPreview)
			if v.previewContent == nil && !v.previewLoading {
				v.previewLoading = true
				return v.loadPreview(v.metadata.Size)
			}
		}
	case 'o':
		return v.openObject()
	case 'd':
		return v.downloadObject()
	case 'D':
		v.showDeleteConfirm = true
		v.deleteConfirm = v.createDeleteConfirmDialog()
	case 'r':
		v.loading = true
		v.err = nil
		return tea.Batch(v.spinner.Tick, v.loadMetadata())
	}
	return nil
}

// isPreviewable returns true if the object can be previewed
func (v *ObjectDetailsView) isPreviewable() bool {
	if v.metadata == nil {
		return false
	}
	// Delegate to shared helper function
	return isFilePreviewable(v.metadata.ContentType, v.metadata.Size, v.metadata.Name)
}

// downloadObject downloads the object to the current directory
func (v *ObjectDetailsView) downloadObject() tea.Cmd {
	return func() tea.Msg {
		cwd, err := os.Getwd()
		if err != nil {
			return objectDownloadCompleteMsg{err: fmt.Errorf("failed to get working directory: %w", err)}
		}

		localPath := filepath.Join(cwd, v.displayName)

		// Check if file already exists
		if _, err := os.Stat(localPath); err == nil {
			// File exists - append a suffix
			baseName := v.displayName
			ext := filepath.Ext(baseName)
			nameWithoutExt := strings.TrimSuffix(baseName, ext)

			// Try to find an available filename
			for i := 1; i < 1000; i++ {
				newPath := filepath.Join(cwd, fmt.Sprintf("%s (%d)%s", nameWithoutExt, i, ext))
				if _, err := os.Stat(newPath); os.IsNotExist(err) {
					localPath = newPath
					break
				}
			}
		}

		err = v.storageClient.DownloadObject(
			gocontext.Background(),
			v.bucketName,
			v.objectName,
			localPath,
			nil, // No progress callback for simplicity
		)

		if err != nil {
			return objectDownloadCompleteMsg{err: err}
		}
		return objectDownloadCompleteMsg{path: localPath}
	}
}

// openObject downloads the object to a temp file and opens it with the default app
// Note: Temp files are left in the OS temp directory and rely on OS cleanup.
// This is acceptable as the files remain accessible while the app that opened them runs.
// Users can manually clean up /tmp/gcon-* files if needed.
func (v *ObjectDetailsView) openObject() tea.Cmd {
	return func() tea.Msg {
		// Create temp file with appropriate extension
		ext := filepath.Ext(v.displayName)
		tempFile, err := os.CreateTemp("", "gcon-*"+ext)
		if err != nil {
			return objectOpenCompleteMsg{err: fmt.Errorf("failed to create temp file: %w", err)}
		}
		tempPath := tempFile.Name()
		_ = tempFile.Close()

		// Download to temp file
		err = v.storageClient.DownloadObject(
			gocontext.Background(),
			v.bucketName,
			v.objectName,
			tempPath,
			nil,
		)
		if err != nil {
			// Clean up temp file on download failure
			_ = os.Remove(tempPath)
			return objectOpenCompleteMsg{err: err}
		}

		// Open with default app based on OS
		// Note: exec.Command automatically handles proper argument quoting/escaping
		// for spaces and special characters in paths across all platforms
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", tempPath) // #nosec G204 -- Opening user-downloaded file with system default app
		case "linux":
			cmd = exec.Command("xdg-open", tempPath) // #nosec G204 -- Opening user-downloaded file with system default app
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", tempPath) // #nosec G204 -- Opening user-downloaded file with system default app
		default:
			_ = os.Remove(tempPath)
			return objectOpenCompleteMsg{err: fmt.Errorf("%w: %s", uierrors.ErrUnsupportedOS, runtime.GOOS)}
		}

		if err := cmd.Start(); err != nil {
			_ = os.Remove(tempPath)
			return objectOpenCompleteMsg{err: fmt.Errorf("failed to open file: %w", err)}
		}

		return objectOpenCompleteMsg{}
	}
}

// deleteObject deletes the object from GCS
func (v *ObjectDetailsView) deleteObject() tea.Cmd {
	return func() tea.Msg {
		err := v.storageClient.DeleteObject(
			gocontext.Background(),
			v.bucketName,
			v.objectName,
		)
		return objectDeleteCompleteMsg{err: err}
	}
}

// createDeleteConfirmDialog creates the deletion confirmation dialog
func (v *ObjectDetailsView) createDeleteConfirmDialog() *confirm.ConfirmDialog {
	title := "Delete Object"
	message := fmt.Sprintf("Are you sure you want to delete '%s'?", v.displayName)

	dialog := confirm.New(title, message, nil)
	dialog.SetSize(v.width-20, 10)
	return dialog
}

// View renders the object details view
func (v *ObjectDetailsView) View() string {
	if v.loading {
		return v.renderLoading("Loading object details...")
	}

	if v.deleting {
		return v.renderLoading("Deleting object...")
	}

	if v.err != nil {
		return v.renderLoading(fmt.Sprintf("Error: %v\n  Press 'esc' to go back", v.err))
	}

	if v.metadata == nil {
		return v.renderLoading("No object details available.\n  Press 'esc' to go back.")
	}

	if !v.ready {
		return v.renderLoading("Initializing view...")
	}

	// Render tab bar
	tabBar := "  " + v.tabs.View()

	// Get active tab viewport
	activeIdx := v.tabs.ActiveIndex()
	var viewportContent string
	var scrollPercent float64
	if activeIdx >= 0 && activeIdx < len(v.tabViewports) {
		viewportContent = v.tabViewports[activeIdx].View()
		scrollPercent = v.tabViewports[activeIdx].ScrollPercent()
	}

	// Help text
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	scrollInfo := scrollStyle.Render(fmt.Sprintf("%.0f%%", scrollPercent*100))

	helpText := v.buildHelpText()
	help := helpStyle.Render(helpText) + " " + scrollInfo

	mainContent := tabBar + "\n" + viewportContent + help

	// Overlay action menu if open
	if v.menuOpen && v.actionMenu != nil {
		return v.renderWithActionMenu(mainContent)
	}

	// Overlay delete confirmation if shown
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.renderWithDeleteConfirm(mainContent)
	}

	return mainContent
}

// renderWithActionMenu overlays the action menu centered on the content
func (v *ObjectDetailsView) renderWithActionMenu(content string) string {
	menuView := v.actionMenu.View()
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, menuView, v.width, contentHeight)
}

// renderWithDeleteConfirm overlays the delete confirmation dialog
func (v *ObjectDetailsView) renderWithDeleteConfirm(content string) string {
	dialogView := v.deleteConfirm.View()
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, dialogView, v.width, contentHeight)
}

// SetContext updates the view with shared program context
func (v *ObjectDetailsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// IsMenuOpen returns true if the action menu is currently open
func (v *ObjectDetailsView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
}

// applySize applies the given dimensions to the viewports
func (v *ObjectDetailsView) applySize(width, height int) {
	// Reserve space for tab bar and footer
	viewportHeight := height - 5
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	if !v.ready {
		for i := range v.tabViewports {
			v.tabViewports[i] = viewport.New(width, viewportHeight)
			v.tabViewports[i].Style = lipgloss.NewStyle().Padding(0, 2)
		}
		v.ready = true
	} else {
		for i := range v.tabViewports {
			v.tabViewports[i].Width = width
			v.tabViewports[i].Height = viewportHeight
		}
	}

	if v.metadata != nil {
		v.updateViewportContent()
	}
}

// updateViewportContent renders the content for the active tab's viewport
func (v *ObjectDetailsView) updateViewportContent() {
	if v.metadata == nil || !v.ready {
		return
	}

	activeIdx := v.tabs.ActiveIndex()
	if activeIdx < 0 || activeIdx >= len(v.tabViewports) {
		return
	}

	var content string
	switch v.tabs.ActiveTab().ID {
	case objectTabIDDetails:
		content = v.renderDetailsTab()
	case objectTabIDPreview:
		content = v.renderPreviewTab()
	default:
		content = v.renderDetailsTab()
	}

	v.tabViewports[activeIdx].SetContent(content)
}

// renderDetailsTab generates the Details tab content
func (v *ObjectDetailsView) renderDetailsTab() string {
	m := v.metadata
	var b strings.Builder

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).MarginTop(1)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Width(24)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B6B6B"))
	urlStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8AB4F8")) // Light blue for URLs

	// Header
	b.WriteString(titleStyle.Render(fmt.Sprintf("Object: %s", v.displayName)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(v.width-4, 60)))
	b.WriteString("\n\n")

	// Overview section
	b.WriteString(sectionStyle.Render("Overview"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Name", m.Name))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Bucket", m.Bucket))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Type", defaultIfEmpty(m.ContentType, "Unknown")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Size", gcp.FormatSize(m.Size)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Created", timeutil.FormatTimestamp(m.Created.Format("2006-01-02T15:04:05Z07:00"))))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Last modified", timeutil.FormatTimestamp(m.Updated.Format("2006-01-02T15:04:05Z07:00"))))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Storage class", defaultIfEmpty(m.StorageClass, "Standard")))
	b.WriteString("\n")

	// URLs section
	b.WriteString(sectionStyle.Render("URLs"))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Public URL:") + " " + urlStyle.Render(m.PublicURL) + "\n")
	b.WriteString(labelStyle.Render("Authenticated:") + " " + urlStyle.Render(m.AuthenticatedURL) + "\n")
	b.WriteString(labelStyle.Render("gsutil URI:") + " " + urlStyle.Render(m.GsutilURI) + "\n")
	b.WriteString("\n")

	// Technical details section
	b.WriteString(sectionStyle.Render("Technical Details"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "MD5 Hash", defaultIfEmpty(m.MD5Hash, "—")))
	if m.CRC32C != 0 {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "CRC32C", fmt.Sprintf("%08X", m.CRC32C)))
	} else {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "CRC32C", "—"))
	}
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "ETag", defaultIfEmpty(m.Etag, "—")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Generation", fmt.Sprintf("%d", m.Generation)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Metageneration", fmt.Sprintf("%d", m.Metageneration)))
	b.WriteString("\n")

	// Content headers section (if any are set)
	if m.CacheControl != "" || m.ContentDisposition != "" || m.ContentEncoding != "" || m.ContentLanguage != "" {
		b.WriteString(sectionStyle.Render("Content Headers"))
		b.WriteString("\n")
		if m.CacheControl != "" {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Cache-Control", m.CacheControl))
		}
		if m.ContentDisposition != "" {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Content-Disposition", m.ContentDisposition))
		}
		if m.ContentEncoding != "" {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Content-Encoding", m.ContentEncoding))
		}
		if m.ContentLanguage != "" {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Content-Language", m.ContentLanguage))
		}
		b.WriteString("\n")
	}

	// Custom metadata section
	if len(m.CustomMetadata) > 0 {
		b.WriteString(sectionStyle.Render("Custom Metadata"))
		b.WriteString("\n")
		// Sort keys for consistent display
		keys := make([]string, 0, len(m.CustomMetadata))
		for k := range m.CustomMetadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(fmt.Sprintf("    %s: %s\n", k, m.CustomMetadata[k]))
		}
		b.WriteString("\n")
	}

	// Owner section
	if m.Owner != "" {
		b.WriteString(sectionStyle.Render("Access"))
		b.WriteString("\n")
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Owner", m.Owner))
		b.WriteString("\n")
	}

	return b.String()
}

// renderPreviewTab generates the Preview tab content
func (v *ObjectDetailsView) renderPreviewTab() string {
	var b strings.Builder

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))

	// Header
	b.WriteString(titleStyle.Render(fmt.Sprintf("Preview: %s", v.displayName)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(v.width-4, 60)))
	b.WriteString("\n\n")

	// Check if previewable
	if !v.isPreviewable() {
		b.WriteString(warningStyle.Render("  Preview not available"))
		b.WriteString("\n\n")
		if v.metadata != nil && v.metadata.Size > maxPreviewBytes {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  File is too large for preview (max %s)", gcp.FormatSize(maxPreviewBytes))))
		} else {
			b.WriteString(mutedStyle.Render("  File type cannot be previewed"))
		}
		b.WriteString("\n\n")
		b.WriteString(mutedStyle.Render("  Use 'o' to open with default application or 'd' to download"))
		return b.String()
	}

	// Show loading state
	if v.previewLoading {
		b.WriteString(fmt.Sprintf("  %s Loading preview...\n", v.spinner.View()))
		return b.String()
	}

	// Show error state
	if v.previewError != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  Error loading preview: %s", v.previewError.Error())))
		b.WriteString("\n\n")
		b.WriteString(mutedStyle.Render("  Press 'r' to retry"))
		return b.String()
	}

	// Show preview content
	if v.previewContent != nil {
		if v.previewTruncated {
			b.WriteString(warningStyle.Render(fmt.Sprintf("  Showing first %s of %s", gcp.FormatSize(maxPreviewBytes), gcp.FormatSize(v.metadata.Size))))
			b.WriteString("\n")
			b.WriteString(strings.Repeat("─", min(v.width-4, 60)))
			b.WriteString("\n\n")
		}

		// Render content with line numbers
		lines := strings.Split(string(v.previewContent), "\n")
		lineNumStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B6B6B"))
		for i, line := range lines {
			lineNum := fmt.Sprintf("%4d", i+1)
			b.WriteString(lineNumStyle.Render(lineNum) + " │ " + line + "\n")
		}
	} else {
		b.WriteString(mutedStyle.Render("  Press 'v' to load preview"))
	}

	return b.String()
}

// buildHelpText generates context-sensitive help text
func (v *ObjectDetailsView) buildHelpText() string {
	bindings := focus.HelpForRegion(v.focusMgr.ActiveType(), "")
	helpStr := focus.FormatHelp(bindings)
	return "\n  " + helpStr + " • v:preview • o:open • d:download • D:delete • .:menu"
}

// renderLoading renders a loading message
func (v *ObjectDetailsView) renderLoading(msg string) string {
	return fmt.Sprintf("\n  %s %s\n", v.spinner.View(), msg)
}

// GetStorageClient returns the storage client for reuse
func (v *ObjectDetailsView) GetStorageClient() *gcp.StorageClient {
	return v.storageClient
}

// GetBucketName returns the bucket name
func (v *ObjectDetailsView) GetBucketName() string {
	return v.bucketName
}
