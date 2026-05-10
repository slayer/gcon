package views

import (
	gocontext "context"
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/gcp/usage"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/actionmenu"
	"github.com/slayer/gcon/internal/ui/components/confirm"
	"github.com/slayer/gcon/internal/ui/components/filepicker"
	"github.com/slayer/gcon/internal/ui/components/progress"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
	"github.com/slayer/gcon/internal/ui/symbols"
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// Larger page size reduces round-trips for infinite scroll
const defaultPageSize = 200

// Cap prevents unbounded memory growth in large buckets
const maxLoadedObjects = 10000

// objectKeyMap defines object-specific key bindings
type objectKeyMap struct {
	Enter      key.Binding
	Refresh    key.Binding
	Download   key.Binding
	Upload     key.Binding
	Delete     key.Binding
	ActionMenu key.Binding
	DeepScan   key.Binding
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
		Download: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "download"),
		),
		Upload: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "upload"),
		),
		Delete: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "delete"),
		),
		ActionMenu: key.NewBinding(
			key.WithKeys("."),
			key.WithHelp(".", "actions"),
		),
		DeepScan: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "calculate folder size"),
		),
	}
}

// Table column definitions for objects (no sorting or field filtering)
func objectColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 40, Grow: true},
		{Title: "Size", Width: 12},
		{Title: "Content Type", Width: 20},
		{Title: "Modified", Width: 12},
	}
}

// ObjectsView displays and manages objects within a bucket using a table format
type ObjectsView struct {
	TableClickDelegate
	storageClient *gcp.StorageClient
	bucketName    string
	currentPrefix string                  // Current folder path (e.g., "folder1/folder2/")
	prefixStack   []string                // Navigation history for back functionality
	ctx           *context.ProgramContext // Shared context for dimensions and styles
	table         table.Model
	spinner       spinner.Model
	loading       bool
	err           error
	width         int
	height        int
	objects       []gcp.StorageObject
	keys          objectKeyMap

	// Infinite scroll state
	nextPageToken  string // Token for loading next page
	loadingMore    bool   // Currently fetching next batch
	allLoaded      bool   // No more data available
	loadMoreErr    error  // Non-fatal error from last load-more attempt
	loadGeneration int    // Increments on folder navigation to discard stale responses

	// Download state
	downloading      bool
	downloadProgress *progress.Progress
	downloadFiles    []gcp.StorageObject // Files being downloaded (for folder downloads)
	downloadIndex    int                 // Current file index in multi-file download
	downloadChan     chan progress.ProgressUpdate

	// Upload state
	showFilePicker bool
	filePicker     *filepicker.FilePicker
	uploading      bool
	uploadProgress *progress.Progress
	uploadFiles    []string // Local file paths to upload
	uploadChan     chan progress.ProgressUpdate

	// Delete state
	pendingDelete      *gcp.StorageObject  // Object pending delete confirmation
	pendingDeleteFiles []gcp.StorageObject // Files to delete (resolved for folders)
	showDeleteConfirm  bool
	deleteConfirm      *confirm.ConfirmDialog
	deleting           bool
	deleteProgress     *progress.Progress
	deleteChan         chan deleteProgressUpdate

	// Action menu state
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// folderUsage holds the most recent deep-scan result for the current
	// (bucketName, currentPrefix) pair, displayed as an inline stats line
	// above the table. Set by ReadyMsg / ProgressMsg from the usage scanner.
	folderUsage *usage.BucketUsage
}

// NewObjectsView creates a new objects view with table display
func NewObjectsView(bucketName string, storageClient *gcp.StorageClient) *ObjectsView {
	title := fmt.Sprintf("Bucket: %s", bucketName)
	t := table.NewWithColumns(objectColumns(), title)

	s := components.NewGCPSpinner()

	v := &ObjectsView{
		storageClient:    storageClient,
		bucketName:       bucketName,
		currentPrefix:    "",
		prefixStack:      make([]string, 0),
		table:            t,
		spinner:          s,
		loading:          true,
		keys:             defaultObjectKeyMap(),
		downloadProgress: progress.New(),
		uploadProgress:   progress.New(),
		deleteProgress:   progress.New(),
	}
	v.Table = &v.table
	// Enable near-bottom detection for infinite scroll
	v.table.SetNearBottomThreshold(5)
	return v
}

// Init initializes the view and starts loading objects
func (v *ObjectsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.loadObjects(),
	)
}

// loadObjects fetches the initial batch of objects from the bucket
func (v *ObjectsView) loadObjects() tea.Cmd {
	gen := v.loadGeneration
	return func() tea.Msg {
		result, err := v.storageClient.ListObjects(
			gocontext.Background(),
			v.bucketName,
			v.currentPrefix,
			"",
			defaultPageSize,
		)
		if err != nil {
			return objectsErrorMsg{err: err}
		}
		return objectsLoadedMsg{
			objects:    result.Objects,
			nextToken:  result.NextToken,
			hasMore:    result.HasMore,
			generation: gen,
		}
	}
}

// loadMoreObjects fetches the next batch for infinite scroll.
// Returns objectsMoreErrorMsg (non-fatal) on failure to preserve existing data.
func (v *ObjectsView) loadMoreObjects() tea.Cmd {
	gen := v.loadGeneration
	token := v.nextPageToken
	return func() tea.Msg {
		result, err := v.storageClient.ListObjects(
			gocontext.Background(),
			v.bucketName,
			v.currentPrefix,
			token,
			defaultPageSize,
		)
		if err != nil {
			return objectsMoreErrorMsg{err: err}
		}
		return objectsMoreLoadedMsg{
			objects:    result.Objects,
			nextToken:  result.NextToken,
			hasMore:    result.HasMore,
			generation: gen,
		}
	}
}

// Message types for objects view
type objectsLoadedMsg struct {
	objects    []gcp.StorageObject
	nextToken  string
	hasMore    bool
	generation int // Matches loadGeneration to discard stale responses
}

type objectsMoreLoadedMsg struct {
	objects    []gcp.StorageObject
	nextToken  string
	hasMore    bool
	generation int
}

type objectsErrorMsg struct {
	err error
}

// objectsMoreErrorMsg is a non-fatal error from loading more data.
// Unlike objectsErrorMsg, it preserves already-loaded objects.
type objectsMoreErrorMsg struct {
	err error
}

// ObjectsBackMsg signals to return to buckets view (exported for app.go)
type ObjectsBackMsg struct{}

// objectToRow converts a GCS object to a table row
func objectToRow(obj gcp.StorageObject) table.Row { //nolint:gocritic // Copying object is acceptable
	var name, size, contentType, modified string

	if obj.IsFolder {
		name = symbols.Folder() + " " + obj.DisplayName + "/"
		size = "-"
		contentType = "Folder"
		modified = "-"
	} else {
		name = symbols.File() + " " + obj.DisplayName
		size = gcp.FormatSize(obj.Size)
		contentType = obj.ContentType
		if contentType == "" {
			contentType = "unknown"
		}
		modified = timeutil.FormatDate(obj.Updated)
	}

	return table.Row{
		Data:        []string{name, size, contentType, modified},
		FilterValue: obj.DisplayName + " " + contentType,
		ID:          obj.Name, // Full path name for lookup
	}
}

// folderUsageKey computes the ByTopPrefix key for a folder row, given the
// scan prefix (= currentPrefix at the time of scan). Mirrors the logic in
// usage.tally.topPrefixSegment so cell lookups match scan results for folder
// rows. Files are not looked up via this helper (they map to "(root)" or
// other extension-based buckets in the usage breakdown).
func folderUsageKey(rowID, scanPrefix string) string {
	rel := rowID
	if scanPrefix != "" {
		normalized := scanPrefix
		if !strings.HasSuffix(normalized, "/") {
			normalized += "/"
		}
		rel = strings.TrimPrefix(rowID, normalized)
	}
	if rel == "" {
		return "(root)"
	}
	if i := strings.Index(rel, "/"); i >= 0 {
		return rel[:i+1]
	}
	return rel
}

// Download-related messages
type downloadStartMsg struct {
	files []gcp.StorageObject // Files to download (single file or folder contents)
}

type downloadProgressMsg struct {
	update progress.ProgressUpdate
}

type downloadCompleteMsg struct {
	err error
}

// Upload-related messages
type uploadStartMsg struct {
	files []string // Local file paths to upload
}

type uploadProgressMsg struct {
	update progress.ProgressUpdate
}

type uploadCompleteMsg struct {
	err error
}

// Delete-related messages
type deleteRequestMsg struct {
	object gcp.StorageObject
}

type deleteFilesResolvedMsg struct {
	files []gcp.StorageObject
	err   error
}

type deleteStartMsg struct {
	files []gcp.StorageObject
}

type deleteProgressUpdate struct {
	deletedCount int
	totalCount   int
	currentFile  string
	done         bool
	err          error
	failedObject string
}

type deleteProgressMsg struct {
	deletedCount int
	totalCount   int
	currentFile  string
}

type deleteCompleteMsg struct {
	err          error
	deletedCount int
	failedObject string
}

// Update handles messages for the objects view
//
//nolint:gocognit // Bubble Tea Update pattern - complexity 108
func (v *ObjectsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case objectsLoadedMsg:
		// Discard stale responses from previous folder
		if msg.generation != v.loadGeneration {
			return nil
		}
		v.loading = false
		v.objects = msg.objects
		v.nextPageToken = msg.nextToken
		v.allLoaded = !msg.hasMore

		// Convert to table rows
		rows := make([]table.Row, len(msg.objects))
		for i, obj := range msg.objects {
			rows[i] = objectToRow(obj)
		}
		v.table.SetRows(rows)
		return nil

	case objectsMoreLoadedMsg:
		// Discard stale responses from previous folder
		if msg.generation != v.loadGeneration {
			v.loadingMore = false
			return nil
		}
		v.loadingMore = false
		v.objects = append(v.objects, msg.objects...)
		v.nextPageToken = msg.nextToken
		v.allLoaded = !msg.hasMore

		// Append to table (preserves cursor position)
		newRows := make([]table.Row, len(msg.objects))
		for i, obj := range msg.objects {
			newRows[i] = objectToRow(obj)
		}
		v.table.AppendRows(newRows)
		return nil

	case table.NearBottomMsg:
		// Infinite scroll: load more when cursor approaches bottom
		if !v.loadingMore && !v.allLoaded && v.nextPageToken != "" && len(v.objects) < maxLoadedObjects {
			v.loadingMore = true
			v.loadMoreErr = nil
			return v.loadMoreObjects()
		}
		return nil

	case objectsErrorMsg:
		v.loading = false
		v.loadingMore = false
		v.err = msg.err
		return nil

	case objectsMoreErrorMsg:
		// Non-fatal: keep existing data visible, just stop loading more
		v.loadingMore = false
		v.loadMoreErr = msg.err
		return nil

	case table.RowDoubleClickedMsg:
		// Handle double-click on table row - navigate into folder
		obj := v.findObjectByName(msg.RowID)
		if obj != nil && obj.IsFolder {
			// Push current prefix to stack and navigate into folder
			v.prefixStack = append(v.prefixStack, v.currentPrefix)
			v.currentPrefix = obj.Name
			v.resetScrollState()
			v.loading = true
			return tea.Batch(v.spinner.Tick, v.loadObjects())
		}
		return nil

	case usage.ReadyMsg:
		// Errors arrive without a usable Usage payload; ignore so we don't
		// overwrite previously-rendered values.
		if msg.Err != nil {
			return nil
		}
		// Only consume results that match this view's (bucket, prefix) tuple.
		if msg.Usage.Bucket != v.bucketName || msg.Usage.Prefix != v.currentPrefix {
			return nil
		}
		u := msg.Usage
		v.folderUsage = &u
		// Populate per-sub-folder Size cells from the per-prefix breakdown.
		// Only the final ReadyMsg has ByTopPrefix populated; ProgressMsg
		// updates the inline header but leaves cells alone.
		v.applyFolderUsageToTable()
		return nil

	case usage.ProgressMsg:
		if msg.Bucket != v.bucketName || msg.Prefix != v.currentPrefix {
			return nil
		}
		v.folderUsage = &usage.BucketUsage{
			Bucket:      msg.Bucket,
			Prefix:      msg.Prefix,
			TotalBytes:  msg.BytesScanned,
			ObjectCount: msg.ObjectsScanned,
			Source:      usage.SourceDeepScan,
		}
		return nil

	case spinner.TickMsg:
		if v.loading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case downloadStartMsg:
		v.downloading = true
		v.downloadFiles = msg.files
		v.downloadIndex = 0

		// Calculate total size for progress
		var totalSize int64
		for _, f := range msg.files {
			totalSize += f.Size
		}

		v.downloadProgress.Start() // Start elapsed time tracking
		v.downloadProgress.SetProgress(
			"Downloading",
			msg.files[0].DisplayName,
			1,
			len(msg.files),
			0,
			totalSize,
		)
		v.downloadProgress.SetSize(v.width)

		// Channel created inside startDownload to avoid leak on early error
		return v.startDownload()

	case downloadProgressMsg:
		v.downloadProgress.SetProgress(
			"Downloading",
			msg.update.CurrentFile,
			msg.update.CurrentFileNum,
			msg.update.TotalFiles,
			msg.update.BytesTransferred,
			msg.update.TotalBytes,
		)
		return v.waitForProgress()

	case downloadCompleteMsg:
		v.downloading = false
		v.downloadFiles = nil
		v.downloadIndex = 0
		if v.downloadChan != nil {
			close(v.downloadChan)
			v.downloadChan = nil
		}
		if msg.err != nil {
			v.err = msg.err
		}
		return nil

	case filepicker.FilePickerConfirmMsg:
		// User confirmed file selection - start upload
		v.showFilePicker = false
		v.filePicker = nil
		if len(msg.SelectedPaths) > 0 {
			return func() tea.Msg {
				return uploadStartMsg{files: msg.SelectedPaths}
			}
		}
		return nil

	case filepicker.FilePickerCancelMsg:
		// User canceled file picker
		v.showFilePicker = false
		v.filePicker = nil
		return nil

	case uploadStartMsg:
		v.uploading = true
		v.uploadFiles = msg.files

		// Calculate total size for initial progress display
		var totalSize int64
		for _, path := range msg.files {
			info, err := os.Stat(path)
			if err == nil && !info.IsDir() {
				totalSize += info.Size()
			}
		}

		v.uploadProgress.Start() // Start elapsed time tracking
		v.uploadProgress.SetProgress(
			"Uploading",
			filepath.Base(msg.files[0]),
			1,
			len(msg.files),
			0,
			totalSize,
		)
		v.uploadProgress.SetSize(v.width)

		// Channel created inside startUpload to avoid leak on early error
		return v.startUpload()

	case uploadProgressMsg:
		v.uploadProgress.SetProgress(
			"Uploading",
			msg.update.CurrentFile,
			msg.update.CurrentFileNum,
			msg.update.TotalFiles,
			msg.update.BytesTransferred,
			msg.update.TotalBytes,
		)
		return v.waitForUploadProgress()

	case uploadCompleteMsg:
		v.uploading = false
		v.uploadFiles = nil
		if v.uploadChan != nil {
			close(v.uploadChan)
			v.uploadChan = nil
		}
		if msg.err != nil {
			v.err = msg.err
		} else {
			// Refresh the list after successful upload
			v.resetScrollState()
			v.loading = true
			return tea.Batch(v.spinner.Tick, v.loadObjects())
		}
		return nil

	case deleteRequestMsg:
		// Store pending delete and resolve files if folder
		obj := msg.object
		v.pendingDelete = &obj
		if msg.object.IsFolder {
			return v.resolveDeleteFiles(msg.object)
		}
		return func() tea.Msg {
			return deleteFilesResolvedMsg{files: []gcp.StorageObject{msg.object}}
		}

	case deleteFilesResolvedMsg:
		if msg.err != nil {
			v.err = msg.err
			v.pendingDelete = nil
			return nil
		}
		// Handle empty folder case - nothing to delete
		if len(msg.files) == 0 {
			v.err = fmt.Errorf("%w, nothing to delete", uierrors.ErrFolderEmpty)
			v.pendingDelete = nil
			return nil
		}
		v.pendingDeleteFiles = msg.files
		// Show confirmation dialog
		v.showDeleteConfirm = true
		v.deleteConfirm = v.createDeleteConfirmDialog(msg.files)
		return nil

	case confirm.ConfirmMsg:
		if v.showDeleteConfirm {
			v.showDeleteConfirm = false
			v.deleteConfirm = nil
			files := v.pendingDeleteFiles
			return func() tea.Msg {
				return deleteStartMsg{files: files}
			}
		}
		return nil

	case confirm.CancelMsg:
		if v.showDeleteConfirm {
			v.showDeleteConfirm = false
			v.deleteConfirm = nil
			v.pendingDelete = nil
			v.pendingDeleteFiles = nil
		}
		return nil

	case deleteStartMsg:
		v.deleting = true
		if len(msg.files) > 0 {
			v.deleteProgress.Start() // Start elapsed time tracking
			v.deleteProgress.SetProgress(
				"Deleting",
				msg.files[0].DisplayName,
				1,
				len(msg.files),
				0,
				int64(len(msg.files)),
			)
			v.deleteProgress.SetSize(v.width)
		}
		return v.startDelete()

	case deleteProgressMsg:
		v.deleteProgress.SetProgress(
			"Deleting",
			msg.currentFile,
			msg.deletedCount+1,
			msg.totalCount,
			int64(msg.deletedCount),
			int64(msg.totalCount),
		)
		return v.waitForDeleteProgress()

	case deleteCompleteMsg:
		v.deleting = false
		v.pendingDelete = nil
		v.pendingDeleteFiles = nil
		if v.deleteChan != nil {
			close(v.deleteChan)
			v.deleteChan = nil
		}
		if msg.err != nil {
			if msg.deletedCount > 0 {
				v.err = fmt.Errorf("deleted %d files, failed on %s: %w", msg.deletedCount, msg.failedObject, msg.err)
			} else {
				v.err = msg.err
			}
		} else {
			// Refresh list after successful deletion
			v.resetScrollState()
			v.loading = true
			return tea.Batch(v.spinner.Tick, v.loadObjects())
		}
		return nil

	case actionmenu.ActionSelectedMsg:
		// Handle action menu selection
		v.menuOpen = false
		return v.executeMenuAction(msg.Key)

	case actionmenu.ActionMenuClosedMsg:
		v.menuOpen = false
		return nil

	case tea.KeyMsg:
		// Route keys to action menu when open
		if v.menuOpen && v.actionMenu != nil {
			return v.actionMenu.Update(msg)
		}

		// If delete confirmation is shown, delegate to it
		if v.showDeleteConfirm && v.deleteConfirm != nil {
			cmd := v.deleteConfirm.Update(msg)
			return cmd
		}

		// If file picker is shown, delegate to it
		if v.showFilePicker && v.filePicker != nil {
			cmd := v.filePicker.Update(msg)
			return cmd
		}

		// Don't handle keys during loading, downloading, uploading, or deleting
		if v.loading || v.downloading || v.uploading || v.deleting {
			return nil
		}

		// Let table handle filtering mode
		if v.table.IsFiltering() {
			var cmd tea.Cmd
			v.table, cmd = v.table.Update(msg)
			return cmd
		}

		switch {
		case key.Matches(msg, v.keys.ActionMenu):
			// Open action menu for selected item
			if row := v.table.SelectedRow(); row != nil {
				obj := v.findObjectByName(row.ID)
				if obj != nil {
					v.actionMenu = actionmenu.New("Object Actions", v.buildObjectActions(*obj))
					v.menuOpen = true
				}
			}
			return nil

		case key.Matches(msg, v.keys.Upload):
			// Open file picker for upload
			cwd, _ := os.Getwd() //nolint:errcheck // Fallback to empty
			v.filePicker = filepicker.New(cwd, true)
			v.filePicker.SetSize(v.width-10, v.height-10)
			v.showFilePicker = true
			return v.filePicker.Init()

		case key.Matches(msg, v.keys.Delete):
			// Delete selected file or folder
			if row := v.table.SelectedRow(); row != nil {
				obj := v.findObjectByName(row.ID)
				if obj != nil {
					return v.prepareDelete(*obj)
				}
			}

		case key.Matches(msg, v.keys.Download):
			// Download selected file or folder
			if row := v.table.SelectedRow(); row != nil {
				obj := v.findObjectByName(row.ID)
				if obj != nil {
					return v.prepareDownload(*obj)
				}
			}

		case key.Matches(msg, v.keys.Enter):
			// Navigate into folder or view file details on Enter
			if row := v.table.SelectedRow(); row != nil {
				obj := v.findObjectByName(row.ID)
				if obj != nil {
					if obj.IsFolder {
						// Navigate into folder
						v.prefixStack = append(v.prefixStack, v.currentPrefix)
						v.currentPrefix = obj.Name
						v.resetScrollState()
						v.loading = true
						return tea.Batch(v.spinner.Tick, v.loadObjects())
					}
					// For files, emit selection message to open details view
					selectedObj := *obj
					return func() tea.Msg {
						return ObjectSelectedMsg{Object: selectedObj}
					}
				}
			}

		case key.Matches(msg, v.keys.DeepScan):
			// Run a folder-scoped deep scan. App registers the footer task and
			// dispatches Progress/Ready messages back to this view.
			bucket := v.bucketName
			prefix := v.currentPrefix
			return func() tea.Msg {
				return UsageDeepScanRequestMsg{Bucket: bucket, Prefix: prefix}
			}

		case key.Matches(msg, v.keys.Refresh):
			v.resetScrollState()
			v.loading = true
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.loadObjects())
		}

	default:
		// Forward other messages to file picker when active (for async directory loading)
		if v.showFilePicker && v.filePicker != nil {
			cmd := v.filePicker.Update(msg)
			return cmd
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

// lookupObjectByName returns a pointer into v.objects whose Name matches; nil
// if not found. Unlike findObjectByName this returns a stable pointer into the
// slice (rather than a loop-variable copy), but for our read-only callers the
// distinction is immaterial.
func (v *ObjectsView) lookupObjectByName(name string) *gcp.StorageObject {
	for i := range v.objects {
		if v.objects[i].Name == name {
			return &v.objects[i]
		}
	}
	return nil
}

// applyFolderUsageToTable walks the current table rows; for each folder row,
// looks up its recursive size in v.folderUsage.ByTopPrefix and updates the
// Size cell in place. Files are left untouched. Called after a deep-scan
// ReadyMsg arrives. Sort state is preserved by snapshotting before SetRows.
func (v *ObjectsView) applyFolderUsageToTable() {
	if v.folderUsage == nil || v.folderUsage.Source != usage.SourceDeepScan {
		return
	}
	if len(v.folderUsage.ByTopPrefix) == 0 {
		return
	}
	rows := v.table.Rows()
	for i := range rows {
		obj := v.lookupObjectByName(rows[i].ID)
		if obj == nil || !obj.IsFolder {
			continue
		}
		key := folderUsageKey(rows[i].ID, v.folderUsage.Prefix)
		stat, ok := v.folderUsage.ByTopPrefix[key]
		if !ok {
			continue // no data for this folder
		}
		// Size cell is index 1 per objectColumns().
		rows[i].Data[1] = gcp.FormatSize(stat.Bytes) + " ✓"
	}
	// Preserve the user's sort across SetRows (which clears it).
	sortCol, sortAsc := v.table.SortState()
	v.table.SetRows(rows)
	if sortCol >= 0 {
		v.table.SortBy(sortCol, sortAsc)
	}
}

// HandleBack handles ESC key - returns true if handled internally (went up a folder)
func (v *ObjectsView) HandleBack() (handled bool, cmd tea.Cmd) {
	// If we're in a subfolder, go up
	if len(v.prefixStack) > 0 {
		v.currentPrefix = v.prefixStack[len(v.prefixStack)-1]
		v.prefixStack = v.prefixStack[:len(v.prefixStack)-1]
		v.resetScrollState()
		v.loading = true
		return true, tea.Batch(v.spinner.Tick, v.loadObjects())
	}
	// At root, signal to return to buckets view
	return false, nil
}

// resetScrollState resets infinite scroll state for folder navigation.
// Also clears any cached folder-usage stats since they were tied to the old
// (bucket, prefix) and don't apply to the new folder.
func (v *ObjectsView) resetScrollState() {
	v.nextPageToken = ""
	v.loadingMore = false
	v.allLoaded = false
	v.loadMoreErr = nil
	v.loadGeneration++ // Invalidate any in-flight responses
	v.objects = nil
	v.folderUsage = nil
	v.table.ResetNearBottom()
}

// scrollInfo returns the infinite scroll state suffix for the status bar.
func (v *ObjectsView) scrollInfo() string {
	switch {
	case v.loadMoreErr != nil:
		return "(load error, press r to retry)"
	case v.loadingMore:
		return "(loading more...)"
	case len(v.objects) >= maxLoadedObjects && v.nextPageToken != "":
		return fmt.Sprintf("(showing first %d, use filter to narrow)", maxLoadedObjects)
	case v.allLoaded:
		return "(all loaded)"
	case v.nextPageToken != "":
		return "(scroll for more)"
	default:
		return ""
	}
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
		return renderLoading(v.spinner, loadingMsg)
	}

	if v.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
		return errStyle.Render(fmt.Sprintf("\n  Error: %v\n\n  Press 'r' to retry", v.err))
	}

	if len(v.objects) == 0 {
		// When file picker is shown, render it directly (not as overlay)
		// since empty bucket content is too short for proper overlay
		if v.showFilePicker && v.filePicker != nil {
			return v.renderCenteredFilePicker()
		}

		msg := "This bucket is empty."
		if v.currentPrefix != "" {
			msg = "This folder is empty."
		}
		return fmt.Sprintf("\n  %s\n  Press 'u' to upload files, 'esc' to go back.", msg)
	}

	// Keep table title and status in sync with current state
	v.table.SetTitle(v.buildTitle())
	v.table.SetStatusSuffix(v.scrollInfo())

	// Help text for actions
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: open • C: folder size • d: download • u: upload • D: delete • .: menu • /: filter • r: refresh • esc: back")

	// Inline folder-size stats line shown above the table when a deep scan
	// has produced (or is producing) data for the current folder.
	var statsLine string
	if v.folderUsage != nil {
		muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
		statsLine = "  " + muted.Render(fmt.Sprintf(
			"Folder size: %s · %s objects (deep scan)",
			gcp.FormatSize(v.folderUsage.TotalBytes),
			formatObjectCount(v.folderUsage.ObjectCount),
		)) + "\n"
	}

	content := statsLine + v.table.View() + help

	// Overlay action menu when shown
	if v.menuOpen && v.actionMenu != nil {
		content = v.overlayActionMenu(content)
	}

	// Overlay file picker when shown
	if v.showFilePicker && v.filePicker != nil {
		content = v.overlayFilePicker(content)
	}

	// Overlay progress bar during download
	if v.downloading {
		content = v.overlayProgress(content)
	}

	// Overlay progress bar during upload
	if v.uploading {
		content = v.overlayUploadProgress(content)
	}

	// Overlay delete confirmation dialog
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		content = v.overlayDeleteConfirm(content)
	}

	// Overlay progress bar during delete
	if v.deleting {
		content = v.overlayDeleteProgress(content)
	}

	return content
}

// SetContext updates the view with shared program context.
// Reads dimensions from the context for consistent sizing.
func (v *ObjectsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.table.SetSize(ctx.ContentWidth, ctx.ContentHeight-2)
}

// GetCurrentPath returns the current folder path being browsed
func (v *ObjectsView) GetCurrentPath() string {
	return v.currentPrefix
}

// GetBucketName returns the bucket name
func (v *ObjectsView) GetBucketName() string {
	return v.bucketName
}

// IsFilePickerShown returns true if the file picker is currently shown
func (v *ObjectsView) IsFilePickerShown() bool {
	return v.showFilePicker
}

// HasTextInputFocused returns true if the table filter is active.
// Used to prevent global hotkeys (like 'q' for quit) from triggering while typing.
func (v *ObjectsView) HasTextInputFocused() bool {
	return v.table.HasTextInputFocused()
}

// ObjectsLoadedMsgForTest creates an objectsLoadedMsg for testing
func ObjectsLoadedMsgForTest(objects []gcp.StorageObject, nextToken string, hasMore bool) objectsLoadedMsg {
	return objectsLoadedMsg{
		objects:    objects,
		nextToken:  nextToken,
		hasMore:    hasMore,
		generation: 0, // Default generation for tests
	}
}

// prepareDownload initiates download for a file or folder
func (v *ObjectsView) prepareDownload(obj gcp.StorageObject) tea.Cmd { //nolint:gocritic // Copying object is acceptable
	return func() tea.Msg {
		if obj.IsFolder {
			// For folders, list all objects recursively
			objects, err := v.storageClient.ListAllObjects(
				gocontext.Background(),
				v.bucketName,
				obj.Name,
			)
			if err != nil {
				return downloadCompleteMsg{err: err}
			}
			if len(objects) == 0 {
				return downloadCompleteMsg{err: uierrors.ErrFolderEmpty}
			}
			return downloadStartMsg{files: objects}
		}
		// Single file download
		return downloadStartMsg{files: []gcp.StorageObject{obj}}
	}
}

// startDownload begins the actual download process in a background goroutine
func (v *ObjectsView) startDownload() tea.Cmd {
	files := v.downloadFiles
	if len(files) == 0 {
		return func() tea.Msg {
			return downloadCompleteMsg{err: nil}
		}
	}

	// Get current working directory for downloads
	cwd, err := os.Getwd()
	if err != nil {
		return func() tea.Msg {
			return downloadCompleteMsg{err: fmt.Errorf("failed to get working directory: %w", err)}
		}
	}

	// Create channel for progress updates
	v.downloadChan = make(chan progress.ProgressUpdate, 10)

	// Calculate total size
	var totalSize int64
	for _, f := range files {
		totalSize += f.Size
	}

	// Run download in background goroutine
	go func() {
		var bytesDownloaded int64

		for i, file := range files {
			// Determine local path - preserve folder structure for multi-file downloads
			localPath := filepath.Join(cwd, file.DisplayName)
			if len(files) > 1 {
				// For folder downloads, preserve relative path structure
				localPath = filepath.Join(cwd, file.Name)
			}

			// Send progress update before starting this file (non-blocking)
			select {
			case v.downloadChan <- progress.ProgressUpdate{
				BytesTransferred: bytesDownloaded,
				TotalBytes:       totalSize,
				CurrentFile:      file.DisplayName,
				CurrentFileNum:   i + 1,
				TotalFiles:       len(files),
			}:
			default:
			}

			// Download with progress callback
			err := v.storageClient.DownloadObject(
				gocontext.Background(),
				v.bucketName,
				file.Name,
				localPath,
				func(transferred, total int64) {
					// Non-blocking send to avoid deadlock if channel is full
					select {
					case v.downloadChan <- progress.ProgressUpdate{
						BytesTransferred: bytesDownloaded + transferred,
						TotalBytes:       totalSize,
						CurrentFile:      file.DisplayName,
						CurrentFileNum:   i + 1,
						TotalFiles:       len(files),
					}:
					default:
					}
				},
			)
			if err != nil {
				// Send error completion
				select {
				case v.downloadChan <- progress.ProgressUpdate{
					Done:  true,
					Error: fmt.Errorf("failed to download %s: %w", file.DisplayName, err),
				}:
				default:
				}
				return
			}

			bytesDownloaded += file.Size
		}

		// Send completion signal
		select {
		case v.downloadChan <- progress.ProgressUpdate{Done: true}:
		default:
		}
	}()

	// Immediately start polling for progress updates
	return v.waitForProgress()
}

// waitForProgress waits for the next progress update from the download goroutine
func (v *ObjectsView) waitForProgress() tea.Cmd {
	return func() tea.Msg {
		if v.downloadChan == nil {
			return downloadCompleteMsg{err: nil}
		}
		update, ok := <-v.downloadChan
		if !ok {
			return downloadCompleteMsg{err: nil}
		}
		// Check if download is complete
		if update.Done {
			return downloadCompleteMsg{err: update.Error}
		}
		return downloadProgressMsg{update: update}
	}
}

// overlayProgress renders the progress bar centered over the content
func (v *ObjectsView) overlayProgress(content string) string {
	// Split content into lines
	lines := strings.Split(content, "\n")

	// Get progress bar content
	progressView := v.downloadProgress.View()
	progressLines := strings.Split(progressView, "\n")

	// Calculate vertical position (center)
	startRow := (len(lines) - len(progressLines)) / 2
	if startRow < 0 {
		startRow = 0
	}

	// Calculate horizontal position (center) for each progress line
	for i, pLine := range progressLines {
		row := startRow + i
		if row >= len(lines) {
			break
		}

		// Get visible width of progress line (accounting for ANSI codes)
		pWidth := lipgloss.Width(pLine)
		contentWidth := lipgloss.Width(lines[row])

		// Calculate padding to center the progress bar
		leftPad := (v.width - pWidth) / 2
		if leftPad < 0 {
			leftPad = 0
		}

		// Build the new line with progress centered
		if contentWidth > 0 && leftPad > 0 {
			// Create padded progress line
			lines[row] = strings.Repeat(" ", leftPad) + pLine
		} else {
			lines[row] = pLine
		}
	}

	return strings.Join(lines, "\n")
}

// startUpload begins the actual upload process in a background goroutine
//
//nolint:gocognit // Upload orchestration with progress tracking - complexity 41
func (v *ObjectsView) startUpload() tea.Cmd {
	files := v.uploadFiles
	if len(files) == 0 {
		return func() tea.Msg {
			return uploadCompleteMsg{err: nil}
		}
	}

	// Calculate total size and collect file info before starting goroutine
	type uploadFile struct {
		localPath  string
		remotePath string
		size       int64
	}
	var filesToUpload []uploadFile
	var totalSize int64

	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			return func() tea.Msg {
				return uploadCompleteMsg{err: fmt.Errorf("failed to stat %s: %w", path, err)}
			}
		}

		if info.IsDir() {
			// Walk directory and collect all files
			err := filepath.Walk(path, func(filePath string, fi os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !fi.IsDir() {
					relPath, relErr := filepath.Rel(filepath.Dir(path), filePath)
					if relErr != nil {
						return fmt.Errorf("failed to compute relative path for %s: %w", filePath, relErr)
					}
					// Use path.Join for GCS object names (forward slashes)
					remotePath := pathpkg.Join(v.currentPrefix, filepath.ToSlash(relPath))
					filesToUpload = append(filesToUpload, uploadFile{
						localPath:  filePath,
						remotePath: remotePath,
						size:       fi.Size(),
					})
					totalSize += fi.Size()
				}
				return nil
			})
			if err != nil {
				return func() tea.Msg {
					return uploadCompleteMsg{err: fmt.Errorf("failed to walk directory %s: %w", path, err)}
				}
			}
		} else {
			// Single file - upload to current prefix using path.Join for correct GCS paths
			remotePath := pathpkg.Join(v.currentPrefix, info.Name())
			filesToUpload = append(filesToUpload, uploadFile{
				localPath:  path,
				remotePath: remotePath,
				size:       info.Size(),
			})
			totalSize += info.Size()
		}
	}

	// Create channel for progress updates
	v.uploadChan = make(chan progress.ProgressUpdate, 10)

	// Run upload in background goroutine
	go func() {
		var bytesUploaded int64

		for i, file := range filesToUpload {
			// Send progress update before starting this file (non-blocking)
			select {
			case v.uploadChan <- progress.ProgressUpdate{
				BytesTransferred: bytesUploaded,
				TotalBytes:       totalSize,
				CurrentFile:      filepath.Base(file.localPath),
				CurrentFileNum:   i + 1,
				TotalFiles:       len(filesToUpload),
			}:
			default:
			}

			// Upload with progress callback
			err := v.storageClient.UploadObject(
				gocontext.Background(),
				v.bucketName,
				file.remotePath,
				file.localPath,
				func(transferred, total int64) {
					// Non-blocking send to avoid deadlock if channel is full
					select {
					case v.uploadChan <- progress.ProgressUpdate{
						BytesTransferred: bytesUploaded + transferred,
						TotalBytes:       totalSize,
						CurrentFile:      filepath.Base(file.localPath),
						CurrentFileNum:   i + 1,
						TotalFiles:       len(filesToUpload),
					}:
					default:
					}
				},
			)
			if err != nil {
				// Send error completion
				select {
				case v.uploadChan <- progress.ProgressUpdate{
					Done:  true,
					Error: fmt.Errorf("failed to upload %s: %w", filepath.Base(file.localPath), err),
				}:
				default:
				}
				return
			}

			bytesUploaded += file.size
		}

		// Send completion signal
		select {
		case v.uploadChan <- progress.ProgressUpdate{Done: true}:
		default:
		}
	}()

	// Immediately start polling for progress updates
	return v.waitForUploadProgress()
}

// waitForUploadProgress waits for the next progress update from the upload goroutine
func (v *ObjectsView) waitForUploadProgress() tea.Cmd {
	return func() tea.Msg {
		if v.uploadChan == nil {
			return uploadCompleteMsg{err: nil}
		}
		update, ok := <-v.uploadChan
		if !ok {
			return uploadCompleteMsg{err: nil}
		}
		// Check if upload is complete
		if update.Done {
			return uploadCompleteMsg{err: update.Error}
		}
		return uploadProgressMsg{update: update}
	}
}

// renderCenteredFilePicker renders the file picker centered on screen
// Used when there's no meaningful background content (e.g., empty bucket)
func (v *ObjectsView) renderCenteredFilePicker() string {
	pickerView := v.filePicker.View()
	pickerLines := strings.Split(pickerView, "\n")

	// Calculate vertical padding to center
	topPad := (v.height - len(pickerLines)) / 2
	if topPad < 0 {
		topPad = 0
	}

	var result strings.Builder
	// Add top padding
	for range topPad {
		result.WriteString("\n")
	}

	// Add centered picker lines
	for _, pLine := range pickerLines {
		pWidth := lipgloss.Width(pLine)
		leftPad := (v.width - pWidth) / 2
		if leftPad < 0 {
			leftPad = 0
		}
		result.WriteString(strings.Repeat(" ", leftPad))
		result.WriteString(pLine)
		result.WriteString("\n")
	}

	return result.String()
}

// overlayFilePicker renders the file picker centered over the content
func (v *ObjectsView) overlayFilePicker(content string) string {
	lines := strings.Split(content, "\n")
	pickerView := v.filePicker.View()
	pickerLines := strings.Split(pickerView, "\n")

	// Calculate vertical position (center)
	startRow := (len(lines) - len(pickerLines)) / 2
	if startRow < 0 {
		startRow = 0
	}

	// Overlay file picker lines
	for i, pLine := range pickerLines {
		row := startRow + i
		if row >= len(lines) {
			break
		}

		pWidth := lipgloss.Width(pLine)
		leftPad := (v.width - pWidth) / 2
		if leftPad < 0 {
			leftPad = 0
		}

		lines[row] = strings.Repeat(" ", leftPad) + pLine
	}

	return strings.Join(lines, "\n")
}

// overlayUploadProgress renders the upload progress bar centered over the content
func (v *ObjectsView) overlayUploadProgress(content string) string {
	lines := strings.Split(content, "\n")
	progressView := v.uploadProgress.View()
	progressLines := strings.Split(progressView, "\n")

	startRow := (len(lines) - len(progressLines)) / 2
	if startRow < 0 {
		startRow = 0
	}

	for i, pLine := range progressLines {
		row := startRow + i
		if row >= len(lines) {
			break
		}

		pWidth := lipgloss.Width(pLine)
		leftPad := (v.width - pWidth) / 2
		if leftPad < 0 {
			leftPad = 0
		}

		lines[row] = strings.Repeat(" ", leftPad) + pLine
	}

	return strings.Join(lines, "\n")
}

// prepareDelete initiates the delete flow
func (v *ObjectsView) prepareDelete(obj gcp.StorageObject) tea.Cmd { //nolint:gocritic // Copying object is acceptable
	return func() tea.Msg {
		return deleteRequestMsg{object: obj}
	}
}

// resolveDeleteFiles lists all files under a folder prefix
func (v *ObjectsView) resolveDeleteFiles(folder gcp.StorageObject) tea.Cmd { //nolint:gocritic // Copying object is acceptable
	return func() tea.Msg {
		objects, err := v.storageClient.ListAllObjects(
			gocontext.Background(),
			v.bucketName,
			folder.Name,
		)
		if err != nil {
			return deleteFilesResolvedMsg{err: err}
		}
		return deleteFilesResolvedMsg{files: objects}
	}
}

// createDeleteConfirmDialog creates the confirmation dialog for deletion
func (v *ObjectsView) createDeleteConfirmDialog(files []gcp.StorageObject) *confirm.ConfirmDialog {
	var title, message string
	var details []string

	if len(files) == 1 {
		title = "Delete File"
		message = fmt.Sprintf("Are you sure you want to delete '%s'?", files[0].DisplayName)
	} else {
		title = "Delete Folder"
		message = fmt.Sprintf("Are you sure you want to delete %d files?", len(files))
		// Show first few files as details
		maxDetails := 5
		for i, f := range files {
			if i >= maxDetails {
				details = append(details, fmt.Sprintf("... and %d more", len(files)-maxDetails))
				break
			}
			details = append(details, f.DisplayName)
		}
	}

	dialog := confirm.New(title, message, details)
	dialog.SetSize(v.width-20, 15)
	return dialog
}

// startDelete begins the deletion process in a background goroutine
func (v *ObjectsView) startDelete() tea.Cmd {
	files := v.pendingDeleteFiles
	if len(files) == 0 {
		return func() tea.Msg {
			return deleteCompleteMsg{}
		}
	}

	v.deleteChan = make(chan deleteProgressUpdate, 10)

	go func() {
		for i, file := range files {
			// Send progress before each delete (non-blocking)
			select {
			case v.deleteChan <- deleteProgressUpdate{
				deletedCount: i,
				totalCount:   len(files),
				currentFile:  file.DisplayName,
			}:
			default:
			}

			err := v.storageClient.DeleteObject(
				gocontext.Background(),
				v.bucketName,
				file.Name,
			)
			if err != nil {
				// Send error completion
				select {
				case v.deleteChan <- deleteProgressUpdate{
					done:         true,
					err:          err,
					deletedCount: i,
					failedObject: file.DisplayName,
				}:
				default:
				}
				return
			}
		}

		// Send completion signal
		select {
		case v.deleteChan <- deleteProgressUpdate{
			done:         true,
			deletedCount: len(files),
		}:
		default:
		}
	}()

	return v.waitForDeleteProgress()
}

// waitForDeleteProgress waits for delete progress updates from the goroutine
func (v *ObjectsView) waitForDeleteProgress() tea.Cmd {
	return func() tea.Msg {
		if v.deleteChan == nil {
			return deleteCompleteMsg{}
		}
		update, ok := <-v.deleteChan
		if !ok {
			return deleteCompleteMsg{}
		}
		if update.done {
			return deleteCompleteMsg{
				err:          update.err,
				deletedCount: update.deletedCount,
				failedObject: update.failedObject,
			}
		}
		return deleteProgressMsg{
			deletedCount: update.deletedCount,
			totalCount:   update.totalCount,
			currentFile:  update.currentFile,
		}
	}
}

// overlayDeleteConfirm renders the delete confirmation dialog centered over the content
func (v *ObjectsView) overlayDeleteConfirm(content string) string {
	lines := strings.Split(content, "\n")
	dialogView := v.deleteConfirm.View()
	dialogLines := strings.Split(dialogView, "\n")

	startRow := (len(lines) - len(dialogLines)) / 2
	if startRow < 0 {
		startRow = 0
	}

	for i, dLine := range dialogLines {
		row := startRow + i
		if row >= len(lines) {
			break
		}

		dWidth := lipgloss.Width(dLine)
		leftPad := (v.width - dWidth) / 2
		if leftPad < 0 {
			leftPad = 0
		}

		lines[row] = strings.Repeat(" ", leftPad) + dLine
	}

	return strings.Join(lines, "\n")
}

// overlayDeleteProgress renders the delete progress bar centered over the content
func (v *ObjectsView) overlayDeleteProgress(content string) string {
	lines := strings.Split(content, "\n")
	progressView := v.deleteProgress.View()
	progressLines := strings.Split(progressView, "\n")

	startRow := (len(lines) - len(progressLines)) / 2
	if startRow < 0 {
		startRow = 0
	}

	for i, pLine := range progressLines {
		row := startRow + i
		if row >= len(lines) {
			break
		}

		pWidth := lipgloss.Width(pLine)
		leftPad := (v.width - pWidth) / 2
		if leftPad < 0 {
			leftPad = 0
		}

		lines[row] = strings.Repeat(" ", leftPad) + pLine
	}

	return strings.Join(lines, "\n")
}

// overlayActionMenu renders the action menu centered over the content
func (v *ObjectsView) overlayActionMenu(content string) string {
	lines := strings.Split(content, "\n")
	menuView := v.actionMenu.View()
	menuLines := strings.Split(menuView, "\n")

	startRow := (len(lines) - len(menuLines)) / 2
	if startRow < 0 {
		startRow = 0
	}

	for i, mLine := range menuLines {
		row := startRow + i
		if row >= len(lines) {
			break
		}

		mWidth := lipgloss.Width(mLine)
		leftPad := (v.width - mWidth) / 2
		if leftPad < 0 {
			leftPad = 0
		}

		lines[row] = strings.Repeat(" ", leftPad) + mLine
	}

	return strings.Join(lines, "\n")
}

// buildObjectActions creates action menu items for the selected object
//
//nolint:gocritic // hugeParam: StorageObject struct passed by value for clarity
func (v *ObjectsView) buildObjectActions(obj gcp.StorageObject) []actionmenu.Action {
	if obj.IsFolder {
		return []actionmenu.Action{
			{Key: 'o', Label: "Open folder", Enabled: true},
			{Key: 'd', Label: "Download", Enabled: true},
			{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true},
		}
	}

	// File actions - include preview and open options
	previewable := isFilePreviewable(obj.ContentType, obj.Size, obj.Name)

	return []actionmenu.Action{
		{Key: 'v', Label: "Preview", Enabled: previewable},
		{Key: 'O', Label: "Open with app", Enabled: true},
		{Key: 'o', Label: "View details", Enabled: true},
		{Key: 'd', Label: "Download", Enabled: true},
		{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true},
	}
}

// isFilePreviewable checks if a file can be previewed based on content type, size, and name
func isFilePreviewable(contentType string, size int64, name string) bool {
	// Max preview size: 500KB
	const maxPreviewBytes = 500 * 1024

	if size > maxPreviewBytes {
		return false
	}

	// Check content type
	ct := strings.ToLower(contentType)
	if strings.HasPrefix(ct, "text/") {
		return true
	}
	if ct == "application/json" || ct == "application/xml" ||
		ct == "application/javascript" || ct == "application/x-yaml" {
		return true
	}

	// Check file extension
	nameLower := strings.ToLower(name)
	previewableExts := []string{
		".txt", ".md", ".log", ".yaml", ".yml", ".json", ".xml",
		".html", ".css", ".js", ".ts", ".go", ".py", ".sh", ".bash",
		".rb", ".rs", ".java", ".c", ".cpp", ".h", ".hpp",
		".sql", ".csv", ".toml", ".ini", ".cfg", ".conf",
	}
	for _, ext := range previewableExts {
		if strings.HasSuffix(nameLower, ext) {
			return true
		}
	}

	return false
}

// executeMenuAction performs the action selected from the menu
func (v *ObjectsView) executeMenuAction(actionKey rune) tea.Cmd {
	row := v.table.SelectedRow()
	if row == nil {
		return nil
	}

	obj := v.findObjectByName(row.ID)
	if obj == nil {
		return nil
	}

	switch actionKey {
	case 'v':
		// Preview - open details view with preview action
		if !obj.IsFolder {
			selectedObj := *obj
			return func() tea.Msg {
				return ObjectSelectedMsg{Object: selectedObj, Action: ObjectActionPreview}
			}
		}
	case 'O':
		// Open with app - open details view with open action
		if !obj.IsFolder {
			selectedObj := *obj
			return func() tea.Msg {
				return ObjectSelectedMsg{Object: selectedObj, Action: ObjectActionOpen}
			}
		}
	case 'o':
		if obj.IsFolder {
			// Navigate into folder
			v.prefixStack = append(v.prefixStack, v.currentPrefix)
			v.currentPrefix = obj.Name
			v.resetScrollState()
			v.loading = true
			return tea.Batch(v.spinner.Tick, v.loadObjects())
		}
		// For files, open details view
		selectedObj := *obj
		return func() tea.Msg {
			return ObjectSelectedMsg{Object: selectedObj}
		}
	case 'd':
		return v.prepareDownload(*obj)
	case 'D':
		return v.prepareDelete(*obj)
	}

	return nil
}

// IsMenuOpen returns true if the action menu is currently open
func (v *ObjectsView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
}

// GetStorageClient returns the storage client for reuse
func (v *ObjectsView) GetStorageClient() *gcp.StorageClient {
	return v.storageClient
}
