package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	btable "github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/table"
)

const defaultPageSize = 100

// objectKeyMap defines object-specific key bindings
type objectKeyMap struct {
	Enter    key.Binding
	Refresh  key.Binding
	NextPage key.Binding
	PrevPage key.Binding
}

func defaultObjectKeyMap() objectKeyMap {
	return objectKeyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		NextPage: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next page"),
		),
		PrevPage: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "prev page"),
		),
	}
}

// Table column definitions for objects
func objectColumns() []btable.Column {
	return []btable.Column{
		{Title: "Name", Width: 40},
		{Title: "Size", Width: 12},
		{Title: "Content Type", Width: 20},
		{Title: "Modified", Width: 12},
	}
}

// ObjectsView displays and manages objects within a bucket using a table format
type ObjectsView struct {
	storageClient *gcp.StorageClient
	bucketName    string
	currentPrefix string   // Current folder path (e.g., "folder1/folder2/")
	prefixStack   []string // Navigation history for back functionality
	table         table.Model
	spinner       spinner.Model
	loading       bool
	err           error
	width         int
	height        int
	objects       []gcp.StorageObject
	keys          objectKeyMap

	// Pagination state
	currentPage      int
	nextPageToken    string   // Token for loading next page
	currentPageToken string   // Token used to load current page (empty for first page)
	pageTokenHistory []string // History of page tokens used (for back navigation)
	hasMore          bool
	totalLoaded      int // Total objects loaded across pages
}

// NewObjectsView creates a new objects view with table display
func NewObjectsView(bucketName string, storageClient *gcp.StorageClient) *ObjectsView {
	title := fmt.Sprintf("Bucket: %s", bucketName)
	t := table.New(objectColumns(), title)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	return &ObjectsView{
		storageClient:    storageClient,
		bucketName:       bucketName,
		currentPrefix:    "",
		prefixStack:      make([]string, 0),
		table:            t,
		spinner:          s,
		loading:          true,
		keys:             defaultObjectKeyMap(),
		currentPage:      1,
		pageTokenHistory: make([]string, 0),
	}
}

// Init initializes the view and starts loading objects
func (v *ObjectsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.loadObjects(""), // Load first page
	)
}

// loadObjects fetches objects from the bucket
func (v *ObjectsView) loadObjects(pageToken string) tea.Cmd {
	return func() tea.Msg {
		result, err := v.storageClient.ListObjects(
			context.Background(),
			v.bucketName,
			v.currentPrefix,
			pageToken,
			defaultPageSize,
		)
		if err != nil {
			return objectsErrorMsg{err: err}
		}
		return objectsLoadedMsg{
			objects:   result.Objects,
			nextToken: result.NextToken,
			hasMore:   result.HasMore,
		}
	}
}

// Message types for objects view
type objectsLoadedMsg struct {
	objects   []gcp.StorageObject
	nextToken string
	hasMore   bool
}

type objectsErrorMsg struct {
	err error
}

// ObjectsBackMsg signals to return to buckets view (exported for app.go)
type ObjectsBackMsg struct{}

// objectToRow converts a GCS object to a table row
func objectToRow(obj gcp.StorageObject) table.Row {
	var name, size, contentType, modified string

	if obj.IsFolder {
		name = "📁 " + obj.DisplayName + "/"
		size = "-"
		contentType = "Folder"
		modified = "-"
	} else {
		name = "📄 " + obj.DisplayName
		size = gcp.FormatSize(obj.Size)
		contentType = obj.ContentType
		if contentType == "" {
			contentType = "unknown"
		}
		modified = obj.Updated.Format("2006-01-02")
	}

	return table.Row{
		Data:        []string{name, size, contentType, modified},
		FilterValue: obj.DisplayName + " " + contentType,
		ID:          obj.Name, // Full path name for lookup
	}
}

// Update handles messages for the objects view
func (v *ObjectsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case objectsLoadedMsg:
		v.loading = false
		v.objects = msg.objects
		v.nextPageToken = msg.nextToken
		v.hasMore = msg.hasMore
		v.totalLoaded = len(msg.objects)

		// Convert to table rows
		rows := make([]table.Row, len(msg.objects))
		for i, obj := range msg.objects {
			rows[i] = objectToRow(obj)
		}
		v.table.SetRows(rows)
		return nil

	case objectsErrorMsg:
		v.loading = false
		v.err = msg.err
		return nil

	case spinner.TickMsg:
		if v.loading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case tea.KeyMsg:
		// Don't handle keys during loading
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
			// Navigate into folder on Enter
			if row := v.table.SelectedRow(); row != nil {
				obj := v.findObjectByName(row.ID)
				if obj != nil && obj.IsFolder {
					// Push current prefix to stack and navigate into folder
					v.prefixStack = append(v.prefixStack, v.currentPrefix)
					v.currentPrefix = obj.Name
					v.resetPagination()
					v.loading = true
					return tea.Batch(v.spinner.Tick, v.loadObjects(""))
				}
				// For files, could open details view in the future
			}

		case key.Matches(msg, v.keys.Refresh):
			v.resetPagination()
			v.loading = true
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.loadObjects(""))

		case key.Matches(msg, v.keys.NextPage):
			if v.hasMore && v.nextPageToken != "" {
				// Save current page token to history before navigating forward
				v.pageTokenHistory = append(v.pageTokenHistory, v.currentPageToken)
				v.currentPageToken = v.nextPageToken
				v.currentPage++
				v.loading = true
				return tea.Batch(v.spinner.Tick, v.loadObjects(v.currentPageToken))
			}

		case key.Matches(msg, v.keys.PrevPage):
			if v.currentPage > 1 && len(v.pageTokenHistory) > 0 {
				// Pop previous page token from history
				prevToken := v.pageTokenHistory[len(v.pageTokenHistory)-1]
				v.pageTokenHistory = v.pageTokenHistory[:len(v.pageTokenHistory)-1]
				v.currentPageToken = prevToken
				v.currentPage--
				v.loading = true
				return tea.Batch(v.spinner.Tick, v.loadObjects(prevToken))
			}
		}
	}

	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

// findObjectByName looks up an object by its full name
func (v *ObjectsView) findObjectByName(name string) *gcp.StorageObject {
	for _, obj := range v.objects {
		if obj.Name == name {
			return &obj
		}
	}
	return nil
}

// HandleBack handles ESC key - returns true if handled internally (went up a folder)
func (v *ObjectsView) HandleBack() (handled bool, cmd tea.Cmd) {
	// If we're in a subfolder, go up
	if len(v.prefixStack) > 0 {
		v.currentPrefix = v.prefixStack[len(v.prefixStack)-1]
		v.prefixStack = v.prefixStack[:len(v.prefixStack)-1]
		v.resetPagination()
		v.loading = true
		return true, tea.Batch(v.spinner.Tick, v.loadObjects(""))
	}
	// At root, signal to return to buckets view
	return false, nil
}

// resetPagination resets pagination state
func (v *ObjectsView) resetPagination() {
	v.currentPage = 1
	v.nextPageToken = ""
	v.currentPageToken = ""
	v.pageTokenHistory = make([]string, 0)
	v.hasMore = false
}

// buildTitle builds the title showing current path
func (v *ObjectsView) buildTitle() string {
	title := fmt.Sprintf("Bucket: %s", v.bucketName)
	if v.currentPrefix != "" {
		// Show path in title
		path := strings.TrimSuffix(v.currentPrefix, "/")
		title = fmt.Sprintf("Bucket: %s / %s", v.bucketName, path)
	}
	return title
}

// View renders the objects view
func (v *ObjectsView) View() string {
	if v.loading {
		loadingMsg := "Loading objects..."
		if v.currentPrefix != "" {
			loadingMsg = fmt.Sprintf("Loading %s...", v.currentPrefix)
		}
		return fmt.Sprintf("\n  %s %s\n", v.spinner.View(), loadingMsg)
	}

	if v.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
		return errStyle.Render(fmt.Sprintf("\n  Error: %v\n\n  Press 'r' to retry", v.err))
	}

	if len(v.objects) == 0 {
		msg := "This bucket is empty."
		if v.currentPrefix != "" {
			msg = "This folder is empty."
		}
		return fmt.Sprintf("\n  %s\n  Press 'esc' to go back.", msg)
	}

	// Build pagination info
	pageInfo := ""
	if v.hasMore || v.currentPage > 1 {
		pageInfo = fmt.Sprintf(" • Page %d", v.currentPage)
		if v.hasMore {
			pageInfo += " (more available)"
		}
	}

	// Status line with path and pagination
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	status := statusStyle.Render(fmt.Sprintf("  %d items%s", len(v.objects), pageInfo))

	// Help text for actions
	help := statusStyle.Render("\n  enter: open • n/p: next/prev page • /: filter • r: refresh • esc: back")

	// Build title with current path
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#4285F4")).
		MarginBottom(1)

	return titleStyle.Render(v.buildTitle()) + "\n" + v.table.View() + "\n" + status + help
}

// SetSize updates the view dimensions
func (v *ObjectsView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.table.SetSize(width, height-8) // Extra space for title, status and help
}

// GetCurrentPath returns the current folder path being browsed
func (v *ObjectsView) GetCurrentPath() string {
	return v.currentPrefix
}

// GetBucketName returns the bucket name
func (v *ObjectsView) GetBucketName() string {
	return v.bucketName
}
