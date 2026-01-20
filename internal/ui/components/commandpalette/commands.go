package commandpalette

import (
	tea "github.com/charmbracelet/bubbletea"
)

// CommandType distinguishes different command behaviors
type CommandType int

const (
	CommandTypeNavigation CommandType = iota // Navigate to a view
	CommandTypeAction                        // Execute an action (refresh, toggle)
	CommandTypeRecent                        // Recently accessed item
)

// ViewType mirrors ui.ViewType to avoid import cycle
type ViewType int

const (
	ViewInstances ViewType = iota
	ViewDisks
	ViewSnapshots
	ViewImages
	ViewProjectMetadata
	ViewBuckets
	ViewNetworks
	ViewFirewall
)

// Icons for commands
const (
	IconVM       = "■"
	IconDisk     = "●"
	IconImage    = "◉"
	IconMetadata = "◐"
	IconBucket   = "▪"
	IconVPC      = "◆"
	IconFirewall = "▲"
	IconRefresh  = "↻"
	IconSidebar  = "☰"
	IconHelp     = "?"
	IconRecent   = "⏱"
	IconProject  = "◎"
	IconQuit     = "✕"
)

// Command represents an executable palette item
type Command struct {
	ID       string // Unique identifier: "nav:vm-instances", "action:refresh"
	Label    string // Display: "Compute Engine: VM instances"
	Icon     string // Unicode icon
	Type     CommandType
	ViewType ViewType       // For navigation commands
	Action   func() tea.Cmd // For action commands
	Enabled  bool           // Whether command can be executed
}

// CommandSelectedMsg emitted when user selects a command
type CommandSelectedMsg struct {
	Command Command
}

// CommandCancelMsg emitted when user cancels (Esc)
type CommandCancelMsg struct{}

// NavigationCommands returns the navigation commands from sidebar menu
func NavigationCommands() []Command {
	return []Command{
		{
			ID:       "nav:vm-instances",
			Label:    "Compute Engine: VM instances",
			Icon:     IconVM,
			Type:     CommandTypeNavigation,
			ViewType: ViewInstances,
			Enabled:  true,
		},
		{
			ID:       "nav:disks",
			Label:    "Compute Engine: Disks",
			Icon:     IconDisk,
			Type:     CommandTypeNavigation,
			ViewType: ViewDisks,
			Enabled:  true,
		},
		{
			ID:       "nav:snapshots",
			Label:    "Compute Engine: Snapshots",
			Icon:     IconDisk,
			Type:     CommandTypeNavigation,
			ViewType: ViewSnapshots,
			Enabled:  true,
		},
		{
			ID:       "nav:images",
			Label:    "Compute Engine: Images",
			Icon:     IconImage,
			Type:     CommandTypeNavigation,
			ViewType: ViewImages,
			Enabled:  true,
		},
		{
			ID:       "nav:metadata",
			Label:    "Compute Engine: Metadata",
			Icon:     IconMetadata,
			Type:     CommandTypeNavigation,
			ViewType: ViewProjectMetadata,
			Enabled:  true,
		},
		{
			ID:       "nav:buckets",
			Label:    "Cloud Storage: Buckets",
			Icon:     IconBucket,
			Type:     CommandTypeNavigation,
			ViewType: ViewBuckets,
			Enabled:  true,
		},
		{
			ID:       "nav:networks",
			Label:    "VPC Network: VPC networks",
			Icon:     IconVPC,
			Type:     CommandTypeNavigation,
			ViewType: ViewNetworks,
			Enabled:  true,
		},
		{
			ID:       "nav:firewall",
			Label:    "VPC Network: Firewall",
			Icon:     IconFirewall,
			Type:     CommandTypeNavigation,
			ViewType: ViewFirewall,
			Enabled:  true,
		},
	}
}

// ActionCommands returns the action commands
func ActionCommands() []Command {
	return []Command{
		{
			ID:      "action:switch-project",
			Label:   "Switch Project",
			Icon:    IconProject,
			Type:    CommandTypeAction,
			Enabled: true,
		},
		{
			ID:      "action:refresh",
			Label:   "Refresh",
			Icon:    IconRefresh,
			Type:    CommandTypeAction,
			Enabled: true,
		},
		{
			ID:      "action:toggle-sidebar",
			Label:   "Toggle sidebar",
			Icon:    IconSidebar,
			Type:    CommandTypeAction,
			Enabled: true,
		},
		{
			ID:      "action:help",
			Label:   "Help",
			Icon:    IconHelp,
			Type:    CommandTypeAction,
			Enabled: true,
		},
		{
			ID:      "action:quit",
			Label:   "Quit",
			Icon:    IconQuit,
			Type:    CommandTypeAction,
			Enabled: true,
		},
	}
}

// DefaultCommands returns all default commands (navigation + actions)
func DefaultCommands() []Command {
	commands := make([]Command, 0, 8)
	commands = append(commands, NavigationCommands()...)
	commands = append(commands, ActionCommands()...)
	return commands
}
