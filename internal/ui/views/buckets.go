package views

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
)

// bucketItem implements list.Item for GCS buckets
type bucketItem struct {
	bucket gcp.Bucket
}

func (i bucketItem) Title() string {
	return fmt.Sprintf("📦 %s", i.bucket.Name)
}

func (i bucketItem) Description() string {
	created := i.bucket.Created.Format("2006-01-02")
	return fmt.Sprintf("%s • %s • %s", i.bucket.Location, i.bucket.StorageClass, created)
}

func (i bucketItem) FilterValue() string {
	return i.bucket.Name + " " + i.bucket.Location + " " + i.bucket.StorageClass
}

// BucketsView displays and manages Cloud Storage buckets
type BucketsView struct {
	storageClient *gcp.StorageClient
	projectID     string
	list          list.Model
	spinner       spinner.Model
	loading       bool
	err           error
	width         int
	height        int
	buckets       []gcp.Bucket
	keys          bucketKeyMap
}

// bucketKeyMap defines bucket-specific key bindings
type bucketKeyMap struct {
	Enter   key.Binding
	Refresh key.Binding
}

func defaultBucketKeyMap() bucketKeyMap {
	return bucketKeyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "browse"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
	}
}

// NewBucketsView creates a new buckets view
func NewBucketsView(projectID string) *BucketsView {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#4285F4")).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#CCCCCC")).
		Background(lipgloss.Color("#4285F4"))

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = fmt.Sprintf("Cloud Storage Buckets • %s", projectID)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#4285F4")).
		Padding(0, 1)

	// Add help keys
	l.AdditionalShortHelpKeys = func() []key.Binding {
		km := defaultBucketKeyMap()
		return []key.Binding{km.Enter, km.Refresh}
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	return &BucketsView{
		projectID: projectID,
		list:      l,
		spinner:   s,
		loading:   true,
		keys:      defaultBucketKeyMap(),
	}
}

// Init initializes the view and starts loading buckets
func (v *BucketsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.initStorageClient(),
	)
}

// initStorageClient creates the storage client then loads buckets
func (v *BucketsView) initStorageClient() tea.Cmd {
	return func() tea.Msg {
		client, err := gcp.NewStorageClient(context.Background())
		if err != nil {
			return bucketsErrorMsg{err: err}
		}
		return storageClientReadyMsg{client: client}
	}
}

// loadBuckets fetches buckets from GCP
func (v *BucketsView) loadBuckets() tea.Cmd {
	return func() tea.Msg {
		buckets, err := v.storageClient.ListBuckets(context.Background(), v.projectID)
		if err != nil {
			return bucketsErrorMsg{err: err}
		}
		return bucketsLoadedMsg{buckets: buckets}
	}
}

// Message types for buckets view
type storageClientReadyMsg struct {
	client *gcp.StorageClient
}

type bucketsLoadedMsg struct {
	buckets []gcp.Bucket
}

type bucketsErrorMsg struct {
	err error
}

// BucketSelectedMsg is sent when a bucket is selected (exported for app.go)
type BucketSelectedMsg struct {
	Bucket gcp.Bucket
}

// Update handles messages for the buckets view
func (v *BucketsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case storageClientReadyMsg:
		v.storageClient = msg.client
		return v.loadBuckets()

	case bucketsLoadedMsg:
		v.loading = false
		v.buckets = msg.buckets

		items := make([]list.Item, len(msg.buckets))
		for i, bucket := range msg.buckets {
			items[i] = bucketItem{bucket: bucket}
		}
		v.list.SetItems(items)
		return nil

	case bucketsErrorMsg:
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
			// Navigate to bucket contents on Enter
			if item, ok := v.list.SelectedItem().(bucketItem); ok {
				return func() tea.Msg {
					return BucketSelectedMsg{Bucket: item.bucket}
				}
			}

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.loadBuckets())
		}
	}

	var cmd tea.Cmd
	v.list, cmd = v.list.Update(msg)
	return cmd
}

// View renders the buckets view
func (v *BucketsView) View() string {
	if v.loading && v.storageClient == nil {
		return fmt.Sprintf("\n  %s Initializing Cloud Storage client...\n", v.spinner.View())
	}

	if v.loading {
		return fmt.Sprintf("\n  %s Loading buckets...\n", v.spinner.View())
	}

	if v.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
		return errStyle.Render(fmt.Sprintf("\n  Error: %v\n\n  Press 'r' to retry", v.err))
	}

	if len(v.buckets) == 0 {
		return "\n  No buckets found in this project.\n  Press 'esc' to go back."
	}

	// Help text for actions
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: browse • r: refresh • /: filter • esc: back")

	return v.list.View() + help
}

// SetSize updates the view dimensions
func (v *BucketsView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.list.SetSize(width, height-4)
}

// GetStorageClient returns the storage client for reuse in objects view
func (v *BucketsView) GetStorageClient() *gcp.StorageClient {
	return v.storageClient
}
