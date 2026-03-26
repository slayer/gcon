package ui

import (
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/config"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/commandpalette"
	"github.com/slayer/gcon/internal/ui/components/projectselector"
	"github.com/slayer/gcon/internal/ui/components/sidebar"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/layout"
	"github.com/slayer/gcon/internal/ui/views"
	"golang.org/x/term"
)

// ViewType represents different screens in the application
type ViewType int

const (
	ViewNone ViewType = -1 // Sentinel value for unset/invalid view

	ViewProjects ViewType = iota
	ViewInstances
	ViewInstanceDetails
	ViewMetadata
	ViewProjectMetadata
	ViewDisks
	ViewDiskDetails
	ViewSnapshots
	ViewSnapshotDetails
	ViewImages
	ViewImageDetails
	ViewBuckets
	ViewObjects        // Browsing objects within a bucket
	ViewObjectDetails  // Viewing object details
	ViewInstanceEditor // Editing instance properties (labels, etc.)
	ViewBucketCreate   // Creating a new GCS bucket
	ViewSnapshotCreate // Creating a snapshot from a disk
	ViewImageCreate    // Creating an image from a disk
	ViewDiskCreate     // Creating a disk from a snapshot
	ViewNetworks
	ViewNetworkDetails
	ViewFirewall
	ViewFirewallDetails
	ViewSQLInstances
	ViewSQLInstanceDetails
	ViewServiceAccounts
	ViewServiceAccountDetails
	ViewServiceAccountCreate
	ViewIAMPolicy
	ViewCustomRoles
	ViewCustomRoleDetails
	ViewCloudRunServices
	ViewCloudRunServiceDetails
	ViewCloudRunServiceEdit
	ViewInstanceCreate     // Creating a new VM instance
	ViewInstanceConfigEdit // Editing an existing VM instance configuration
	ViewLogs
	ViewFormDemo // Demo view for testing form components
)

// FocusedPanel indicates which panel has keyboard focus
type FocusedPanel int

const (
	FocusContent FocusedPanel = iota
	FocusSidebar
)

// App is the main application model
type App struct {
	gcpClient *gcp.Client
	ctx       *context.ProgramContext // Shared context for all views
	styles    Styles
	keys      KeyMap
	help      help.Model
	width     int
	height    int
	layout    *layout.Layout // Tile-based layout manager

	// Current view state
	currentView                ViewType
	viewStack                  []ViewType // For back navigation
	projectView                *views.ProjectsView
	instancesView              *views.InstancesView
	instanceDetailsView        *views.InstanceDetailsView
	metadataView               *views.InstanceMetadataView
	projectMetadataView        *views.ProjectMetadataView
	disksView                  *views.DisksView
	diskDetailsView            *views.DiskDetailsView
	snapshotsView              *views.SnapshotsView
	snapshotDetailsView        *views.SnapshotDetailsView
	imagesView                 *views.ImagesView
	imageDetailsView           *views.ImageDetailsView
	bucketsView                *views.BucketsView
	objectsView                *views.ObjectsView
	objectDetailsView          *views.ObjectDetailsView
	instanceEditorView         *views.InstanceEditorView
	bucketCreateView           *views.BucketCreateView
	snapshotCreateView         *views.SnapshotCreateView
	imageCreateView            *views.ImageCreateView
	diskCreateView             *views.DiskCreateView
	networksView               *views.NetworksView
	networkDetailsView         *views.NetworkDetailsView
	firewallsView              *views.FirewallsView
	firewallDetailsView        *views.FirewallDetailsView
	sqlInstancesView           *views.SQLInstancesView
	sqlInstanceDetailsView     *views.SQLInstanceDetailsView
	serviceAccountsView        *views.ServiceAccountsView
	serviceAccountDetailsView  *views.ServiceAccountDetailsView
	serviceAccountCreateView   *views.ServiceAccountCreateView
	iamPolicyView              *views.IAMPolicyView
	customRolesView            *views.CustomRolesView
	customRoleDetailsView      *views.CustomRoleDetailsView
	cloudRunServicesView       *views.CloudRunServicesView
	cloudRunServiceDetailsView *views.CloudRunServiceDetailsView
	cloudRunServiceEditView    *views.CloudRunEditView
	instanceCreateView         *views.InstanceCreateView
	instanceConfigEditView     *views.InstanceConfigEditView
	logsView                   *views.LogsView
	formDemoView               *views.FormDemoView

	// Selected context
	selectedProject         *gcp.Project
	selectedInstance        *gcp.Instance
	selectedDisk            *gcp.Disk
	selectedSnapshot        *gcp.Snapshot
	selectedImage           *gcp.Image
	selectedBucket          *gcp.Bucket
	selectedObject          *gcp.StorageObject
	selectedNetwork         *gcp.Network
	selectedFirewall        *gcp.FirewallRule
	selectedSQLInstance     *gcp.SQLInstance
	selectedServiceAccount  *gcp.ServiceAccount
	selectedCustomRole      *gcp.CustomRole
	selectedCloudRunService *gcp.CloudRunService

	// UI state
	showHelp              bool
	err                   error
	loadingInitialProject bool // True while loading project from config/flag

	// Initial project from config/flag (skip project selector if set)
	initialProjectID string

	// Sidebar navigation (active after project selection)
	sidebar      *sidebar.Sidebar
	focusedPanel FocusedPanel

	// Command palette
	commandPalette     *commandpalette.CommandPalette
	showCommandPalette bool
	recentTracker      *commandpalette.RecentTracker

	// Project selector modal
	projectSelector               *projectselector.Model
	showProjectSelector           bool
	projectSelectorShownOnStartup bool // Track if selector shown because no default project

	// Header
	header *components.Header

	// Footer
	footer *components.Footer

	// Authenticated identity (email of user or service account)
	authenticatedIdentity string
	identityType          config.IdentityType

	// GCloud configuration profile name
	configProfile string
}

// AppOptions configures the application
type AppOptions struct {
	// InitialProjectID skips project selector and goes directly to this project
	InitialProjectID string
}

// NewApp creates a new application instance
func NewApp(client *gcp.Client, opts AppOptions) *App {
	ctx := context.New()

	// Get authenticated identity and type if client is available
	var authenticatedIdentity string
	var identityType config.IdentityType
	if client != nil {
		authenticatedIdentity = client.GetAuthenticatedIdentity()
		identityType = client.GetIdentityType()
	}

	// Get gcloud config profile
	configProfile := config.ResolveActiveConfigName()

	a := &App{
		gcpClient:             client,
		ctx:                   ctx,
		styles:                DefaultStyles(),
		keys:                  DefaultKeyMap(),
		help:                  help.New(),
		layout:                layout.New(),
		currentView:           ViewProjects,
		viewStack:             []ViewType{},
		projectView:           views.NewProjectsView(client),
		initialProjectID:      opts.InitialProjectID,
		sidebar:               sidebar.New(),
		focusedPanel:          FocusContent,
		commandPalette:        commandpalette.New(),
		recentTracker:         commandpalette.NewRecentTracker(),
		projectSelector:       projectselector.New(client, opts.InitialProjectID),
		header:                components.NewHeader(),
		footer:                components.NewFooter(),
		authenticatedIdentity: authenticatedIdentity,
		identityType:          identityType,
		configProfile:         configProfile,
	}

	// Set up the StartTask callback for async operation tracking
	ctx.StartTask = a.startTask

	return a
}

// ShowProjectSelectorOnStartup configures the app to show project selector on startup
func (a *App) ShowProjectSelectorOnStartup() {
	a.showProjectSelector = true
	a.projectSelectorShownOnStartup = true
	// Sidebar will be hidden automatically since selectedProject is nil
}

// Init implements tea.Model
func (a *App) Init() tea.Cmd {
	// Workaround: Set default size in case WindowSizeMsg never arrives
	// This can happen in some terminal emulators or environments (e.g., tmux/screen)
	if a.width == 0 {
		// Try to get actual terminal size first
		width, height, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil {
			// Fall back to reasonable defaults if we can't detect size
			width = 160
			height = 50
		}
		a.width = width
		a.height = height
		a.layout.SetSize(a.width, a.height)
		a.header.SetSize(a.width)
		a.footer.SetWidth(a.width)
		a.help.Width = a.width
		a.updateViewSizes()
	}

	// If project selector should be shown on startup
	if a.showProjectSelector {
		return a.projectSelector.Init()
	}
	// If initial project is set, load it directly instead of showing selector
	if a.initialProjectID != "" {
		a.loadingInitialProject = true
		return a.loadInitialProject()
	}
	return a.projectView.Init()
}

// loadInitialProject switches directly to instances view using the configured project ID.
// Trade-off: We skip project validation to avoid requiring cloudresourcemanager permissions.
// This means invalid project IDs won't be caught until the user tries to load instances,
// but it allows users without project listing permissions to still use the app.
func (a *App) loadInitialProject() tea.Cmd {
	return func() tea.Msg {
		// Use project ID directly without validation
		return InitialProjectLoadedMsg{
			Project: gcp.Project{
				ID:   a.initialProjectID,
				Name: a.initialProjectID,
			},
		}
	}
}

// sidebarActive returns true if sidebar should be shown
func (a *App) sidebarActive() bool {
	return a.selectedProject != nil && a.currentView != ViewProjects
}

// isViewMenuOpen checks if the current view has an action menu open.
func (a *App) isViewMenuOpen() bool {
	if model := a.getCurrentViewModel(); model != nil {
		if menuOpener, ok := model.(views.MenuOpener); ok {
			return menuOpener.IsMenuOpen()
		}
	}
	return false
}

// updateCurrentView sends a message to the current view and returns its command
func (a *App) updateCurrentView(msg tea.Msg) tea.Cmd {
	if model := a.getCurrentViewModel(); model != nil {
		return model.Update(msg)
	}
	return nil
}

// TextInputFocusable is implemented by views that can have text input fields focused
type TextInputFocusable interface {
	HasTextInputFocused() bool
}

// hasTextInputFocused returns true if the current view has a text input focused.
// When true, character keys should be passed to the view instead of handled globally.
func (a *App) hasTextInputFocused() bool {
	view := a.getCurrentViewModel()
	if focusable, ok := view.(TextInputFocusable); ok {
		return focusable.HasTextInputFocused()
	}
	return false
}

// getCurrentViewModel returns the model for the currently active view.
func (a *App) getCurrentViewModel() views.View {
	switch a.currentView {
	case ViewProjects:
		return a.projectView
	case ViewInstances:
		return a.instancesView
	case ViewInstanceDetails:
		return a.instanceDetailsView
	case ViewMetadata:
		return a.metadataView
	case ViewProjectMetadata:
		return a.projectMetadataView
	case ViewDisks:
		return a.disksView
	case ViewDiskDetails:
		return a.diskDetailsView
	case ViewSnapshots:
		return a.snapshotsView
	case ViewSnapshotDetails:
		return a.snapshotDetailsView
	case ViewImages:
		return a.imagesView
	case ViewImageDetails:
		return a.imageDetailsView
	case ViewBuckets:
		return a.bucketsView
	case ViewObjects:
		return a.objectsView
	case ViewObjectDetails:
		return a.objectDetailsView
	case ViewInstanceEditor:
		return a.instanceEditorView
	case ViewBucketCreate:
		return a.bucketCreateView
	case ViewSnapshotCreate:
		return a.snapshotCreateView
	case ViewImageCreate:
		return a.imageCreateView
	case ViewDiskCreate:
		return a.diskCreateView
	case ViewNetworks:
		return a.networksView
	case ViewNetworkDetails:
		return a.networkDetailsView
	case ViewFirewall:
		return a.firewallsView
	case ViewFirewallDetails:
		return a.firewallDetailsView
	case ViewSQLInstances:
		return a.sqlInstancesView
	case ViewSQLInstanceDetails:
		return a.sqlInstanceDetailsView
	case ViewServiceAccounts:
		return a.serviceAccountsView
	case ViewServiceAccountDetails:
		return a.serviceAccountDetailsView
	case ViewServiceAccountCreate:
		return a.serviceAccountCreateView
	case ViewIAMPolicy:
		return a.iamPolicyView
	case ViewCustomRoles:
		return a.customRolesView
	case ViewCustomRoleDetails:
		return a.customRoleDetailsView
	case ViewCloudRunServices:
		return a.cloudRunServicesView
	case ViewCloudRunServiceDetails:
		return a.cloudRunServiceDetailsView
	case ViewCloudRunServiceEdit:
		return a.cloudRunServiceEditView
	case ViewInstanceCreate:
		return a.instanceCreateView
	case ViewInstanceConfigEdit:
		return a.instanceConfigEditView
	case ViewLogs:
		return a.logsView
	case ViewFormDemo:
		return a.formDemoView
	}
	return nil
}

// Update implements tea.Model
//
//nolint:gocognit // Bubble Tea Update pattern - complexity 60
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle project selector messages first (highest priority when active)
	if a.showProjectSelector {
		switch msg := msg.(type) {
		case projectselector.ProjectSelectedMsg:
			// Project selected, no longer in startup mode
			a.projectSelectorShownOnStartup = false
			//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
			return a, a.handleProjectSwitch(&msg.Project)
		case projectselector.ProjectSelectorCanceledMsg:
			// If project selector was shown on startup (no default project),
			// exit the app when user cancels since no project is available
			if a.projectSelectorShownOnStartup {
				a.cleanup()
				return a, tea.Quit
			}
			// Otherwise, just hide the selector (user was switching projects)
			a.showProjectSelector = false
			return a, nil
		default:
			// Pass all other messages to project selector (including spinner ticks, key msgs, etc)
			cmd := a.projectSelector.Update(msg)
			return a, cmd
		}
	}

	// Handle command palette messages (second priority when active)
	if a.showCommandPalette {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			cmd := a.commandPalette.Update(msg)
			return a, cmd
		case commandpalette.CommandSelectedMsg:
			return a.handleCommandSelected(msg.Command)
		case commandpalette.CommandCancelMsg:
			a.showCommandPalette = false
			a.commandPalette.Reset()
			return a, nil
		}
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		// Handle mouse events
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleMouseEvent(msg)

	case tea.KeyMsg:
		// Handle back navigation first (before view-specific handlers)
		// But skip if a view has an action menu open - let the view handle Esc
		if key.Matches(msg, a.keys.Back) {
			// Check if any view has action menu open
			if a.isViewMenuOpen() {
				// Let the view handle Esc to close its menu
				//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
				return a, a.updateCurrentView(msg)
			}

			// If sidebar is focused, handle Esc within sidebar context
			if a.focusedPanel == FocusSidebar {
				if len(a.sidebar.GetPath()) > 0 {
					// Drilled down — go back one level in sidebar
					a.sidebar.Update(msg)
					return a, nil
				}
				// At root level — unfocus sidebar and return to content
				a.focusedPanel = FocusContent
				a.sidebar.SetFocused(false)
				if a.sidebar.Mode() == sidebar.SidebarModeAutoHide {
					a.sidebar.Collapse()
					a.updateViewSizes()
				}
				return a, nil
			}

			// If view stack is not empty, pop and go back
			if len(a.viewStack) > 0 {
				leavingView := a.currentView

				// Pop the last view from the stack
				lastViewIndex := len(a.viewStack) - 1
				a.currentView = a.viewStack[lastViewIndex]
				a.viewStack = a.viewStack[:lastViewIndex]

				// Clean up the view model we are navigating away from
				switch leavingView {
				case ViewInstanceDetails:
					a.instanceDetailsView = nil
					a.selectedInstance = nil
				case ViewDiskDetails:
					a.diskDetailsView = nil
					a.selectedDisk = nil
				case ViewSnapshotDetails:
					a.snapshotDetailsView = nil
					a.selectedSnapshot = nil
				case ViewImageDetails:
					a.imageDetailsView = nil
					a.selectedImage = nil
				case ViewObjects:
					a.objectsView = nil
					a.selectedBucket = nil
				case ViewObjectDetails:
					a.objectDetailsView = nil
					a.selectedObject = nil
				case ViewNetworkDetails:
					a.networkDetailsView = nil
					a.selectedNetwork = nil
				case ViewFirewallDetails:
					a.firewallDetailsView = nil
					a.selectedFirewall = nil
				case ViewSQLInstanceDetails:
					a.sqlInstanceDetailsView = nil
					a.selectedSQLInstance = nil
				case ViewServiceAccountDetails:
					a.serviceAccountDetailsView = nil
					a.selectedServiceAccount = nil
				case ViewServiceAccountCreate:
					a.serviceAccountCreateView = nil
				case ViewInstanceCreate:
					a.instanceCreateView = nil
				case ViewInstanceConfigEdit:
					a.instanceConfigEditView = nil
				case ViewCustomRoleDetails:
					a.customRoleDetailsView = nil
					a.selectedCustomRole = nil
				case ViewCloudRunServiceDetails:
					if a.cloudRunServiceDetailsView != nil {
						a.cloudRunServiceDetailsView.Close()
					}
					a.cloudRunServiceDetailsView = nil
					a.selectedCloudRunService = nil
				case ViewLogs:
					if a.logsView != nil {
						a.logsView.Close()
					}
					a.logsView = nil
				}

				a.updateSidebarActiveView()
				// Clean up any orphaned running tasks from the view we just left,
				// since in-flight async results will be silently dropped
				a.clearRunningTasks()
				return a, nil
			}

			// If stack is empty, check for special internal back navigation (e.g. Objects view)
			// or quit from top-level views
			switch a.currentView {
			case ViewObjects:
				if a.objectsView != nil {
					// Handle internal back navigation (e.g., going up a folder)
					handled, cmd := a.objectsView.HandleBack()
					if handled {
						return a, cmd
					}
				}
				// If not handled, fall through to quit
				fallthrough

			case ViewInstances, ViewDisks, ViewSnapshots, ViewImages, ViewBuckets, ViewNetworks, ViewFirewall, ViewSQLInstances, ViewServiceAccounts, ViewIAMPolicy, ViewCustomRoles, ViewCloudRunServices, ViewLogs, ViewProjects:
				// Quit from top-level views or if stack is empty
				a.cleanup()
				return a, tea.Quit
			}
		}

		// Handle Ctrl+C quit regardless of text input focus
		if msg.Type == tea.KeyCtrlC {
			a.cleanup()
			return a, tea.Quit
		}

		// Skip character-based global shortcuts when text input is focused
		// This allows typing "q", "?", etc. in form fields
		if a.hasTextInputFocused() {
			//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
			return a, a.updateCurrentView(msg)
		}

		// Global key handlers (only when text input is NOT focused)
		switch {
		case key.Matches(msg, a.keys.Quit):
			// Clean up resources before quitting
			a.cleanup()
			return a, tea.Quit
		case key.Matches(msg, a.keys.Help):
			a.showHelp = !a.showHelp
			return a, nil
		case key.Matches(msg, a.keys.SelectSidebar):
			// '[' - Focus sidebar (if visible), expand if in auto-hide mode
			if a.sidebarActive() && a.focusedPanel != FocusSidebar {
				if a.sidebar.Mode() == sidebar.SidebarModeAutoHide {
					a.sidebar.Expand()
					a.updateViewSizes()
				}
				a.focusedPanel = FocusSidebar
				a.sidebar.SetFocused(true)
			}
			return a, nil
		case key.Matches(msg, a.keys.SelectContent):
			// ']' - Focus content, collapse sidebar in auto-hide mode
			if a.focusedPanel != FocusContent {
				a.focusedPanel = FocusContent
				a.sidebar.SetFocused(false)
				if a.sidebar.Mode() == sidebar.SidebarModeAutoHide {
					a.sidebar.Collapse()
					a.updateViewSizes()
				}
			}
			return a, nil
		case key.Matches(msg, a.keys.ToggleSidebar):
			// '{' - Toggle sidebar mode (auto-hide ↔ always-open/pinned)
			if a.sidebarActive() {
				if a.sidebar.Mode() == sidebar.SidebarModeAutoHide {
					a.sidebar.SetMode(sidebar.SidebarModeAlwaysOpen)
				} else {
					a.sidebar.SetMode(sidebar.SidebarModeAutoHide)
					// Unfocus sidebar when switching to auto-hide
					if a.focusedPanel == FocusSidebar {
						a.focusedPanel = FocusContent
						a.sidebar.SetFocused(false)
					}
				}
				a.updateViewSizes()
			}
			return a, nil
		case key.Matches(msg, a.keys.CommandPalette):
			// Open command palette, show ":" prefix only when triggered by colon key
			showPrefix := key.Matches(msg, key.NewBinding(key.WithKeys(":")))
			a.openCommandPalette(showPrefix)
			return a, nil
		}

		// Route to sidebar if focused
		if a.sidebarActive() && a.focusedPanel == FocusSidebar {
			cmd := a.sidebar.Update(msg)
			return a, cmd
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.help.Width = msg.Width
		// Update layout with new terminal dimensions
		a.layout.SetSize(msg.Width, msg.Height)
		a.header.SetSize(msg.Width)
		a.footer.SetWidth(msg.Width)
		a.updateViewSizes()
		a.syncContext()
		return a, nil

	case context.TaskClearMsg:
		// Remove completed task from tracking
		delete(a.ctx.Tasks, msg.TaskID)
		return a, nil

	case ErrorMsg:
		a.err = msg.Err
		return a, nil

	case views.ProjectSelectedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleProjectSelected(msg)

	case views.InstanceSelectedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleInstanceSelected(msg)

	case views.DiskSelectedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleDiskSelected(msg)

	case views.InstanceDiskSelectedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleInstanceDiskSelected(msg)

	case views.SnapshotSelectedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleSnapshotSelected(msg)

	case views.SnapshotDiskSelectedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleSnapshotDiskSelected(msg)

	case views.ImageSelectedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleImageSelected(msg)

	case InitialProjectLoadedMsg:
		// Initial project loaded successfully, go directly to instances view
		a.loadingInitialProject = false
		a.selectedProject = &msg.Project
		a.currentView = ViewInstances
		a.instancesView = views.NewInstancesView(msg.Project.ID)
		a.focusedPanel = FocusContent
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a, a.instancesView.Init()

	case InitialProjectErrorMsg:
		// Failed to load initial project, fall back to selector with error displayed
		a.loadingInitialProject = false
		a.err = msg.Err
		a.initialProjectID = ""
		return a, a.projectView.Init()

	case sidebar.NavigateMsg:
		// Handle sidebar navigation
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleSidebarNavigation(msg)

	case views.BucketSelectedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleBucketSelected(msg)

	case views.ObjectSelectedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleObjectSelected(msg)

	case views.ObjectDeletedMsg:
		// Object was deleted, go back to objects list and refresh
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleObjectDeleted(msg)

	case views.InstanceEditRequestMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleInstanceEditRequest(msg)

	case views.InstanceEditCompleteMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleInstanceEditComplete(msg)

	case views.InstanceEditCanceledMsg:
		a.handleInstanceEditCancelled()
		return a, nil

	case views.InstanceCreateRequestMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleInstanceCreateRequest(msg)

	case views.CreateInstanceMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleCreateInstance(msg)

	case views.InstanceCreateResultMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleInstanceCreateResult(msg)

	case views.InstanceCreateCanceledMsg:
		a.handleInstanceCreateCanceled()
		return a, nil

	case views.InstanceConfigEditRequestMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleInstanceConfigEditRequest(msg)

	case views.InstanceConfigEditSubmitMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleInstanceConfigEditSubmit(msg)

	case views.InstanceConfigEditResultMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleInstanceConfigEditResult(msg)

	case views.InstanceConfigEditCanceledMsg:
		a.handleInstanceConfigEditCanceled()
		return a, nil

	case views.BucketCreateRequestMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleBucketCreateRequest(msg)

	case views.BucketCreatedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleBucketCreated(msg)

	case views.BucketCreateCanceledMsg:
		a.handleBucketCreateCanceled()
		return a, nil

	case views.DeleteDiskConfirmedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleDeleteDiskConfirmed(msg)

	case views.SnapshotCreateRequestMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleSnapshotCreateRequest(msg)

	case views.SnapshotCreateCanceledMsg:
		a.handleSnapshotCreateCanceled()
		return a, nil

	case views.CreateSnapshotFromDiskMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleCreateSnapshotFromDisk(msg)

	case views.ImageCreateRequestMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleImageCreateRequest(msg)

	case views.ImageCreateCanceledMsg:
		a.handleImageCreateCanceled()
		return a, nil

	case views.CreateImageFromDiskMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleCreateImageFromDisk(msg)

	case views.DiskActionResultMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleDiskActionResult(msg)

	case views.DeleteSnapshotConfirmedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleDeleteSnapshotConfirmed(msg)

	case views.SnapshotActionResultMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleSnapshotActionResult(msg)

	case views.DiskCreateFromSnapshotRequestMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleDiskCreateFromSnapshotRequest(msg)

	case views.DiskCreateCanceledMsg:
		a.handleDiskCreateCanceled()
		return a, nil

	case views.CreateDiskFromSnapshotMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleCreateDiskFromSnapshot(msg)

	case views.ImageCreateFromSnapshotRequestMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleImageCreateFromSnapshotRequest(msg)

	case views.ImageCreateFromSnapshotCanceledMsg:
		a.handleImageCreateFromSnapshotCanceled()
		return a, nil

	case views.CreateImageFromSnapshotMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleCreateImageFromSnapshot(msg)

	case views.DeleteImageConfirmedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleDeleteImageConfirmed(msg)

	case views.ImageActionResultMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleImageActionResult(msg)

	case views.DiskCreateFromImageRequestMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleDiskCreateFromImageRequest(msg)

	case views.CreateDiskFromImageMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleCreateDiskFromImage(msg)

	case views.NetworkSelectedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleNetworkSelected(msg)

	case views.FirewallSelectedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleFirewallSelected(msg)

	case views.DeleteFirewallConfirmedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleDeleteFirewallConfirmed(msg)

	case views.ToggleFirewallMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleToggleFirewall(msg)

	case views.FirewallActionResultMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleFirewallActionResult(msg)

	case views.SQLInstanceSelectedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleSQLInstanceSelected(msg)

	case views.SQLInstanceActionMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleSQLInstanceAction(msg)

	case views.DeleteSQLInstanceConfirmedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleDeleteSQLInstanceConfirmed(msg)

	case views.SQLInstanceActionResultMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleSQLInstanceActionResult(msg)

	case views.CreateSQLBackupMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleCreateSQLBackup(msg)

	case views.SQLBackupActionResultMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleSQLBackupActionResult(msg)

	case views.DeleteInstanceConfirmedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleDeleteInstanceConfirmed(msg)

	case views.InstanceActionResultMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleInstanceActionResult(msg)

	// IAM: Service Accounts
	case views.ServiceAccountSelectedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleServiceAccountSelected(msg)

	case views.ServiceAccountCreateRequestMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleServiceAccountCreateRequest(msg)

	case views.ServiceAccountCreateCanceledMsg:
		a.handleServiceAccountCreateCanceled()
		return a, nil

	case views.CreateServiceAccountMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleCreateServiceAccount(msg)

	case views.DeleteServiceAccountConfirmedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleDeleteServiceAccountConfirmed(msg)

	case views.ToggleServiceAccountMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleToggleServiceAccount(msg)

	case views.ServiceAccountActionResultMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleServiceAccountActionResult(msg)

	// IAM: Service Account Keys
	case views.CreateServiceAccountKeyMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleCreateServiceAccountKey(msg)

	case views.DeleteServiceAccountKeyMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleDeleteServiceAccountKey(msg)

	case views.ServiceAccountKeyActionResultMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleServiceAccountKeyActionResult(msg)

	case views.DownloadServiceAccountKeyMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleDownloadServiceAccountKey(msg)

	// IAM: Policy editing
	case views.AddIAMBindingMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleAddIAMBinding(msg)

	case views.RemoveIAMBindingMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleRemoveIAMBinding(msg)

	case views.IAMPolicyUpdateResultMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleIAMPolicyUpdateResult(msg)

	// IAM: Custom Roles
	case views.CustomRoleSelectedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleCustomRoleSelected(msg)

	// Cloud Run
	case views.CloudRunServiceSelectedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleCloudRunServiceSelected(msg)

	case views.DeleteCloudRunServiceConfirmedMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleDeleteCloudRunServiceConfirmed(msg)

	case views.CloudRunServiceActionResultMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleCloudRunServiceActionResult(msg)

	case views.CloudRunTrafficUpdateMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleCloudRunTrafficUpdate(msg)

	case views.CloudRunEditRequestMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleCloudRunEditRequest(msg)

	case views.CloudRunCreateRequestMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleCloudRunCreateRequest(msg)

	case views.CloudRunEditResultMsg:
		//nolint:gocritic // evalOrder: return pattern is intentional for Bubble Tea model
		return a, a.handleCloudRunEditResult(msg)

	case views.CloudRunEditCanceledMsg:
		a.handleCloudRunEditCanceled()
		return a, nil

	// Logging
	case views.LogsViewRequestMsg:
		cmd := a.handleLogsRequest(msg)
		return a, cmd

	case components.FooterProjectClickedMsg:
		// Project section in footer was clicked, show project selector
		currentProjectID := ""
		if a.selectedProject != nil {
			currentProjectID = a.selectedProject.ID
		}
		a.projectSelector = projectselector.New(a.gcpClient, currentProjectID)
		a.showProjectSelector = true
		return a, a.projectSelector.Init()
	}

	// Delegate to current view (only if content is focused)
	var cmd tea.Cmd
	if a.focusedPanel == FocusContent || !a.sidebarActive() {
		if model := a.getCurrentViewModel(); model != nil {
			cmd = model.Update(msg)
		}
	}

	return a, cmd
}

// cleanup releases resources held by views
func (a *App) cleanup() {
	if a.bucketsView != nil {
		_ = a.bucketsView.Close() //nolint:errcheck // Best-effort cleanup on exit
	}
}

// startTask registers a new async task and returns a command to animate the spinner.
// Tasks are tracked in the context and displayed in the footer.
//
//nolint:gocritic // hugeParam: task struct size is acceptable for clarity
func (a *App) startTask(task context.Task) tea.Cmd {
	task.StartTime = time.Now()
	task.State = context.TaskRunning
	a.ctx.Tasks[task.ID] = task
	return nil // Could return spinner.Tick if we want animation
}

// GetContext returns the shared program context.
// Views can use this to access dimensions, styles, and task tracking.
func (a *App) GetContext() *context.ProgramContext {
	return a.ctx
}

// registerRunningTask adds a task to the footer with a spinner.
func (a *App) registerRunningTask(id, description string) {
	a.ctx.Tasks[id] = context.Task{
		ID:          id,
		Description: description,
		State:       context.TaskRunning,
		StartTime:   time.Now(),
	}
}

// clearRunningTasks removes all tasks still in TaskRunning state.
// Called when navigating back — in-flight async results will be dropped.
func (a *App) clearRunningTasks() {
	for id, task := range a.ctx.Tasks {
		if task.State == context.TaskRunning {
			delete(a.ctx.Tasks, id)
		}
	}
}

// finishTask marks a task as complete and schedules its removal after 2 seconds.
func (a *App) finishTask(taskID string, err error) tea.Cmd {
	if task, ok := a.ctx.Tasks[taskID]; ok {
		now := time.Now()
		task.FinishedTime = &now
		if err != nil {
			task.State = context.TaskError
			task.Error = err
		} else {
			task.State = context.TaskFinished
		}
		a.ctx.Tasks[taskID] = task

		// Schedule task removal after 2 seconds
		return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return context.TaskClearMsg{TaskID: taskID}
		})
	}
	return nil
}

// syncContext updates the shared context with current app state.
// Called after dimension changes or project selection.
func (a *App) syncContext() {
	contentWidth := a.layout.ContentWidth()
	contentHeight := a.layout.ContentHeight()

	a.ctx.SetDimensions(a.width, a.height, contentWidth, contentHeight)
	a.ctx.SidebarActive = a.sidebarActive()
	if a.sidebarActive() {
		a.ctx.SidebarWidth = a.sidebar.Width()
		// Pre-compute sidebar emoji budget. renderWithSidebar subtracts
		// max(sidebarEmojis + contentEmojis) from ContentWidth for MaxWidth.
		// The sidebar part is stable; content part is view-specific.
		a.ctx.EmojiWidthBudget = maxLineEmojiCount(a.sidebar.View())
	} else {
		a.ctx.EmojiWidthBudget = 0
	}
	if a.selectedProject != nil {
		a.ctx.ProjectID = a.selectedProject.ID
	} else {
		a.ctx.ProjectID = ""
	}
	a.ctx.Error = a.err
}

// toggleFocus switches focus between sidebar and content
func (a *App) toggleFocus() {
	if a.focusedPanel == FocusContent {
		a.focusedPanel = FocusSidebar
		a.sidebar.SetFocused(true)
	} else {
		a.focusedPanel = FocusContent
		a.sidebar.SetFocused(false)
	}
}

// updateViewSizes recalculates sizes for all views using the layout manager.
// Uses SetContext to propagate shared context to all views.
func (a *App) updateViewSizes() {
	// Update layout with sidebar state
	if a.sidebarActive() {
		a.layout.SetSidebarWidth(a.sidebar.Width())
		a.layout.SetSidebarActive(true)
	} else {
		a.layout.SetSidebarActive(false)
	}

	// Sync context with current dimensions before propagating
	a.syncContext()

	// Sidebar uses content height directly
	if a.sidebarActive() {
		a.sidebar.SetSize(a.ctx.ContentHeight)
	}

	// Propagate context to all views - they read dimensions from ctx
	if a.projectView != nil {
		a.projectView.SetContext(a.ctx)
	}

	if a.instancesView != nil {
		a.instancesView.SetContext(a.ctx)
	}
	if a.instanceDetailsView != nil {
		a.instanceDetailsView.SetContext(a.ctx)
	}
	if a.metadataView != nil {
		a.metadataView.SetContext(a.ctx)
	}
	if a.projectMetadataView != nil {
		a.projectMetadataView.SetContext(a.ctx)
	}
	if a.disksView != nil {
		a.disksView.SetContext(a.ctx)
	}
	if a.diskDetailsView != nil {
		a.diskDetailsView.SetContext(a.ctx)
	}
	if a.snapshotsView != nil {
		a.snapshotsView.SetContext(a.ctx)
	}
	if a.snapshotDetailsView != nil {
		a.snapshotDetailsView.SetContext(a.ctx)
	}
	if a.imagesView != nil {
		a.imagesView.SetContext(a.ctx)
	}
	if a.imageDetailsView != nil {
		a.imageDetailsView.SetContext(a.ctx)
	}
	if a.bucketsView != nil {
		a.bucketsView.SetContext(a.ctx)
	}
	if a.objectsView != nil {
		a.objectsView.SetContext(a.ctx)
	}
	if a.objectDetailsView != nil {
		a.objectDetailsView.SetContext(a.ctx)
	}
	if a.instanceEditorView != nil {
		a.instanceEditorView.SetContext(a.ctx)
	}
	if a.instanceCreateView != nil {
		a.instanceCreateView.SetContext(a.ctx)
	}
	if a.instanceConfigEditView != nil {
		a.instanceConfigEditView.SetContext(a.ctx)
	}
	if a.bucketCreateView != nil {
		a.bucketCreateView.SetContext(a.ctx)
	}
	if a.snapshotCreateView != nil {
		a.snapshotCreateView.SetContext(a.ctx)
	}
	if a.imageCreateView != nil {
		a.imageCreateView.SetContext(a.ctx)
	}
	if a.diskCreateView != nil {
		a.diskCreateView.SetContext(a.ctx)
	}
	if a.networksView != nil {
		a.networksView.SetContext(a.ctx)
	}
	if a.networkDetailsView != nil {
		a.networkDetailsView.SetContext(a.ctx)
	}
	if a.firewallsView != nil {
		a.firewallsView.SetContext(a.ctx)
	}
	if a.firewallDetailsView != nil {
		a.firewallDetailsView.SetContext(a.ctx)
	}
	if a.sqlInstancesView != nil {
		a.sqlInstancesView.SetContext(a.ctx)
	}
	if a.sqlInstanceDetailsView != nil {
		a.sqlInstanceDetailsView.SetContext(a.ctx)
	}
	if a.serviceAccountsView != nil {
		a.serviceAccountsView.SetContext(a.ctx)
	}
	if a.serviceAccountDetailsView != nil {
		a.serviceAccountDetailsView.SetContext(a.ctx)
	}
	if a.serviceAccountCreateView != nil {
		a.serviceAccountCreateView.SetContext(a.ctx)
	}
	if a.iamPolicyView != nil {
		a.iamPolicyView.SetContext(a.ctx)
	}
	if a.customRolesView != nil {
		a.customRolesView.SetContext(a.ctx)
	}
	if a.customRoleDetailsView != nil {
		a.customRoleDetailsView.SetContext(a.ctx)
	}
	if a.cloudRunServicesView != nil {
		a.cloudRunServicesView.SetContext(a.ctx)
	}
	if a.cloudRunServiceDetailsView != nil {
		a.cloudRunServiceDetailsView.SetContext(a.ctx)
	}
	if a.cloudRunServiceEditView != nil {
		a.cloudRunServiceEditView.SetContext(a.ctx)
	}
	if a.logsView != nil {
		a.logsView.SetContext(a.ctx)
	}
}

// View implements tea.Model
func (a *App) View() string {
	// Header with breadcrumb navigation (always show, even before window size is known)
	header := a.renderHeader()

	if a.width == 0 {
		// Before window size is known, show header with loading message
		return header + "\n\n  Loading..."
	}

	// Get dimensions from layout for consistent rendering
	_, headerHeight := a.layout.HeaderSize()
	contentHeight := a.layout.ContentHeight()
	_, footerHeight := a.layout.FooterSize()

	debugLog("=== View() called ===")
	debugLog("Terminal: width=%d, height=%d", a.width, a.height)
	debugLog("Layout: header=%d, content=%d, footer=%d, total=%d",
		headerHeight, contentHeight, footerHeight, headerHeight+contentHeight+footerHeight)

	// Main content area (with or without sidebar)
	var content string
	if a.sidebarActive() {
		content = a.renderWithSidebar()
	} else {
		content = a.renderCurrentView()
	}

	debugLogView("Raw header", header)
	debugLogView("Raw content", content)

	// Error display
	if a.err != nil {
		content += "\n" + a.styles.Error.Render("Error: "+a.err.Error())
	}

	// Use lipgloss.Place for positioning, then truncate at line boundaries.
	// Line-based truncation is ANSI-safe since escape sequences don't span lines.
	// This avoids the fragmentation issues that MaxHeight() causes in native terminals.
	//
	// Calculate safe width for each component individually.
	// lipgloss.Place() pads based on lipgloss.Width(), but terminals may render
	// some emojis wider than lipgloss measures. We reduce the target width
	// by the emoji count in each component to compensate.
	footer := a.renderFooter()

	// Calculate safe width for the component with the most emojis.
	// JoinVertical pads all components to match the widest one, so we must
	// use a single safe width based on the worst-case emoji count.
	// This ensures lines with emojis don't overflow after JoinVertical padding.
	allContent := header + "\n" + content + "\n" + footer
	safeWidth := SafeWidth(a.width, allContent)

	placedHeader := lipgloss.Place(safeWidth, headerHeight, lipgloss.Left, lipgloss.Top, header)
	placedContent := lipgloss.Place(safeWidth, contentHeight, lipgloss.Left, lipgloss.Top, content)
	placedFooter := lipgloss.Place(safeWidth, footerHeight, lipgloss.Left, lipgloss.Top, footer)

	debugLog("SafeWidth: %d (terminal=%d), maxEmojis=%d",
		safeWidth, a.width, maxLineEmojiCount(allContent))
	debugLogView("Placed header", placedHeader)
	debugLogView("Placed content", placedContent)
	debugLogView("Placed footer", placedFooter)

	styledHeader := truncateToHeight(placedHeader, headerHeight)
	styledContent := truncateToHeight(placedContent, contentHeight)
	styledFooter := truncateToHeight(placedFooter, footerHeight)

	// Compose final layout with guaranteed heights
	result := lipgloss.JoinVertical(
		lipgloss.Left,
		styledHeader,
		styledContent,
		styledFooter,
	)

	// Log width analysis
	resultLines := strings.Split(result, "\n")
	debugLog("Final result: %d lines", len(resultLines))
	for i, line := range resultLines {
		w := lipgloss.Width(line)
		tw := TerminalWidth(line)
		emojiCount := countWideEmojis(line)
		if tw > a.width {
			debugLog("! Line %d exceeds width: lipgloss=%d, terminal=%d > %d, emojis=%d", i, w, tw, a.width, emojiCount)
			// Show first 80 chars of raw line content for debugging
			preview := line
			if len(preview) > 80 {
				preview = preview[:80] + "..."
			}
			debugLog("  content: %q", preview)
		}
	}
	debugLog("")

	// Render project selector overlay if active (highest priority)
	if a.showProjectSelector {
		result = a.renderWithProjectSelector(result)
	}

	// Render command palette overlay if active
	if a.showCommandPalette {
		result = a.renderWithCommandPalette(result)
	}

	return result
}

// renderFooter syncs content to the footer component and renders it
func (a *App) renderFooter() string {
	if a.showHelp {
		return a.help.View(a.keys)
	}

	// Sync footer content based on current state
	a.syncFooter()
	return a.footer.View()
}
