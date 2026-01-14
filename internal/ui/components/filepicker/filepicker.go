package filepicker

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/symbols"
)

// Colors matching GCP theme
var (
	borderColor    = lipgloss.Color("#4285F4")
	selectedBg     = lipgloss.Color("#4285F4")
	mutedTextColor = lipgloss.Color("#9AA0A6")
)

// FileEntry represents a file or directory in the file picker
type FileEntry struct {
	Name     string
	Path     string
	Size     int64
	IsDir    bool
	Selected bool
}

// fileItem implements list.Item for file entries
type fileItem struct {
	entry FileEntry
}

// Style for muted file size text
var sizeStyle = lipgloss.NewStyle().Foreground(mutedTextColor)

func (i fileItem) Title() string {
	// Checkbox: [ ] or [x]
	check := "[ ] "
	if i.entry.Selected {
		check = "[x] "
	}
	// ".." doesn't get a checkbox
	if i.entry.Name == ".." {
		check = "    "
	}

	// Icon and name
	icon := symbols.File()
	if i.entry.IsDir {
		icon = symbols.Folder()
	}

	// Format: "[x] 📄 filename.txt (1.5 KB)" or "[x] 📁 foldername"
	if i.entry.IsDir {
		return fmt.Sprintf("%s%s %s", check, icon, i.entry.Name)
	}
	// File size in gray
	sizeText := sizeStyle.Render(fmt.Sprintf("(%s)", gcp.FormatSize(i.entry.Size)))
	return fmt.Sprintf("%s%s %s %s", check, icon, i.entry.Name, sizeText)
}

func (i fileItem) Description() string {
	// Empty description for single-line display
	return ""
}

func (i fileItem) FilterValue() string {
	return i.entry.Name
}

// FilePicker is a component for browsing and selecting local files
type FilePicker struct {
	list        list.Model
	currentPath string
	entries     []FileEntry
	selected    map[string]bool // Tracks selected paths
	width       int
	height      int
	keys        filePickerKeyMap
	err         error
	title       string
	showHidden  bool
	multiSelect bool
}

// filePickerKeyMap defines key bindings for the file picker
type filePickerKeyMap struct {
	Enter       key.Binding
	Back        key.Binding
	Left        key.Binding // Left arrow - go up when on first item
	Toggle      key.Binding
	SelectAll   key.Binding
	DeselectAll key.Binding
	Confirm     key.Binding
	Cancel      key.Binding
}

func defaultFilePickerKeyMap() filePickerKeyMap {
	return filePickerKeyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open/confirm"),
		),
		Back: key.NewBinding(
			key.WithKeys("backspace", "h"),
			key.WithHelp("backspace", "go up"),
		),
		Left: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "go up"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle select"),
		),
		SelectAll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "select all"),
		),
		DeselectAll: key.NewBinding(
			key.WithKeys("A"),
			key.WithHelp("A", "deselect all"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc", "q"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// New creates a new file picker starting at the given path
func New(startPath string, multiSelect bool) *FilePicker {
	if startPath == "" {
		if wd, err := os.Getwd(); err == nil {
			startPath = wd
		} else {
			// Fallback if working directory cannot be determined
			startPath = "."
		}
	}

	delegate := list.NewDefaultDelegate()
	// Single-line display: no description, compact spacing
	delegate.ShowDescription = false
	delegate.SetHeight(1)
	delegate.SetSpacing(0)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(selectedBg).
		Bold(true)

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Select Files to Upload"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(borderColor).
		Padding(0, 1)

	// Add help keys
	l.AdditionalShortHelpKeys = func() []key.Binding {
		km := defaultFilePickerKeyMap()
		return []key.Binding{km.Toggle, km.SelectAll, km.Back, km.Cancel}
	}

	fp := &FilePicker{
		list:        l,
		currentPath: startPath,
		selected:    make(map[string]bool),
		keys:        defaultFilePickerKeyMap(),
		title:       "Select Files to Upload",
		multiSelect: multiSelect,
	}

	return fp
}

// Init initializes the file picker and loads the initial directory
func (fp *FilePicker) Init() tea.Cmd {
	return fp.loadDirectory("")
}

// loadDirectory loads the contents of the current directory
// selectTarget is the name of an entry to select after loading (used when navigating up)
func (fp *FilePicker) loadDirectory(selectTarget string) tea.Cmd {
	return func() tea.Msg {
		entries, err := fp.readDirectory(fp.currentPath)
		if err != nil {
			return filePickerErrorMsg{err: err}
		}
		return filePickerLoadedMsg{entries: entries, selectTarget: selectTarget}
	}
}

// readDirectory reads and returns entries from a directory
func (fp *FilePicker) readDirectory(path string) ([]FileEntry, error) {
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var entries []FileEntry

	// Add parent directory entry if not at filesystem root
	// Use cross-platform check: at root, filepath.Dir(path) equals path
	parentPath := filepath.Dir(path)
	if parentPath != path {
		entries = append(entries, FileEntry{
			Name:  "..",
			Path:  parentPath,
			IsDir: true,
		})
	}

	for _, de := range dirEntries {
		// Skip hidden files unless showHidden is true
		if !fp.showHidden && strings.HasPrefix(de.Name(), ".") {
			continue
		}

		info, err := de.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(path, de.Name())
		entry := FileEntry{
			Name:     de.Name(),
			Path:     fullPath,
			IsDir:    de.IsDir(),
			Size:     info.Size(),
			Selected: fp.selected[fullPath],
		}
		entries = append(entries, entry)
	}

	// Sort: directories first, then files, alphabetically
	sort.Slice(entries, func(i, j int) bool {
		// Keep ".." at the top
		if entries[i].Name == ".." {
			return true
		}
		if entries[j].Name == ".." {
			return false
		}
		// Directories before files
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		// Alphabetical within same type
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	return entries, nil
}

// Message types
type filePickerLoadedMsg struct {
	entries      []FileEntry
	selectTarget string // Name of entry to select after loading (for "go up" navigation)
}

type filePickerErrorMsg struct {
	err error
}

// FilePickerConfirmMsg is sent when the user confirms their selection
type FilePickerConfirmMsg struct {
	SelectedPaths []string
}

// FilePickerCancelMsg is sent when the user cancels
type FilePickerCancelMsg struct{}

// Update handles messages for the file picker
func (fp *FilePicker) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case filePickerLoadedMsg:
		fp.entries = msg.entries
		fp.updateListItems()

		// Select target entry if specified (used when navigating up to highlight previous folder)
		if msg.selectTarget != "" {
			for i, entry := range fp.entries {
				if entry.Name == msg.selectTarget {
					fp.list.Select(i)
					break
				}
			}
		}
		return nil

	case filePickerErrorMsg:
		fp.err = msg.err
		return nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, fp.keys.Cancel):
			// Cancel at any level
			return func() tea.Msg {
				return FilePickerCancelMsg{}
			}

		case key.Matches(msg, fp.keys.Back):
			// Go up to parent directory, remembering current folder name
			// Cross-platform root check: at root, filepath.Dir equals currentPath
			if parentDir := filepath.Dir(fp.currentPath); parentDir != fp.currentPath {
				currentFolderName := filepath.Base(fp.currentPath)
				fp.currentPath = parentDir
				return fp.loadDirectory(currentFolderName)
			}

		case key.Matches(msg, fp.keys.Left):
			// Left arrow goes up only when on first page of the list
			// Cross-platform root check: at root, filepath.Dir equals currentPath
			if parentDir := filepath.Dir(fp.currentPath); fp.list.Paginator.Page == 0 && parentDir != fp.currentPath {
				currentFolderName := filepath.Base(fp.currentPath)
				fp.currentPath = parentDir
				return fp.loadDirectory(currentFolderName)
			}

		case key.Matches(msg, fp.keys.Toggle):
			// Toggle selection on current item
			if fp.multiSelect {
				if item, ok := fp.list.SelectedItem().(fileItem); ok {
					if item.entry.Name != ".." {
						fp.toggleSelection(item.entry.Path)
						fp.updateListItems()
					}
				}
			}

		case key.Matches(msg, fp.keys.SelectAll):
			// Select all files in current directory
			if fp.multiSelect {
				for _, entry := range fp.entries {
					if entry.Name != ".." {
						fp.selected[entry.Path] = true
					}
				}
				fp.updateListItems()
			}

		case key.Matches(msg, fp.keys.DeselectAll):
			// Deselect all
			if fp.multiSelect {
				fp.selected = make(map[string]bool)
				fp.updateListItems()
			}

		case key.Matches(msg, fp.keys.Enter):
			if item, ok := fp.list.SelectedItem().(fileItem); ok {
				if item.entry.IsDir {
					// Handle ".." specially - navigate up with target selection
					if item.entry.Name == ".." {
						currentFolderName := filepath.Base(fp.currentPath)
						fp.currentPath = item.entry.Path
						return fp.loadDirectory(currentFolderName)
					}
					// Navigate into directory
					fp.currentPath = item.entry.Path
					return fp.loadDirectory("")
				}
				// For files: if multi-select and nothing selected, select this file
				// If selections exist or single-select mode, confirm
				if fp.multiSelect && len(fp.selected) == 0 {
					fp.selected[item.entry.Path] = true
				}
				return fp.confirm()
			}
		}
	}

	var cmd tea.Cmd
	fp.list, cmd = fp.list.Update(msg)
	return cmd
}

// toggleSelection toggles the selection state of a path
func (fp *FilePicker) toggleSelection(path string) {
	if fp.selected[path] {
		delete(fp.selected, path)
	} else {
		fp.selected[path] = true
	}
}

// updateListItems updates the list with current entries and selection state
func (fp *FilePicker) updateListItems() {
	items := make([]list.Item, len(fp.entries))
	for i, entry := range fp.entries {
		entry.Selected = fp.selected[entry.Path]
		items[i] = fileItem{entry: entry}
	}
	fp.list.SetItems(items)
	fp.list.Title = fp.buildTitle()
}

// buildTitle creates the title with path and selection count
func (fp *FilePicker) buildTitle() string {
	title := fp.title
	if len(fp.selected) > 0 {
		title = fmt.Sprintf("%s (%d selected)", fp.title, len(fp.selected))
	}
	return title
}

// confirm sends the confirmation message with selected paths
func (fp *FilePicker) confirm() tea.Cmd {
	return func() tea.Msg {
		var paths []string
		for path := range fp.selected {
			paths = append(paths, path)
		}
		// Sort for consistent ordering
		sort.Strings(paths)
		return FilePickerConfirmMsg{SelectedPaths: paths}
	}
}

// View renders the file picker
func (fp *FilePicker) View() string {
	if fp.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
		return errStyle.Render(fmt.Sprintf("\n  Error: %v\n\n  Press 'esc' to cancel", fp.err))
	}

	// Status line showing current path
	statusStyle := lipgloss.NewStyle().Foreground(mutedTextColor)
	pathDisplay := fp.currentPath
	maxPathLen := fp.width - 10
	if len(pathDisplay) > maxPathLen && maxPathLen > 3 {
		pathDisplay = "..." + pathDisplay[len(pathDisplay)-maxPathLen+3:]
	}
	status := statusStyle.Render(fmt.Sprintf("  %s", pathDisplay))

	// Help text
	help := statusStyle.Render("\n  space: toggle • a/A: select/deselect all • enter: open/confirm • esc: cancel")

	content := fp.list.View() + "\n" + status + help

	// Wrap in a border for modal appearance
	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	return borderStyle.Render(content)
}

// SetSize updates the file picker dimensions
func (fp *FilePicker) SetSize(width, height int) {
	fp.width = width
	fp.height = height
	// Account for border and status lines
	fp.list.SetSize(width-4, height-8)
}

// GetCurrentPath returns the current directory path
func (fp *FilePicker) GetCurrentPath() string {
	return fp.currentPath
}

// GetSelectedPaths returns currently selected paths
func (fp *FilePicker) GetSelectedPaths() []string {
	var paths []string
	for path := range fp.selected {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// SetTitle sets the title of the file picker
func (fp *FilePicker) SetTitle(title string) {
	fp.title = title
	fp.list.Title = fp.buildTitle()
}
