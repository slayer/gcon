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
	ViewSubnets
	ViewRoutes
	ViewSQLInstances
	ViewServiceAccounts
	ViewIAMPolicy
	ViewCustomRoles
	ViewCloudRunServices
	ViewLogs
)

// Icons for commands
const (
	IconVM             = "■"
	IconDisk           = "●"
	IconImage          = "◉"
	IconMetadata       = "◐"
	IconBucket         = "▪"
	IconVPC            = "◆"
	IconFirewall       = "▲"
	IconSubnet         = "▫"
	IconSQLInstance    = "⬢"
	IconServiceAccount = "▲"
	IconIAMPolicy      = "▽"
	IconCustomRole     = "▼"
	IconRoute          = "→"
	IconCloudRun       = "▶"
	IconLogs           = "◆"
	IconRefresh        = "↻"
	IconSidebar        = "☰"
	IconHelp           = "?"
	IconRecent         = "⏱"
	IconProject        = "◎"
	IconQuit           = "✕"
	IconDemo           = "▦"
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
			ID:       "nav:sql-instances",
			Label:    "Databases: SQL instances",
			Icon:     IconSQLInstance,
			Type:     CommandTypeNavigation,
			ViewType: ViewSQLInstances,
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
		{
			ID:       "nav:subnets",
			Label:    "VPC Network: Subnets",
			Icon:     IconSubnet,
			Type:     CommandTypeNavigation,
			ViewType: ViewSubnets,
			Enabled:  true,
		},
		{
			ID:       "nav:routes",
			Label:    "VPC Network: Routes",
			Icon:     IconRoute,
			Type:     CommandTypeNavigation,
			ViewType: ViewRoutes,
			Enabled:  true,
		},
		{
			ID:       "nav:service-accounts",
			Label:    "IAM & Admin: Service accounts",
			Icon:     IconServiceAccount,
			Type:     CommandTypeNavigation,
			ViewType: ViewServiceAccounts,
			Enabled:  true,
		},
		{
			ID:       "nav:iam-policy",
			Label:    "IAM & Admin: IAM policy",
			Icon:     IconIAMPolicy,
			Type:     CommandTypeNavigation,
			ViewType: ViewIAMPolicy,
			Enabled:  true,
		},
		{
			ID:       "nav:custom-roles",
			Label:    "IAM & Admin: Custom roles",
			Icon:     IconCustomRole,
			Type:     CommandTypeNavigation,
			ViewType: ViewCustomRoles,
			Enabled:  true,
		},
		{
			ID:       "nav:cloudrun-services",
			Label:    "Cloud Run: Services",
			Icon:     IconCloudRun,
			Type:     CommandTypeNavigation,
			ViewType: ViewCloudRunServices,
			Enabled:  true,
		},
		{
			ID:       "nav:logs-explorer",
			Label:    "Logging: Logs Explorer",
			Icon:     IconLogs,
			Type:     CommandTypeNavigation,
			ViewType: ViewLogs,
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
		{
			ID:      "action:create-instance",
			Label:   "Compute Engine: Create VM Instance",
			Icon:    IconVM,
			Type:    CommandTypeAction,
			Enabled: true,
		},
		{
			ID:      "action:form-demo",
			Label:   "Form Demo (dev)",
			Icon:    IconDemo,
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
