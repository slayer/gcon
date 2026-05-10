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
	Enter           key.Binding
	NavigateUp      key.Binding
	NavigateInto    key.Binding
	Refresh         key.Binding
	Download        key.Binding
	Upload          key.Binding
	Delete          key.Binding
	ActionMenu      key.Binding
	DeepScan        key.Binding
	ToggleSelect    key.Binding
	ToggleSelectAll key.Binding
}

// parentNavRowID is the sentinel row ID used for the synthetic ".." row that
// appears at the top of the table when the user is inside a subfolder. The
// leading "\n" makes collision with a real GCS object name impossible: per
// the Cloud Storage object naming rules an object name cannot contain a
// carriage return or line feed character.
// https://cloud.google.com/storage/docs/objects#naming
const parentNavRowID = "\n__gcon_parent_nav__"

func defaultObjectKeyMap() objectKeyMap {
	return objectKeyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open"),
		),
		NavigateUp: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "up one folder"),
		),
		NavigateInto: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "enter folder"),
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
		ToggleSelect: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle selection"),
		),
		ToggleSelectAll: key.NewBinding(
			key.WithKeys("*"),
			key.WithHelp("*", "select all (visible)"),
		),
	}
}

// Table column definitions for objects (no sorting or field filtering).
// The first "Sel" column is hidden by default and shown only while a bulk
// selection is active. Column index references (Data[i]) below assume this
// ordering — keep them in sync if columns are reordered.
const (
	objectColIndexSel         = 0
	objectColIndexName        = 1
	objectColIndexSize        = 2
	objectColIndexContentType = 3
	objectColIndexModified    = 4
)

func objectColumns() []table.Column {
	return []table.Column{
		{Title: "Sel", Width: 4},
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

	// Bulk selection: rows the user has marked with Space. Keyed by GCS
	// object full name (= row.ID). Cleared on every navigation gesture.
	// The "..", parent-nav row is never selectable.
	selectedIDs map[string]struct{}

	// Action menu kind. The ActionSelectedMsg handler dispatches differently
	// depending on which menu is open (single-row, bulk, or storage-class
	// picker).
	menuKind objectsMenuKind

	// Bulk-action snapshot. Captured when the user opens the bulk action
	// menu so subsequent toggles in the table don't mutate the list under
	// the operation. Cleared on cancel/completion.
	menuPendingObjects []gcp.StorageObject

	// Storage-class change state. Mirrors the delete state's shape so the
	// progress overlay rendering can reuse the same plumbing pattern.
	changingClass     bool
	changeFiles       []gcp.StorageObject
	changeClass       string
	changeProgress    *progress.Progress
	changeChan        chan storageClassProgressUpdate
}

// objectsMenuKind discriminates the active action menu.
type objectsMenuKind int

const (
	menuKindObject       objectsMenuKind = iota // single-row right-click style
	menuKindBulk                                // bulk-action menu over selectedObjects
	menuKindStorageClass                        // class picker spawned from bulk menu
)

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
		changeProgress:   progress.New(),
		selectedIDs:      make(map[string]struct{}),
	}
	v.Table = &v.table
	// Hide the Sel column until the user actually starts a multi-select.
	v.table.SetColumnHidden("Sel", true)
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

// parentPrefix returns the parent of a folder prefix:
//   - "folder1/folder2/" -> "folder1/"
//   - "folder1/"         -> ""
//   - ""                 -> "" (root has no parent)
func parentPrefix(prefix string) string {
	trimmed := strings.TrimSuffix(prefix, "/")
	if trimmed == "" {
		return ""
	}
	if i := strings.LastIndex(trimmed, "/"); i >= 0 {
		return trimmed[:i+1]
	}
	return ""
}

// parentNavRow builds the synthetic ".." row shown at the top of the table
// when the user is inside a subfolder. Its ID is parentNavRowID so callers
// can detect it without involving v.objects. The Sel cell stays empty —
// the parent row is never selectable.
func parentNavRow() table.Row {
	return table.Row{
		Data:        []string{"", symbols.Folder() + " ..", "-", "Parent folder", "-"},
		FilterValue: "..",
		ID:          parentNavRowID,
	}
}

// navigateUp moves to the parent folder of currentPrefix and reloads, mirroring
// the down-navigation pattern (push current to prefixStack so Esc takes the
// user back to where they came from). No-op at the bucket root.
func (v *ObjectsView) navigateUp() tea.Cmd {
	if v.currentPrefix == "" {
		return nil
	}
	v.prefixStack = append(v.prefixStack, v.currentPrefix)
	v.currentPrefix = parentPrefix(v.currentPrefix)
	v.beginNavigation()
	return tea.Batch(v.spinner.Tick, v.loadObjects())
}

// navigateInto enters the folder identified by its full GCS prefix
// (e.g. "folder1/folder2/"), pushing the current prefix onto the back-stack
// so Esc returns to the previous folder.
func (v *ObjectsView) navigateInto(folderPrefix string) tea.Cmd {
	v.prefixStack = append(v.prefixStack, v.currentPrefix)
	v.currentPrefix = folderPrefix
	v.beginNavigation()
	return tea.Batch(v.spinner.Tick, v.loadObjects())
}

// beginNavigation resets transient state shared by every navigation gesture
// (up, into, refresh): clears scroll/cursor, clears any stale errors that
// would otherwise short-circuit View() after a successful load, clears the
// bulk selection, and flips the loading flag so the spinner takes over.
func (v *ObjectsView) beginNavigation() {
	v.resetScrollState()
	v.err = nil
	v.loadMoreErr = nil
	v.loading = true
	v.clearSelection()
}

// toggleSelection flips the selection state for the cursor row. The ".."
// parent-nav row and folder rows that don't yet have resolved descendants
// are always selectable as containers — bulk operations resolve folders
// to their members before acting (mirrors the existing single-folder
// delete/download flow). Returns true if the selection state changed.
func (v *ObjectsView) toggleSelection() bool {
	row := v.table.SelectedRow()
	if row == nil || row.ID == parentNavRowID {
		return false
	}
	if v.selectedIDs == nil {
		v.selectedIDs = make(map[string]struct{})
	}
	if _, on := v.selectedIDs[row.ID]; on {
		delete(v.selectedIDs, row.ID)
	} else {
		v.selectedIDs[row.ID] = struct{}{}
	}
	v.refreshSelectionView()
	return true
}

// toggleSelectAll selects every visible (post-filter) non-".." row when
// any are unselected; otherwise clears the selection. Folders are
// included so the user can "select all and delete" a filtered set in one
// go.
func (v *ObjectsView) toggleSelectAll() {
	if v.selectedIDs == nil {
		v.selectedIDs = make(map[string]struct{})
	}
	rows := v.table.Rows()
	allSelected := true
	for _, r := range rows {
		if r.ID == parentNavRowID {
			continue
		}
		if _, on := v.selectedIDs[r.ID]; !on {
			allSelected = false
			break
		}
	}
	if allSelected {
		// Toggle off — clear the selection completely.
		v.selectedIDs = make(map[string]struct{})
	} else {
		for _, r := range rows {
			if r.ID == parentNavRowID {
				continue
			}
			v.selectedIDs[r.ID] = struct{}{}
		}
	}
	v.refreshSelectionView()
}

// clearSelection drops the selection and hides the Sel column.
func (v *ObjectsView) clearSelection() {
	if len(v.selectedIDs) == 0 {
		return
	}
	v.selectedIDs = make(map[string]struct{})
	v.refreshSelectionView()
}

// refreshSelectionView reflects the current selection state into the
// table: re-renders the Sel cell on every row, shows/hides the Sel
// column, and updates the status suffix with the "N selected" hint.
// Call after any selectedIDs mutation. The cursor and sort state are
// preserved across the SetRows.
func (v *ObjectsView) refreshSelectionView() {
	rows := v.table.Rows()
	for i := range rows {
		if rows[i].ID == parentNavRowID {
			continue
		}
		rows[i].Data[objectColIndexSel] = v.selMarkFor(rows[i].ID)
	}
	sortCol, sortAsc := v.table.SortState()
	cursor := v.table.SelectedIndex()
	v.table.SetRows(rows)
	if sortCol >= 0 {
		v.table.SortBy(sortCol, sortAsc)
	}
	v.table.SetCursor(cursor)
	v.table.SetColumnHidden("Sel", len(v.selectedIDs) == 0)
}

// objectToRow converts a GCS object to a table row. selMark is the cell
// content for the Sel column (e.g. "[✓]" or "[ ]"; empty when the column
// is hidden).
func objectToRow(obj gcp.StorageObject, selMark string) table.Row { //nolint:gocritic // Copying object is acceptable
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
		Data:        []string{selMark, name, size, contentType, modified},
		FilterValue: obj.DisplayName + " " + contentType,
		ID:          obj.Name, // Full path name for lookup
	}
}

// selMarkFor returns the Sel-column cell for an object in the current
// selection state. Empty when the bulk-select UI is hidden.
func (v *ObjectsView) selMarkFor(objName string) string {
	if len(v.selectedIDs) == 0 {
		return ""
	}
	if _, ok := v.selectedIDs[objName]; ok {
		return "[✓]"
	}
	return "[ ]"
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

// Storage-class change messages (bulk).
type storageClassStartMsg struct {
	class string
	files []gcp.StorageObject
}

type storageClassProgressUpdate struct {
	doneCount    int
	totalCount   int
	currentFile  string
	done         bool
	err          error
	failedObject string
}

type storageClassProgressMsg struct {
	doneCount   int
	totalCount  int
	currentFile string
}

type storageClassCompleteMsg struct {
	err          error
	doneCount    int
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

		// Convert to table rows. Prepend a synthetic ".." row so users can
		// navigate up by pressing Enter on it (left arrow does the same).
		rows := make([]table.Row, 0, len(msg.objects)+1)
		if v.currentPrefix != "" {
			rows = append(rows, parentNavRow())
		}
		for _, obj := range msg.objects {
			rows = append(rows, objectToRow(obj, v.selMarkFor(obj.Name)))
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
			newRows[i] = objectToRow(obj, v.selMarkFor(obj.Name))
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
		// Handle double-click on table row - navigate up if it's the parent
		// nav row, else navigate into the folder.
		if msg.RowID == parentNavRowID {
			return v.navigateUp()
		}
		obj := v.findObjectByName(msg.RowID)
		if obj != nil && obj.IsFolder {
			return v.navigateInto(obj.Name)
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
			v.beginNavigation()
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
			v.beginNavigation()
			return tea.Batch(v.spinner.Tick, v.loadObjects())
		}
		return nil

	case storageClassStartMsg:
		v.changingClass = true
		v.changeFiles = msg.files
		v.changeClass = msg.class
		v.changeProgress.Start()
		v.changeProgress.SetProgress(
			fmt.Sprintf("Setting class %s", msg.class),
			msg.files[0].DisplayName,
			1,
			len(msg.files),
			0,
			int64(len(msg.files)),
		)
		v.changeProgress.SetSize(v.width)
		return v.startStorageClassChange()

	case storageClassProgressMsg:
		v.changeProgress.SetProgress(
			fmt.Sprintf("Setting class %s", v.changeClass),
			msg.currentFile,
			msg.doneCount+1,
			msg.totalCount,
			int64(msg.doneCount),
			int64(msg.totalCount),
		)
		return v.waitForStorageClassProgress()

	case storageClassCompleteMsg:
		v.changingClass = false
		if v.changeChan != nil {
			close(v.changeChan)
			v.changeChan = nil
		}
		files := v.changeFiles
		v.changeFiles = nil
		class := v.changeClass
		v.changeClass = ""
		if msg.err != nil {
			if msg.doneCount > 0 {
				v.err = fmt.Errorf("set class on %d files, failed on %s: %w", msg.doneCount, msg.failedObject, msg.err)
			} else {
				v.err = msg.err
			}
			return nil
		}
		// Refresh list — storage class changes affect the visible Class column
		// (in the details view) and the new generation invalidates any cached
		// metadata.
		_ = files
		_ = class
		v.beginNavigation()
		return tea.Batch(v.spinner.Tick, v.loadObjects())

	case actionmenu.ActionSelectedMsg:
		// Handle action menu selection. The menu is closed up front;
		// individual handlers may re-open a different menu (e.g. the
		// storage-class picker) by setting v.actionMenu and v.menuOpen.
		v.menuOpen = false
		switch v.menuKind {
		case menuKindBulk:
			return v.executeBulkAction(msg.Key)
		case menuKindStorageClass:
			return v.executeStorageClassPick(msg.Key)
		default:
			return v.executeMenuAction(msg.Key)
		}

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

		// Don't handle keys during loading or any in-flight bulk operation.
		if v.loading || v.downloading || v.uploading || v.deleting || v.changingClass {
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
			// Bulk-actions menu when a selection is active; otherwise the
			// per-row menu for whatever the cursor is on.
			if sel := v.selectedObjects(); len(sel) > 0 {
				v.menuKind = menuKindBulk
				v.menuPendingObjects = sel
				title := fmt.Sprintf("Bulk actions on %d items", len(sel))
				v.actionMenu = actionmenu.New(title, buildBulkActions())
				v.menuOpen = true
				return nil
			}
			if row := v.table.SelectedRow(); row != nil {
				obj := v.findObjectByName(row.ID)
				if obj != nil {
					v.menuKind = menuKindObject
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
			// Bulk delete when a selection is active; otherwise the cursor row.
			if sel := v.selectedObjects(); len(sel) > 0 {
				return v.prepareBulkDelete(sel)
			}
			if row := v.table.SelectedRow(); row != nil {
				obj := v.findObjectByName(row.ID)
				if obj != nil {
					return v.prepareDelete(*obj)
				}
			}

		case key.Matches(msg, v.keys.Download):
			// Bulk download when a selection is active; otherwise the cursor row.
			if sel := v.selectedObjects(); len(sel) > 0 {
				return v.prepareBulkDownload(sel)
			}
			if row := v.table.SelectedRow(); row != nil {
				obj := v.findObjectByName(row.ID)
				if obj != nil {
					return v.prepareDownload(*obj)
				}
			}

		case key.Matches(msg, v.keys.NavigateUp):
			return v.navigateUp()

		case key.Matches(msg, v.keys.NavigateInto):
			// Right arrow: enter the selected folder. No-op on files and on
			// the ".." row — Right is a "drill-down" gesture, not the
			// generic open-or-up that Enter performs.
			if row := v.table.SelectedRow(); row != nil && row.ID != parentNavRowID {
				obj := v.findObjectByName(row.ID)
				if obj != nil && obj.IsFolder {
					return v.navigateInto(obj.Name)
				}
			}
			return nil

		case key.Matches(msg, v.keys.Enter):
			// Navigate into folder, up to parent (".." row), or view file
			// details on Enter.
			if row := v.table.SelectedRow(); row != nil {
				if row.ID == parentNavRowID {
					return v.navigateUp()
				}
				obj := v.findObjectByName(row.ID)
				if obj != nil {
					if obj.IsFolder {
						return v.navigateInto(obj.Name)
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
			v.beginNavigation()
			return tea.Batch(v.spinner.Tick, v.loadObjects())

		case key.Matches(msg, v.keys.ToggleSelect):
			v.toggleSelection()
			return nil

		case key.Matches(msg, v.keys.ToggleSelectAll):
			v.toggleSelectAll()
			return nil
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

// selectedObjects returns the StorageObject values for every entry in
// selectedIDs, in the order they appear in v.objects (so file-listing order
// is preserved). The ".." synthetic row is intentionally never in the
// selection set.
func (v *ObjectsView) selectedObjects() []gcp.StorageObject {
	if len(v.selectedIDs) == 0 {
		return nil
	}
	out := make([]gcp.StorageObject, 0, len(v.selectedIDs))
	for i := range v.objects {
		if _, on := v.selectedIDs[v.objects[i].Name]; on {
			out = append(out, v.objects[i])
		}
	}
	return out
}

// applyFolderUsageToTable walks the current table rows; for each folder row,
// looks up its recursive size in v.folderUsage.ByTopPrefix and updates the
// Size cell in place. Files are left untouched. Called after a deep-scan
// ReadyMsg arrives. Sort state is preserved by snapshotting before SetRows.
//
// Builds an O(1)-lookup map from object Name -> *StorageObject once, so total
// cost is O(rows + objects) instead of O(rows * objects). With 10k objects in
// a folder a naive nested scan was ~100M comparisons per call.
func (v *ObjectsView) applyFolderUsageToTable() {
	if v.folderUsage == nil || v.folderUsage.Source != usage.SourceDeepScan {
		return
	}
	if len(v.folderUsage.ByTopPrefix) == 0 {
		return
	}
	objIndex := make(map[string]*gcp.StorageObject, len(v.objects))
	for i := range v.objects {
		objIndex[v.objects[i].Name] = &v.objects[i]
	}
	rows := v.table.Rows()
	for i := range rows {
		obj, ok := objIndex[rows[i].ID]
		if !ok || !obj.IsFolder {
			continue
		}
		key := folderUsageKey(rows[i].ID, v.folderUsage.Prefix)
		stat, ok := v.folderUsage.ByTopPrefix[key]
		if !ok {
			continue // no data for this folder
		}
		// Size cell is at objectColIndexSize per objectColumns().
		rows[i].Data[objectColIndexSize] = gcp.FormatSize(stat.Bytes) + " ✓"
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
	// If a bulk selection is active, clear it first; users typically want to
	// abort the selection, not navigate away. A second Esc then navigates.
	if len(v.selectedIDs) > 0 {
		v.clearSelection()
		return true, nil
	}
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
// Prepends a "N selected" hint when a bulk selection is active so the count
// is always visible alongside scroll state.
func (v *ObjectsView) scrollInfo() string {
	var base string
	switch {
	case v.loadMoreErr != nil:
		base = "(load error, press r to retry)"
	case v.loadingMore:
		base = "(loading more...)"
	case len(v.objects) >= maxLoadedObjects && v.nextPageToken != "":
		base = fmt.Sprintf("(showing first %d, use filter to narrow)", maxLoadedObjects)
	case v.allLoaded:
		base = "(all loaded)"
	case v.nextPageToken != "":
		base = "(scroll for more)"
	}
	if n := len(v.selectedIDs); n > 0 {
		hint := fmt.Sprintf("[%d selected]", n)
		if base == "" {
			return hint
		}
		return hint + " " + base
	}
	return base
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

	// Only show the bare "empty" message at the bucket root. In a subfolder,
	// the synthetic ".." row keeps the table non-empty so the user can still
	// navigate up via Enter on it (or Left arrow).
	if len(v.objects) == 0 && v.currentPrefix == "" {
		// When file picker is shown, render it directly (not as overlay)
		// since empty bucket content is too short for proper overlay
		if v.showFilePicker && v.filePicker != nil {
			return v.renderCenteredFilePicker()
		}
		return "\n  This bucket is empty.\n  Press 'u' to upload files, 'esc' to go back."
	}

	// Keep table title and status in sync with current state
	v.table.SetTitle(v.buildTitle())
	v.table.SetStatusSuffix(v.scrollInfo())

	// Help text for actions. Right arrow is shown unconditionally (always
	// useful for drilling into folders); left/.. are only relevant in a
	// subfolder. When a bulk selection is active, advertise the bulk gesture
	// keys instead of the per-row hint.
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	var helpText string
	switch {
	case len(v.selectedIDs) > 0:
		helpText = fmt.Sprintf("\n  space: toggle • *: select all • d: download • D: delete • .: bulk menu • esc: clear (%d selected)", len(v.selectedIDs))
	case v.currentPrefix != "":
		helpText = "\n  enter: open (or .. to go up) • ←: up • →: enter folder • space: select • d: download • u: upload • D: delete • .: menu • /: filter • r: refresh • esc: back"
	default:
		helpText = "\n  enter: open • →: enter folder • space: select • d: download • u: upload • D: delete • .: menu • /: filter • r: refresh • esc: back"
	}
	help := helpStyle.Render(helpText)

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

	// Overlay progress bar while changing storage class
	if v.changingClass {
		content = v.overlayStorageClassProgress(content)
	}

	return content
}

// overlayStorageClassProgress renders the storage-class change progress bar
// centered over the content.
func (v *ObjectsView) overlayStorageClassProgress(content string) string {
	lines := strings.Split(content, "\n")
	progressView := v.changeProgress.View()
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
		lines[row] = pLine
	}
	return strings.Join(lines, "\n")
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

// prepareBulkDownload initiates a multi-object download. Folder entries are
// expanded to their recursive members; duplicates are deduped. The existing
// startDownload progress flow then iterates the full list. An empty result
// (only-empty folders selected) reports a friendly error.
func (v *ObjectsView) prepareBulkDownload(objs []gcp.StorageObject) tea.Cmd {
	bucket := v.bucketName
	client := v.storageClient
	captured := append([]gcp.StorageObject(nil), objs...)
	return func() tea.Msg {
		ctx := gocontext.Background()
		seen := make(map[string]struct{}, len(captured))
		var combined []gcp.StorageObject
		for _, o := range captured {
			if !o.IsFolder {
				if _, dup := seen[o.Name]; dup {
					continue
				}
				seen[o.Name] = struct{}{}
				combined = append(combined, o)
				continue
			}
			members, err := client.ListAllObjects(ctx, bucket, o.Name)
			if err != nil {
				return downloadCompleteMsg{err: fmt.Errorf("list %s: %w", o.Name, err)}
			}
			for _, m := range members {
				if _, dup := seen[m.Name]; dup {
					continue
				}
				seen[m.Name] = struct{}{}
				combined = append(combined, m)
			}
		}
		if len(combined) == 0 {
			return downloadCompleteMsg{err: uierrors.ErrFolderEmpty}
		}
		return downloadStartMsg{files: combined}
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

// prepareBulkDelete kicks off resolution + confirmation for a multi-object
// delete. Files in the input are kept as-is; folders are resolved to their
// recursive contents via ListAllObjects. Returns a single
// deleteFilesResolvedMsg with the combined list, so the existing delete
// flow (confirm dialog → progress overlay → completion) handles the rest.
func (v *ObjectsView) prepareBulkDelete(objs []gcp.StorageObject) tea.Cmd {
	bucket := v.bucketName
	client := v.storageClient
	captured := append([]gcp.StorageObject(nil), objs...)
	return func() tea.Msg {
		ctx := gocontext.Background()
		seen := make(map[string]struct{}, len(captured))
		var combined []gcp.StorageObject
		for _, o := range captured {
			if !o.IsFolder {
				if _, dup := seen[o.Name]; dup {
					continue
				}
				seen[o.Name] = struct{}{}
				combined = append(combined, o)
				continue
			}
			members, err := client.ListAllObjects(ctx, bucket, o.Name)
			if err != nil {
				return deleteFilesResolvedMsg{err: fmt.Errorf("list %s: %w", o.Name, err)}
			}
			for _, m := range members {
				if _, dup := seen[m.Name]; dup {
					continue
				}
				seen[m.Name] = struct{}{}
				combined = append(combined, m)
			}
		}
		return deleteFilesResolvedMsg{files: combined}
	}
}

// createDeleteConfirmDialog creates the confirmation dialog for deletion.
// Title/message adjust for the single-file vs multi-object case.
func (v *ObjectsView) createDeleteConfirmDialog(files []gcp.StorageObject) *confirm.ConfirmDialog {
	var title, message string
	var details []string

	switch {
	case len(files) == 1:
		title = "Delete File"
		message = fmt.Sprintf("Are you sure you want to delete '%s'?", files[0].DisplayName)
	default:
		title = "Delete Files"
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

// buildBulkActions returns the action items shown when the user opens the
// action menu while a bulk selection is active.
func buildBulkActions() []actionmenu.Action {
	return []actionmenu.Action{
		{Key: 'd', Label: "Download", Enabled: true},
		{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true},
		{Key: 's', Label: "Change storage class", Enabled: true},
	}
}

// buildStorageClassActions returns the action items shown when the user
// is picking a destination storage class for a bulk class-change.
func buildStorageClassActions() []actionmenu.Action {
	return []actionmenu.Action{
		{Key: '1', Label: "STANDARD", Enabled: true},
		{Key: '2', Label: "NEARLINE", Enabled: true},
		{Key: '3', Label: "COLDLINE", Enabled: true},
		{Key: '4', Label: "ARCHIVE", Enabled: true},
	}
}

// executeBulkAction handles a selection from the bulk-actions menu.
func (v *ObjectsView) executeBulkAction(actionKey rune) tea.Cmd {
	objs := v.menuPendingObjects
	if len(objs) == 0 {
		v.menuKind = menuKindObject
		return nil
	}
	switch actionKey {
	case 'd':
		v.menuKind = menuKindObject
		return v.prepareBulkDownload(objs)
	case 'D':
		v.menuKind = menuKindObject
		return v.prepareBulkDelete(objs)
	case 's':
		// Re-open the action menu as a class picker. menuPendingObjects
		// stays set so the picker's selection knows what to operate on.
		v.menuKind = menuKindStorageClass
		v.actionMenu = actionmenu.New("Set storage class", buildStorageClassActions())
		v.menuOpen = true
		return nil
	}
	v.menuKind = menuKindObject
	return nil
}

// executeStorageClassPick handles a selection from the class picker.
func (v *ObjectsView) executeStorageClassPick(actionKey rune) tea.Cmd {
	classByKey := map[rune]string{
		'1': "STANDARD",
		'2': "NEARLINE",
		'3': "COLDLINE",
		'4': "ARCHIVE",
	}
	class, ok := classByKey[actionKey]
	if !ok {
		v.menuKind = menuKindObject
		return nil
	}
	objs := v.menuPendingObjects
	v.menuPendingObjects = nil
	v.menuKind = menuKindObject
	if len(objs) == 0 {
		return nil
	}
	return v.prepareBulkStorageClassChange(objs, class)
}

// prepareBulkStorageClassChange resolves any folder entries to their
// recursive members (folders themselves don't have a storage class) and
// dispatches a storageClassStartMsg with the file-only list.
func (v *ObjectsView) prepareBulkStorageClassChange(objs []gcp.StorageObject, class string) tea.Cmd {
	bucket := v.bucketName
	client := v.storageClient
	captured := append([]gcp.StorageObject(nil), objs...)
	return func() tea.Msg {
		ctx := gocontext.Background()
		seen := make(map[string]struct{}, len(captured))
		var files []gcp.StorageObject
		for _, o := range captured {
			if !o.IsFolder {
				if _, dup := seen[o.Name]; dup {
					continue
				}
				seen[o.Name] = struct{}{}
				files = append(files, o)
				continue
			}
			members, err := client.ListAllObjects(ctx, bucket, o.Name)
			if err != nil {
				return storageClassCompleteMsg{err: fmt.Errorf("list %s: %w", o.Name, err)}
			}
			for _, m := range members {
				if _, dup := seen[m.Name]; dup {
					continue
				}
				seen[m.Name] = struct{}{}
				files = append(files, m)
			}
		}
		if len(files) == 0 {
			return storageClassCompleteMsg{err: uierrors.ErrFolderEmpty}
		}
		return storageClassStartMsg{class: class, files: files}
	}
}

// startStorageClassChange spawns a goroutine that iterates files and
// applies the new storage class to each, sending progress on changeChan.
func (v *ObjectsView) startStorageClassChange() tea.Cmd {
	files := v.changeFiles
	class := v.changeClass
	if len(files) == 0 || class == "" {
		return func() tea.Msg { return storageClassCompleteMsg{} }
	}
	v.changeChan = make(chan storageClassProgressUpdate, 10)
	go func() {
		for i, f := range files {
			select {
			case v.changeChan <- storageClassProgressUpdate{
				doneCount:   i,
				totalCount:  len(files),
				currentFile: f.DisplayName,
			}:
			default:
			}
			if err := v.storageClient.UpdateObjectStorageClass(
				gocontext.Background(),
				v.bucketName,
				f.Name,
				class,
			); err != nil {
				select {
				case v.changeChan <- storageClassProgressUpdate{
					done:         true,
					err:          err,
					doneCount:    i,
					failedObject: f.DisplayName,
				}:
				default:
				}
				return
			}
		}
		select {
		case v.changeChan <- storageClassProgressUpdate{
			done:      true,
			doneCount: len(files),
		}:
		default:
		}
	}()
	return v.waitForStorageClassProgress()
}

// waitForStorageClassProgress blocks on the next update from changeChan and
// translates it into a Bubble Tea message.
func (v *ObjectsView) waitForStorageClassProgress() tea.Cmd {
	return func() tea.Msg {
		if v.changeChan == nil {
			return storageClassCompleteMsg{}
		}
		update, ok := <-v.changeChan
		if !ok {
			return storageClassCompleteMsg{}
		}
		if update.done {
			return storageClassCompleteMsg{
				err:          update.err,
				doneCount:    update.doneCount,
				failedObject: update.failedObject,
			}
		}
		return storageClassProgressMsg{
			doneCount:   update.doneCount,
			totalCount:  update.totalCount,
			currentFile: update.currentFile,
		}
	}
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
