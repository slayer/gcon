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
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// bucketKeyMap defines bucket-specific key bindings
type bucketKeyMap struct {
	Enter   key.Binding
	Refresh key.Binding
	Create  key.Binding
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
		Create: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "create bucket"),
		),
	}
}

// Table column definitions for buckets
func bucketColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 40, Grow: true, Sortable: true},
		{Title: "Location", Width: 15, Sortable: true},
		{Title: "Storage Class", Width: 15, Sortable: true},
		{Title: "Created", Width: 12, Sortable: true},
	}
}

// BucketsView displays and manages Cloud Storage buckets in a table format
type BucketsView struct {
	TableClickDelegate
	storageClient *gcp.StorageClient
	projectID     string
	ctx           *context.ProgramContext // Shared context for dimensions and styles
	table         table.Model
	spinner       spinner.Model
	loading       bool
	err           error
	buckets       []gcp.Bucket
	keys          bucketKeyMap
}

// NewBucketsView creates a new buckets view with table display
func NewBucketsView(projectID string) *BucketsView {
	title := fmt.Sprintf("Cloud Storage Buckets - %s", projectID)
	t := table.NewWithColumns(bucketColumns(), title)

	s := components.NewGCPSpinner()

	v := &BucketsView{
		projectID: projectID,
		table:     t,
		spinner:   s,
		loading:   true,
		keys:      defaultBucketKeyMap(),
	}
	v.Table = &v.table
	return v
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
		client, err := gcp.NewStorageClient(gocontext.Background())
		if err != nil {
			return bucketsErrorMsg{err: err}
		}
		return storageClientReadyMsg{client: client}
	}
}

// loadBuckets fetches buckets from GCP
func (v *BucketsView) loadBuckets() tea.Cmd {
	return func() tea.Msg {
		buckets, err := v.storageClient.ListBuckets(gocontext.Background(), v.projectID)
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

// bucketToRow converts a GCS bucket to a table row
func bucketToRow(b gcp.Bucket) table.Row {
	return table.Row{
		Data: []string{
			"📦 " + b.Name,
			b.Location,
			b.StorageClass,
			timeutil.FormatDate(b.Created),
		},
		FilterValue: b.Name + " " + b.Location + " " + b.StorageClass,
		ID:          b.Name,
	}
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

		// Convert to table rows
		rows := make([]table.Row, len(msg.buckets))
		for i, bucket := range msg.buckets {
			rows[i] = bucketToRow(bucket)
		}
		v.table.SetRows(rows)
		return nil

	case bucketsErrorMsg:
		v.loading = false
		v.err = msg.err
		return nil

	case table.RowDoubleClickedMsg:
		// Handle double-click on table row - navigate to bucket contents
		bucket := v.findBucketByName(msg.RowID)
		if bucket != nil {
			return func() tea.Msg {
				return BucketSelectedMsg{Bucket: *bucket}
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
		// Don't handle keys during loading
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
			// Navigate to bucket contents on Enter
			if row := v.table.SelectedRow(); row != nil {
				bucket := v.findBucketByName(row.ID)
				if bucket != nil {
					return func() tea.Msg {
						return BucketSelectedMsg{Bucket: *bucket}
					}
				}
			}

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			// Re-initialize client if previous attempt failed
			if v.storageClient == nil {
				return tea.Batch(v.spinner.Tick, v.initStorageClient())
			}
			return tea.Batch(v.spinner.Tick, v.loadBuckets())

		case key.Matches(msg, v.keys.Create):
			return func() tea.Msg {
				return BucketCreateRequestMsg{ProjectID: v.projectID}
			}
		}
	}

	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

// findBucketByName looks up a bucket by name
func (v *BucketsView) findBucketByName(name string) *gcp.Bucket {
	for _, b := range v.buckets {
		if b.Name == name {
			return &b
		}
	}
	return nil
}

// View renders the buckets view
func (v *BucketsView) View() string {
	if v.loading && v.storageClient == nil {
		return renderLoading(v.spinner, "Initializing Cloud Storage client...")
	}

	if v.loading {
		return renderLoading(v.spinner, "Loading buckets...")
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
	help := helpStyle.Render("\n  enter: browse • c: create • S: sort • /: filter • r: refresh • esc: back")

	return v.table.View() + help
}

// SetContext updates the view with shared program context.
// Reads dimensions from the context for consistent sizing.
func (v *BucketsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.table.SetSize(ctx.ContentWidth, ctx.ContentHeight-2)
}

// GetStorageClient returns the storage client for reuse in objects view
func (v *BucketsView) GetStorageClient() *gcp.StorageClient {
	return v.storageClient
}

// Close cleans up resources held by the view
func (v *BucketsView) Close() error {
	if v.storageClient != nil {
		return v.storageClient.Close()
	}
	return nil
}

// HasTextInputFocused returns true if the table filter is active.
// Used to prevent global hotkeys (like 'q' for quit) from triggering while typing.
func (v *BucketsView) HasTextInputFocused() bool {
	return v.table.HasTextInputFocused()
}
