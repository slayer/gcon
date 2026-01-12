package views

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/filepicker"
	"github.com/slayer/gcon/internal/ui/components/progress"
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

	// Pagination state (using master's improved token handling)
	currentPage      int
	nextPageToken    string   // Token for loading next page
	currentPageToken string   // Token used to load current page (empty for first page)
	pageTokenHistory []string // History of page tokens used (for back navigation)
	hasMore          bool
	totalLoaded      int // Total objects loaded across pages

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
}

// objectKeyMap defines object-specific key bindings
type objectKeyMap struct {
	Enter    key.Binding
	Refresh  key.Binding
	NextPage key.Binding
	PrevPage key.Binding
	Download key.Binding
	Upload   key.Binding
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
		Download: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "download"),
		),
		Upload: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "upload"),
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
		return []key.Binding{km.Enter, km.Download, km.Upload, km.Refresh, km.NextPage, km.PrevPage}
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	return &ObjectsView{
		storageClient:    storageClient,
		bucketName:       bucketName,
		currentPrefix:    "",
		prefixStack:      make([]string, 0),
		list:             l,
		spinner:          s,
		loading:          true,
		keys:             defaultObjectKeyMap(),
		currentPage:      1,
		pageTokenHistory: make([]string, 0),
		downloadProgress: progress.New(),
		uploadProgress:   progress.New(),
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

// Update handles messages for the objects view
func (v *ObjectsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case objectsLoadedMsg:
		v.loading = false
		v.objects = msg.objects
		v.nextPageToken = msg.nextToken
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

	case downloadStartMsg:
		v.downloading = true
		v.downloadFiles = msg.files
		v.downloadIndex = 0

		// Calculate total size for progress
		var totalSize int64
		for _, f := range msg.files {
			totalSize += f.Size
		}

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
		// User cancelled file picker
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
			v.loading = true
			return tea.Batch(v.spinner.Tick, v.loadObjects(""))
		}
		return nil

	case tea.KeyMsg:
		// If file picker is shown, delegate to it
		if v.showFilePicker && v.filePicker != nil {
			cmd := v.filePicker.Update(msg)
			return cmd
		}

		// Don't handle keys during loading, downloading, or uploading
		if v.loading || v.downloading || v.uploading {
			return nil
		}

		// If list is filtering, let it handle all keys except our shortcuts
		// Check upload key first since it should work even in empty buckets
		if key.Matches(msg, v.keys.Upload) {
			// Open file picker for upload
			cwd, _ := os.Getwd()
			v.filePicker = filepicker.New(cwd, true)
			v.filePicker.SetSize(v.width-10, v.height-10)
			v.showFilePicker = true
			return v.filePicker.Init()
		}

		// If list is in filtering mode, delegate to list
		if v.list.FilterState() == list.Filtering {
			var cmd tea.Cmd
			v.list, cmd = v.list.Update(msg)
			return cmd
		}

		switch {
		case key.Matches(msg, v.keys.Download):
			// Download selected file or folder
			if item, ok := v.list.SelectedItem().(objectItem); ok {
				return v.prepareDownload(item.object)
			}

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

	default:
		// Forward other messages to file picker when active (for async directory loading)
		if v.showFilePicker && v.filePicker != nil {
			cmd := v.filePicker.Update(msg)
			return cmd
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
	v.nextPageToken = ""
	v.currentPageToken = ""
	v.pageTokenHistory = make([]string, 0)
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
		return fmt.Sprintf("\n  %s\n  Press 'u' to upload files, 'esc' to go back.", msg)
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
	help := statusStyle.Render("\n  enter: open • d: download • u: upload • n/p: next/prev page • r: refresh • /: filter • esc: back")

	content := v.list.View() + "\n" + status + help

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

	return content
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

// prepareDownload initiates download for a file or folder
func (v *ObjectsView) prepareDownload(obj gcp.StorageObject) tea.Cmd {
	return func() tea.Msg {
		if obj.IsFolder {
			// For folders, list all objects recursively
			objects, err := v.storageClient.ListAllObjects(
				context.Background(),
				v.bucketName,
				obj.Name,
			)
			if err != nil {
				return downloadCompleteMsg{err: err}
			}
			if len(objects) == 0 {
				return downloadCompleteMsg{err: fmt.Errorf("folder is empty")}
			}
			return downloadStartMsg{files: objects}
		}
		// Single file download
		return downloadStartMsg{files: []gcp.StorageObject{obj}}
	}
}

// startDownload begins the actual download process
func (v *ObjectsView) startDownload() tea.Cmd {
	return func() tea.Msg {
		files := v.downloadFiles
		if len(files) == 0 {
			return downloadCompleteMsg{err: nil}
		}

		// Get current working directory for downloads
		cwd, err := os.Getwd()
		if err != nil {
			return downloadCompleteMsg{err: fmt.Errorf("failed to get working directory: %w", err)}
		}

		// Create channel here (inside goroutine) to avoid leak on early error above
		v.downloadChan = make(chan progress.ProgressUpdate, 10)

		// Calculate total size
		var totalSize int64
		for _, f := range files {
			totalSize += f.Size
		}

		var bytesDownloaded int64

		for i, file := range files {
			// Determine local path - preserve folder structure for multi-file downloads
			localPath := filepath.Join(cwd, file.DisplayName)
			if len(files) > 1 {
				// For folder downloads, preserve relative path structure
				localPath = filepath.Join(cwd, file.Name)
			}

			// Send progress update before starting this file
			v.downloadChan <- progress.ProgressUpdate{
				BytesTransferred: bytesDownloaded,
				TotalBytes:       totalSize,
				CurrentFile:      file.DisplayName,
				CurrentFileNum:   i + 1,
				TotalFiles:       len(files),
			}

			// Download with progress callback
			err := v.storageClient.DownloadObject(
				context.Background(),
				v.bucketName,
				file.Name,
				localPath,
				func(transferred, total int64) {
					v.downloadChan <- progress.ProgressUpdate{
						BytesTransferred: bytesDownloaded + transferred,
						TotalBytes:       totalSize,
						CurrentFile:      file.DisplayName,
						CurrentFileNum:   i + 1,
						TotalFiles:       len(files),
					}
				},
			)
			if err != nil {
				return downloadCompleteMsg{err: fmt.Errorf("failed to download %s: %w", file.DisplayName, err)}
			}

			bytesDownloaded += file.Size
		}

		return downloadCompleteMsg{err: nil}
	}
}

// waitForProgress waits for the next progress update from the download goroutine
func (v *ObjectsView) waitForProgress() tea.Cmd {
	return func() tea.Msg {
		if v.downloadChan == nil {
			return nil
		}
		update, ok := <-v.downloadChan
		if !ok {
			return nil
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

// startUpload begins the actual upload process
func (v *ObjectsView) startUpload() tea.Cmd {
	return func() tea.Msg {
		files := v.uploadFiles
		if len(files) == 0 {
			return uploadCompleteMsg{err: nil}
		}

		// Calculate total size and collect file info
		type uploadFile struct {
			localPath  string
			remotePath string
			size       int64
		}
		var uploadFiles []uploadFile
		var totalSize int64

		for _, path := range files {
			info, err := os.Stat(path)
			if err != nil {
				return uploadCompleteMsg{err: fmt.Errorf("failed to stat %s: %w", path, err)}
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
						remotePath := v.currentPrefix + relPath
						uploadFiles = append(uploadFiles, uploadFile{
							localPath:  filePath,
							remotePath: remotePath,
							size:       fi.Size(),
						})
						totalSize += fi.Size()
					}
					return nil
				})
				if err != nil {
					return uploadCompleteMsg{err: fmt.Errorf("failed to walk directory %s: %w", path, err)}
				}
			} else {
				// Single file - upload to current prefix
				remotePath := v.currentPrefix + info.Name()
				uploadFiles = append(uploadFiles, uploadFile{
					localPath:  path,
					remotePath: remotePath,
					size:       info.Size(),
				})
				totalSize += info.Size()
			}
		}

		// Create channel here (inside goroutine) to avoid leak on early error above
		v.uploadChan = make(chan progress.ProgressUpdate, 10)

		var bytesUploaded int64

		for i, file := range uploadFiles {
			// Send progress update before starting this file
			v.uploadChan <- progress.ProgressUpdate{
				BytesTransferred: bytesUploaded,
				TotalBytes:       totalSize,
				CurrentFile:      filepath.Base(file.localPath),
				CurrentFileNum:   i + 1,
				TotalFiles:       len(uploadFiles),
			}

			// Upload with progress callback
			err := v.storageClient.UploadObject(
				context.Background(),
				v.bucketName,
				file.remotePath,
				file.localPath,
				func(transferred, total int64) {
					v.uploadChan <- progress.ProgressUpdate{
						BytesTransferred: bytesUploaded + transferred,
						TotalBytes:       totalSize,
						CurrentFile:      filepath.Base(file.localPath),
						CurrentFileNum:   i + 1,
						TotalFiles:       len(uploadFiles),
					}
				},
			)
			if err != nil {
				return uploadCompleteMsg{err: fmt.Errorf("failed to upload %s: %w", filepath.Base(file.localPath), err)}
			}

			bytesUploaded += file.size
		}

		return uploadCompleteMsg{err: nil}
	}
}

// waitForUploadProgress waits for the next progress update from the upload goroutine
func (v *ObjectsView) waitForUploadProgress() tea.Cmd {
	return func() tea.Msg {
		if v.uploadChan == nil {
			return nil
		}
		update, ok := <-v.uploadChan
		if !ok {
			return nil
		}
		return uploadProgressMsg{update: update}
	}
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
