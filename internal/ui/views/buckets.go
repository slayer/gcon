package views

import (
	gocontext "context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/gcp/usage"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// bucketKeyMap defines bucket-specific key bindings
type bucketKeyMap struct {
	Enter    key.Binding
	Refresh  key.Binding
	Create   key.Binding
	DeepScan key.Binding
	Details  key.Binding
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
		DeepScan: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "calculate usage"),
		),
		Details: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "details"),
		),
	}
}

// Table column definitions for buckets
func bucketColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 36, Grow: true, Sortable: true},
		{Title: "Location", Width: 14, Sortable: true},
		{Title: "Storage Class", Width: 13, Sortable: true},
		{Title: "Size", Width: 12, Sortable: true},
		{Title: "Objects", Width: 11, Sortable: true},
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
	// usageByBucket caches the most recent usage record per bucket so that
	// ProgressMsg updates can be re-rendered without losing prior info.
	usageByBucket map[string]usage.BucketUsage
}

// NewBucketsView creates a new buckets view with table display
func NewBucketsView(projectID string) *BucketsView {
	title := fmt.Sprintf("Cloud Storage Buckets - %s", projectID)
	t := table.NewWithColumns(bucketColumns(), title)

	s := components.NewGCPSpinner()

	v := &BucketsView{
		projectID:     projectID,
		table:         t,
		spinner:       s,
		loading:       true,
		keys:          defaultBucketKeyMap(),
		usageByBucket: make(map[string]usage.BucketUsage),
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

// bucketToRow converts a GCS bucket to a table row.
// The Size and Objects cells render as a faint "…" until usage data arrives;
// they are updated in place when UsageReadyMsg is processed.
func bucketToRow(b gcp.Bucket) table.Row {
	mutedDots := "…"
	return table.Row{
		Data: []string{
			"📦 " + b.Name,
			b.Location,
			b.StorageClass,
			mutedDots,
			mutedDots,
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

		// Convert to table rows.
		rows := make([]table.Row, len(msg.buckets))
		for i, bucket := range msg.buckets {
			rows[i] = bucketToRow(bucket)
		}
		v.table.SetRows(rows)

		// Fan out one monitoring request per bucket so the App can fetch
		// totals via the usage scanner.
		cmds := make([]tea.Cmd, 0, len(msg.buckets))
		for _, bucket := range msg.buckets {
			cmds = append(cmds, func() tea.Msg {
				return UsageMonitoringRequestMsg{Bucket: bucket.Name}
			})
		}
		return tea.Batch(cmds...)

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

	case usage.ReadyMsg:
		// Errors arrive without a usable Usage payload; ignore so we don't
		// overwrite previously-rendered values with zeros.
		if msg.Err != nil {
			return nil
		}
		// Bucket list only cares about whole-bucket usage; ignore folder-scoped
		// scans (Prefix != "") forwarded from ObjectsView, otherwise the bucket
		// row's Size/Objects cells get clobbered with the folder's totals.
		if msg.Usage.Prefix != "" {
			return nil
		}
		v.applyUsage(msg.Usage, true)
		return nil

	case usage.ProgressMsg:
		// Same scope guard as ReadyMsg: ignore folder-scoped progress events.
		if msg.Prefix != "" {
			return nil
		}
		// Show in-progress totals immediately so the user gets feedback.
		v.applyUsage(usage.BucketUsage{
			Bucket:      msg.Bucket,
			TotalBytes:  msg.BytesScanned,
			ObjectCount: msg.ObjectsScanned,
			Source:      usage.SourceDeepScan,
			ScannedAt:   time.Now(),
		}, false)
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

		case key.Matches(msg, v.keys.DeepScan):
			if row := v.table.SelectedRow(); row != nil {
				bucketName := row.ID
				return func() tea.Msg {
					return UsageDeepScanRequestMsg{Bucket: bucketName, Prefix: ""}
				}
			}

		case key.Matches(msg, v.keys.Details):
			if row := v.table.SelectedRow(); row != nil {
				bucketName := row.ID
				return func() tea.Msg {
					return BucketDetailsRequestMsg{Bucket: bucketName}
				}
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
	help := helpStyle.Render("\n  enter: browse • C: calculate usage • c: create • S: sort • /: filter • r: refresh • esc: back")

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

// Buckets returns the cached list of buckets. Used by App for navigation
// (e.g., locating the bucket struct for BucketDetailsView).
func (v *BucketsView) Buckets() []gcp.Bucket {
	return v.buckets
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

// applyUsage stores the usage record and updates the corresponding table row's
// Size and Objects cells in place. The "✓" suffix is reserved for FINAL
// deep-scan results — in-flight progress ticks update the numbers but never
// the marker, so the user can tell at a glance whether the value is verified.
//
// final == true means msg came from a usage.ReadyMsg (final result).
// final == false means msg came from a usage.ProgressMsg (still scanning).
//
// Late-arriving ProgressMsg deliveries (channel reordering after a final
// ReadyMsg already landed) are dropped so they can't overwrite the verified
// total with a smaller in-progress number.
func (v *BucketsView) applyUsage(u usage.BucketUsage, final bool) {
	existing, hasExisting := v.usageByBucket[u.Bucket]
	if !final && hasExisting {
		if existing.Source == usage.SourceDeepScan && existing.ScannedAt.After(u.ScannedAt) {
			return // stale progress after final result
		}
	}
	// Deep-scan results take precedence over monitoring; never let a monitoring
	// refresh (e.g. cache TTL expiry) overwrite a completed deep scan and its ✓
	// marker with a stale ~24h-old monitoring value.
	if hasExisting && u.Source == usage.SourceMonitoring && existing.Source == usage.SourceDeepScan {
		return
	}
	v.usageByBucket[u.Bucket] = u
	rows := v.table.Rows()
	for i := range rows {
		if rows[i].ID != u.Bucket {
			continue
		}
		sizeStr := gcp.FormatSize(u.TotalBytes)
		if final && u.Source == usage.SourceDeepScan {
			sizeStr += " ✓"
		}
		countStr := formatObjectCount(u.ObjectCount)
		// Replace the cells. Data length is fixed by bucketColumns().
		rows[i].Data[3] = sizeStr
		rows[i].Data[4] = countStr
	}
	// Preserve the user's sort across SetRows (which clears it).
	sortCol, sortAsc := v.table.SortState()
	v.table.SetRows(rows)
	if sortCol >= 0 {
		v.table.SortBy(sortCol, sortAsc)
	}
}

