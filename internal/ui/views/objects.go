package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
)

const defaultPageSize = 100

// objectItem implements list.Item for GCS objects
type objectItem struct {
	object gcp.StorageObject
}

func (i objectItem) Title() string {
	if i.object.IsFolder {
		return fmt.Sprintf("📁 %s/", i.object.DisplayName)
	}
	return fmt.Sprintf("📄 %s", i.object.DisplayName)
}

func (i objectItem) Description() string {
	if i.object.IsFolder {
		return "— • Folder"
	}
	size := gcp.FormatSize(i.object.Size)
	contentType := i.object.ContentType
	if contentType == "" {
		contentType = "unknown"
	}
	updated := i.object.Updated.Format("2006-01-02")
	return fmt.Sprintf("%s • %s • %s", size, contentType, updated)
}

func (i objectItem) FilterValue() string {
	return i.object.DisplayName + " " + i.object.ContentType
}

// ObjectsView displays and manages objects within a bucket
type ObjectsView struct {
	storageClient *gcp.StorageClient
	bucketName    string
	currentPrefix string   // Current folder path (e.g., "folder1/folder2/")
	prefixStack   []string // Navigation history for back functionality
	list          list.Model
	spinner       spinner.Model
	loading       bool
	err           error
	width         int
	height        int
	objects       []gcp.StorageObject
	keys          objectKeyMap

	// Pagination state
	currentPage    int
	pageToken      string   // Token for next page
	prevPageTokens []string // Stack of previous page tokens
	hasMore        bool
	totalLoaded    int // Total objects loaded across pages
}

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

// NewObjectsView creates a new objects view
func NewObjectsView(bucketName string, storageClient *gcp.StorageClient) *ObjectsView {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#4285F4")).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#CCCCCC")).
		Background(lipgloss.Color("#4285F4"))

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = fmt.Sprintf("📦 %s", bucketName)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#4285F4")).
		Padding(0, 1)

	// Add help keys
	l.AdditionalShortHelpKeys = func() []key.Binding {
		km := defaultObjectKeyMap()
		return []key.Binding{km.Enter, km.Refresh, km.NextPage, km.PrevPage}
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	return &ObjectsView{
		storageClient:  storageClient,
		bucketName:     bucketName,
		currentPrefix:  "",
		prefixStack:    make([]string, 0),
		list:           l,
		spinner:        s,
		loading:        true,
		keys:           defaultObjectKeyMap(),
		currentPage:    1,
		prevPageTokens: make([]string, 0),
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

// Update handles messages for the objects view
func (v *ObjectsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case objectsLoadedMsg:
		v.loading = false
		v.objects = msg.objects
		v.pageToken = msg.nextToken
		v.hasMore = msg.hasMore
		v.totalLoaded = len(msg.objects)

		items := make([]list.Item, len(msg.objects))
		for i, obj := range msg.objects {
			items[i] = objectItem{object: obj}
		}
		v.list.SetItems(items)
		v.updateTitle()
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

		switch {
		case key.Matches(msg, v.keys.Enter):
			// Navigate into folder on Enter
			if item, ok := v.list.SelectedItem().(objectItem); ok {
				if item.object.IsFolder {
					// Push current prefix to stack and navigate into folder
					v.prefixStack = append(v.prefixStack, v.currentPrefix)
					v.currentPrefix = item.object.Name
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
			if v.hasMore && v.pageToken != "" {
				// Save empty token for first page to allow navigating back
				if v.currentPage == 1 {
					v.prevPageTokens = append(v.prevPageTokens, "")
				}
				v.prevPageTokens = append(v.prevPageTokens, v.pageToken)
				v.currentPage++
				v.loading = true
				return tea.Batch(v.spinner.Tick, v.loadObjects(v.pageToken))
			}

		case key.Matches(msg, v.keys.PrevPage):
			if v.currentPage > 1 && len(v.prevPageTokens) > 0 {
				// Pop the token that got us to current page
				v.prevPageTokens = v.prevPageTokens[:len(v.prevPageTokens)-1]
				// Get the previous page token
				prevToken := ""
				if len(v.prevPageTokens) > 0 {
					prevToken = v.prevPageTokens[len(v.prevPageTokens)-1]
					v.prevPageTokens = v.prevPageTokens[:len(v.prevPageTokens)-1]
				}
				v.currentPage--
				v.loading = true
				return tea.Batch(v.spinner.Tick, v.loadObjects(prevToken))
			}
		}
	}

	var cmd tea.Cmd
	v.list, cmd = v.list.Update(msg)
	return cmd
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
	v.pageToken = ""
	v.prevPageTokens = make([]string, 0)
	v.hasMore = false
}

// updateTitle updates the list title with current path
func (v *ObjectsView) updateTitle() {
	title := fmt.Sprintf("📦 %s", v.bucketName)
	if v.currentPrefix != "" {
		// Show path in title
		path := strings.TrimSuffix(v.currentPrefix, "/")
		title = fmt.Sprintf("📦 %s / %s", v.bucketName, path)
	}
	v.list.Title = title
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
	help := statusStyle.Render("\n  enter: open • n/p: next/prev page • r: refresh • /: filter • esc: back")

	return v.list.View() + "\n" + status + help
}

// SetSize updates the view dimensions
func (v *ObjectsView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.list.SetSize(width, height-6) // Extra space for status and help
}

// GetCurrentPath returns the current folder path being browsed
func (v *ObjectsView) GetCurrentPath() string {
	return v.currentPrefix
}

// GetBucketName returns the bucket name
func (v *ObjectsView) GetBucketName() string {
	return v.bucketName
}
