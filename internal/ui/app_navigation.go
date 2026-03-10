package ui

import (
	gocontext "context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/sidebar"
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
	"github.com/slayer/gcon/internal/ui/views"
)

var errKeyFilenameExhausted = errors.New("could not find available filename for key file (tried 999 suffixes)")

// handleMouseEvent processes mouse events and routes them to appropriate components.
// Uses region-based click handling for better maintainability and correctness.
//
//nolint:gocognit // Mouse event routing - complexity 59
func (a *App) handleMouseEvent(msg tea.MouseMsg) tea.Cmd {
	// Fast path: ignore motion events entirely - they're too frequent and cause lag
	if msg.Action == tea.MouseActionMotion {
		return nil
	}

	// Get header height to adjust Y coordinate
	_, headerHeight := a.layout.HeaderSize()

	// Check if click is in header area
	if msg.Y < headerHeight {
		return nil
	}

	// Calculate sidebar offset
	sidebarWidth := 0
	if a.sidebarActive() {
		sidebarWidth = a.sidebar.Width()
	}

	// Get footer position (at bottom of screen)
	_, footerHeight := a.layout.FooterSize()
	footerY := a.height - footerHeight

	// Handle mouse clicks using region-based system
	// Only use regions for left-click press events to avoid performance overhead
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		// Check footer first (if click is in footer area)
		if msg.Y >= footerY {
			// Focus content on footer click
			if a.focusedPanel != FocusContent {
				a.focusedPanel = FocusContent
				a.sidebar.SetFocused(false)
			}

			if clickable, ok := interface{}(a.footer).(components.Clickable); ok {
				// Update regions with footer position (x=0, y=footerY)
				clickable.UpdateRegions(0, footerY)

				// Find which region was clicked
				regions := clickable.GetRegions()
				for _, region := range regions {
					if region.Bounds.Contains(msg.X, msg.Y) {
						return clickable.HandleRegionClick(region.ID)
					}
				}
			}
			return nil
		}

		// Check sidebar (if active and click is in sidebar area)
		if a.sidebarActive() && msg.X < sidebarWidth {
			// When collapsed in auto-hide mode, just expand + focus (no item selection)
			if a.sidebar.IsCollapsed() && a.sidebar.Mode() == sidebar.SidebarModeAutoHide {
				a.sidebar.Expand()
				a.focusedPanel = FocusSidebar
				a.sidebar.SetFocused(true)
				a.updateViewSizes()
				return nil
			}

			// Focus sidebar on click
			if a.focusedPanel != FocusSidebar {
				a.focusedPanel = FocusSidebar
				a.sidebar.SetFocused(true)
			}

			if clickable, ok := interface{}(a.sidebar).(components.Clickable); ok {
				// Update regions with sidebar position (x=0, y=headerHeight)
				clickable.UpdateRegions(0, headerHeight)

				// Find which region was clicked
				regions := clickable.GetRegions()
				for _, region := range regions {
					if region.Bounds.Contains(msg.X, msg.Y) {
						return clickable.HandleRegionClick(region.ID)
					}
				}
			}
			return nil
		} else {
			// Focus content on click
			if a.focusedPanel != FocusContent {
				a.focusedPanel = FocusContent
				a.sidebar.SetFocused(false)
			}

			// Collapse sidebar when clicking content area in auto-hide mode
			if a.sidebarActive() && a.sidebar.Mode() == sidebar.SidebarModeAutoHide && !a.sidebar.IsCollapsed() {
				a.sidebar.Collapse()
				a.updateViewSizes()
			}

			// Check content area
			if model := a.getCurrentViewModel(); model != nil {
				if clickable, ok := model.(components.Clickable); ok {
					// Update regions with content area position
					offsetX := sidebarWidth
					offsetY := headerHeight
					clickable.UpdateRegions(offsetX, offsetY)

					// Find which region was clicked
					regions := clickable.GetRegions()
					for _, region := range regions {
						if region.Bounds.Contains(msg.X, msg.Y) {
							return clickable.HandleRegionClick(region.ID)
						}
					}
				}
			}
			return nil
		}
	}

	// Only pass wheel scroll events to components, ignore motion events
	// Motion events are too frequent and cause performance issues
	if msg.Action == tea.MouseActionRelease {
		switch msg.Button {
		case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
			// Pass wheel scroll to components with adjusted coordinates
			adjustedMsg := msg
			adjustedMsg.Y -= headerHeight

			if a.sidebarActive() && adjustedMsg.X < sidebarWidth {
				return a.sidebar.Update(adjustedMsg)
			}

			adjustedMsg.X -= sidebarWidth
			if model := a.getCurrentViewModel(); model != nil {
				return model.Update(adjustedMsg)
			}
		}
	}

	return nil
}

// handleSidebarNavigation processes sidebar navigation messages and initializes views
//
//nolint:gocognit // Sidebar navigation routing - complexity 35
func (a *App) handleSidebarNavigation(msg sidebar.NavigateMsg) tea.Cmd {
	var cmd tea.Cmd

	// Map sidebar ViewType to app ViewType and navigate
	switch msg.ViewType {
	case sidebar.ViewInstances:
		if a.currentView != ViewInstances && a.currentView != ViewInstanceDetails && a.currentView != ViewInstanceEditor && a.currentView != ViewInstanceCreate && a.currentView != ViewInstanceConfigEdit {
			a.currentView = ViewInstances
			a.instanceDetailsView = nil
			a.selectedInstance = nil
			if a.instancesView == nil {
				a.instancesView = views.NewInstancesView(a.selectedProject.ID)
				a.updateViewSizes()
				cmd = a.instancesView.Init()
			}
		}
	case sidebar.ViewDisks:
		if a.currentView != ViewDisks {
			a.currentView = ViewDisks
			if a.disksView == nil {
				a.disksView = views.NewDisksView(a.selectedProject.ID)
				a.updateViewSizes()
				cmd = a.disksView.Init()
			}
		}
	case sidebar.ViewSnapshots:
		if a.currentView != ViewSnapshots {
			a.currentView = ViewSnapshots
			if a.snapshotsView == nil {
				a.snapshotsView = views.NewSnapshotsView(a.selectedProject.ID)
				a.updateViewSizes()
				cmd = a.snapshotsView.Init()
			}
		}
	case sidebar.ViewImages:
		if a.currentView != ViewImages {
			a.currentView = ViewImages
			if a.imagesView == nil {
				a.imagesView = views.NewImagesView(a.selectedProject.ID)
				a.updateViewSizes()
				cmd = a.imagesView.Init()
			}
		}
	case sidebar.ViewProjectMetadata:
		if a.selectedProject == nil {
			return nil
		}
		if a.currentView != ViewProjectMetadata {
			a.currentView = ViewProjectMetadata
			if a.instancesView != nil {
				a.projectMetadataView = views.NewProjectMetadataView(
					a.selectedProject.ID,
					a.instancesView.GetComputeClient(),
				)
				a.updateViewSizes()
				cmd = a.projectMetadataView.Init()
			}
		}
	case sidebar.ViewBuckets:
		if a.currentView != ViewBuckets && a.currentView != ViewObjects {
			a.currentView = ViewBuckets
			a.objectsView = nil
			a.selectedBucket = nil
			if a.bucketsView == nil {
				a.bucketsView = views.NewBucketsView(a.selectedProject.ID)
				a.updateViewSizes()
				cmd = a.bucketsView.Init()
			}
		}
	case sidebar.ViewNetworks:
		if a.currentView != ViewNetworks && a.currentView != ViewNetworkDetails {
			a.currentView = ViewNetworks
			if a.networksView == nil {
				a.networksView = views.NewNetworksView(a.selectedProject.ID)
				a.updateViewSizes()
				cmd = a.networksView.Init()
			}
		}
	case sidebar.ViewFirewall:
		if a.currentView != ViewFirewall && a.currentView != ViewFirewallDetails {
			a.currentView = ViewFirewall
			a.firewallDetailsView = nil
			a.selectedFirewall = nil
			if a.firewallsView == nil {
				a.firewallsView = views.NewFirewallsView(a.selectedProject.ID)
				a.updateViewSizes()
				cmd = a.firewallsView.Init()
			}
		}
	case sidebar.ViewSQLInstances:
		if a.currentView != ViewSQLInstances && a.currentView != ViewSQLInstanceDetails {
			a.currentView = ViewSQLInstances
			a.sqlInstanceDetailsView = nil
			a.selectedSQLInstance = nil
			if a.sqlInstancesView == nil {
				a.sqlInstancesView = views.NewSQLInstancesView(a.selectedProject.ID)
				a.updateViewSizes()
				cmd = a.sqlInstancesView.Init()
			}
		}
	case sidebar.ViewServiceAccounts:
		if a.currentView != ViewServiceAccounts && a.currentView != ViewServiceAccountDetails && a.currentView != ViewServiceAccountCreate {
			a.currentView = ViewServiceAccounts
			a.serviceAccountDetailsView = nil
			a.serviceAccountCreateView = nil
			a.selectedServiceAccount = nil
			if a.serviceAccountsView == nil {
				a.serviceAccountsView = views.NewServiceAccountsView(a.selectedProject.ID)
				a.updateViewSizes()
				cmd = a.serviceAccountsView.Init()
			}
		}
	case sidebar.ViewIAMPolicy:
		if a.currentView != ViewIAMPolicy {
			a.currentView = ViewIAMPolicy
			if a.iamPolicyView == nil {
				a.iamPolicyView = views.NewIAMPolicyView(a.selectedProject.ID)
				a.updateViewSizes()
				cmd = a.iamPolicyView.Init()
			}
		}
	case sidebar.ViewCustomRoles:
		if a.currentView != ViewCustomRoles && a.currentView != ViewCustomRoleDetails {
			a.currentView = ViewCustomRoles
			a.customRoleDetailsView = nil
			a.selectedCustomRole = nil
			if a.customRolesView == nil {
				a.customRolesView = views.NewCustomRolesView(a.selectedProject.ID)
				a.updateViewSizes()
				cmd = a.customRolesView.Init()
			}
		}

	case sidebar.ViewCloudRunServices:
		if a.currentView != ViewCloudRunServices && a.currentView != ViewCloudRunServiceDetails && a.currentView != ViewCloudRunServiceEdit {
			a.currentView = ViewCloudRunServices
			if a.cloudRunServiceDetailsView != nil {
				a.cloudRunServiceDetailsView.Close()
			}
			a.cloudRunServiceDetailsView = nil
			a.cloudRunServiceEditView = nil
			a.selectedCloudRunService = nil
			if a.cloudRunServicesView == nil {
				a.cloudRunServicesView = views.NewCloudRunServicesView(a.selectedProject.ID)
				a.updateViewSizes()
				cmd = a.cloudRunServicesView.Init()
			}
		}
	}

	a.updateSidebarActiveView()
	// Switch focus back to content after navigation
	a.focusedPanel = FocusContent
	a.sidebar.SetFocused(false)

	// Auto-collapse sidebar after leaf item selection in auto-hide mode
	if a.sidebar.Mode() == sidebar.SidebarModeAutoHide {
		a.sidebar.Collapse()
		a.updateViewSizes()
	}

	return cmd
}

// updateSidebarActiveView sets the active view highlight in sidebar based on current view
func (a *App) updateSidebarActiveView() {
	switch a.currentView {
	case ViewInstances, ViewInstanceDetails, ViewInstanceCreate, ViewInstanceConfigEdit:
		a.sidebar.SetActiveView(sidebar.ViewInstances)
	case ViewDisks, ViewDiskDetails:
		a.sidebar.SetActiveView(sidebar.ViewDisks)
	case ViewSnapshots, ViewSnapshotDetails:
		a.sidebar.SetActiveView(sidebar.ViewSnapshots)
	case ViewImages, ViewImageDetails:
		a.sidebar.SetActiveView(sidebar.ViewImages)
	case ViewBuckets, ViewObjects, ViewObjectDetails:
		a.sidebar.SetActiveView(sidebar.ViewBuckets)
	case ViewNetworks, ViewNetworkDetails:
		a.sidebar.SetActiveView(sidebar.ViewNetworks)
	case ViewFirewall, ViewFirewallDetails:
		a.sidebar.SetActiveView(sidebar.ViewFirewall)
	case ViewSQLInstances, ViewSQLInstanceDetails:
		a.sidebar.SetActiveView(sidebar.ViewSQLInstances)
	case ViewServiceAccounts, ViewServiceAccountDetails, ViewServiceAccountCreate:
		a.sidebar.SetActiveView(sidebar.ViewServiceAccounts)
	case ViewIAMPolicy:
		a.sidebar.SetActiveView(sidebar.ViewIAMPolicy)
	case ViewCustomRoles, ViewCustomRoleDetails:
		a.sidebar.SetActiveView(sidebar.ViewCustomRoles)
	case ViewCloudRunServices, ViewCloudRunServiceDetails:
		a.sidebar.SetActiveView(sidebar.ViewCloudRunServices)
	}
}

// handleProjectSelected processes project selection and navigates to instances view
func (a *App) handleProjectSelected(msg views.ProjectSelectedMsg) tea.Cmd {
	project := msg.Project
	a.selectedProject = &project
	// Track recent project access
	a.recentTracker.Track("project", project.ID, project.Name)
	// Navigate to instances view with sidebar
	a.currentView = ViewInstances
	a.instancesView = views.NewInstancesView(project.ID)
	a.focusedPanel = FocusContent
	a.updateSidebarActiveView()
	a.updateViewSizes()
	a.syncContext()
	return a.instancesView.Init()
}

// handleInstanceSelected processes instance selection and navigates to details view
//
//nolint:gocritic // hugeParam: message struct passed by value
func (a *App) handleInstanceSelected(msg views.InstanceSelectedMsg) tea.Cmd {
	inst := msg.Instance
	a.selectedInstance = &inst
	// Push current view onto stack for back navigation
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewInstanceDetails
	// Clean up old instance details view to stop any running tickers
	if a.instanceDetailsView != nil {
		a.instanceDetailsView.Close()
	}
	// Pass compute client from instances view to avoid re-initialization
	a.instanceDetailsView = views.NewInstanceDetailsView(
		a.selectedProject.ID,
		inst.Zone,
		inst.Name,
		a.instancesView.GetComputeClient(),
		a.gcpClient,
	)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.instanceDetailsView.Init()
}

// handleDiskSelected processes disk selection from disks view and navigates to details view
//
//nolint:gocritic // hugeParam: message struct passed by value
func (a *App) handleDiskSelected(msg views.DiskSelectedMsg) tea.Cmd {
	disk := msg.Disk
	a.selectedDisk = &disk
	// Track recent disk access
	a.recentTracker.Track("disk", disk.Name, disk.Name)
	// Push current view onto stack for back navigation
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewDiskDetails
	// Pass compute client from disks view to avoid re-initialization
	a.diskDetailsView = views.NewDiskDetailsView(
		a.selectedProject.ID,
		disk.Zone,
		disk.Name,
		a.disksView.GetComputeClient(),
	)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.diskDetailsView.Init()
}

// handleInstanceDiskSelected processes disk selection from instance details view
func (a *App) handleInstanceDiskSelected(msg views.InstanceDiskSelectedMsg) tea.Cmd {
	// Track recent disk access
	a.recentTracker.Track("disk", msg.DiskName, msg.DiskName)
	// Push current view onto stack for back navigation
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewDiskDetails
	// Pass compute client from instance details view
	a.diskDetailsView = views.NewDiskDetailsView(
		a.selectedProject.ID,
		msg.Zone,
		msg.DiskName,
		a.instanceDetailsView.GetComputeClient(),
	)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.diskDetailsView.Init()
}

// handleSnapshotSelected processes snapshot selection and navigates to details view
//
//nolint:gocritic // hugeParam: message struct passed by value
func (a *App) handleSnapshotSelected(msg views.SnapshotSelectedMsg) tea.Cmd {
	snapshot := msg.Snapshot
	a.selectedSnapshot = &snapshot
	// Track recent snapshot access
	a.recentTracker.Track("snapshot", snapshot.Name, snapshot.Name)
	// Push current view onto stack for back navigation
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewSnapshotDetails
	// Pass compute client from snapshots view to avoid re-initialization
	a.snapshotDetailsView = views.NewSnapshotDetailsView(
		a.selectedProject.ID,
		snapshot.Name,
		a.snapshotsView.GetComputeClient(),
	)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.snapshotDetailsView.Init()
}

// handleImageSelected processes image selection and navigates to details view
//
//nolint:gocritic // hugeParam: message struct passed by value
func (a *App) handleImageSelected(msg views.ImageSelectedMsg) tea.Cmd {
	image := msg.Image
	a.selectedImage = &image
	// Track recent image access
	a.recentTracker.Track("image", image.Name, image.Name)
	// Push current view onto stack for back navigation
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewImageDetails
	// Pass compute client from images view to avoid re-initialization
	a.imageDetailsView = views.NewImageDetailsView(
		a.selectedProject.ID,
		image.Name,
		a.imagesView.GetComputeClient(),
	)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.imageDetailsView.Init()
}

// handleBucketSelected processes bucket selection and navigates to objects view
func (a *App) handleBucketSelected(msg views.BucketSelectedMsg) tea.Cmd {
	if a.bucketsView == nil {
		return nil
	}
	storageClient := a.bucketsView.GetStorageClient()
	if storageClient == nil {
		return nil
	}
	bucket := msg.Bucket
	a.selectedBucket = &bucket
	// Track recent bucket access
	a.recentTracker.Track("bucket", bucket.Name, bucket.Name)
	// Push current view onto stack for back navigation
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewObjects
	a.objectsView = views.NewObjectsView(bucket.Name, storageClient)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.objectsView.Init()
}

// handleSnapshotDiskSelected processes disk selection from snapshot details view
func (a *App) handleSnapshotDiskSelected(msg views.SnapshotDiskSelectedMsg) tea.Cmd {
	// Track recent disk access
	a.recentTracker.Track("disk", msg.DiskName, msg.DiskName)
	// Push current view onto stack for back navigation
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewDiskDetails
	// Pass compute client from snapshot details view
	a.diskDetailsView = views.NewDiskDetailsView(
		a.selectedProject.ID,
		msg.Zone,
		msg.DiskName,
		a.snapshotDetailsView.GetComputeClient(),
	)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.diskDetailsView.Init()
}

// handleObjectSelected processes object selection and navigates to details view
//
//nolint:gocritic // hugeParam: message struct passed by value
func (a *App) handleObjectSelected(msg views.ObjectSelectedMsg) tea.Cmd {
	if a.objectsView == nil {
		return nil
	}
	storageClient := a.objectsView.GetStorageClient()
	if storageClient == nil {
		return nil
	}
	if a.selectedBucket == nil {
		return nil
	}

	obj := msg.Object
	a.selectedObject = &obj

	// Track recent object access
	a.recentTracker.Track("object", obj.Name, obj.Name)

	// Push current view onto stack for back navigation
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewObjectDetails

	// Get the display name (file name only, not full path)
	displayName := filepath.Base(obj.Name)

	a.objectDetailsView = views.NewObjectDetailsView(
		a.selectedBucket.Name,
		obj.Name,
		displayName,
		storageClient,
		msg.Action,
	)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.objectDetailsView.Init()
}

// handleObjectDeleted processes object deletion and refreshes the objects list
func (a *App) handleObjectDeleted(_ views.ObjectDeletedMsg) tea.Cmd {
	// Navigate back to objects list
	if len(a.viewStack) > 0 {
		lastViewIndex := len(a.viewStack) - 1
		a.currentView = a.viewStack[lastViewIndex]
		a.viewStack = a.viewStack[:lastViewIndex]
	}

	// Clean up object details view
	a.objectDetailsView = nil
	a.selectedObject = nil

	a.updateSidebarActiveView()

	// Refresh objects list if available
	if a.objectsView != nil {
		return a.objectsView.Init()
	}
	return nil
}

// handleProjectSwitch switches to a different project and reloads all state
func (a *App) handleProjectSwitch(newProject *gcp.Project) tea.Cmd {
	// Skip if selecting same project
	if a.selectedProject != nil && a.selectedProject.ID == newProject.ID {
		a.showProjectSelector = false
		return nil
	}

	// Update selected project
	a.selectedProject = newProject
	a.recentTracker.Track("project", newProject.ID, newProject.Name)

	// Clear all view instances to force reload
	a.clearAllViews()

	// Update context with new project
	a.syncContext()

	// Reload current view with new project
	cmd := a.reloadCurrentView(newProject.ID)

	// Close modal
	a.showProjectSelector = false

	// Sidebar will be activated automatically via sidebarActive() check

	return cmd
}

// clearAllViews nils out all view instances to force reload with new project
func (a *App) clearAllViews() {
	a.projectView = nil
	a.instancesView = nil
	a.instanceDetailsView = nil
	a.disksView = nil
	a.diskDetailsView = nil
	a.snapshotsView = nil
	a.snapshotDetailsView = nil
	a.imagesView = nil
	a.imageDetailsView = nil
	a.bucketsView = nil
	a.objectsView = nil
	a.objectDetailsView = nil
	a.projectMetadataView = nil
	a.metadataView = nil
	a.instanceEditorView = nil
	a.instanceCreateView = nil
	a.instanceConfigEditView = nil
	a.bucketCreateView = nil
	a.snapshotCreateView = nil
	a.imageCreateView = nil
	a.diskCreateView = nil
	a.networksView = nil
	a.networkDetailsView = nil
	a.firewallsView = nil
	a.firewallDetailsView = nil
	a.sqlInstancesView = nil
	a.sqlInstanceDetailsView = nil
	a.serviceAccountsView = nil
	a.serviceAccountDetailsView = nil
	a.serviceAccountCreateView = nil
	a.iamPolicyView = nil
	a.customRolesView = nil
	a.customRoleDetailsView = nil
	a.cloudRunServicesView = nil
	if a.cloudRunServiceDetailsView != nil {
		a.cloudRunServiceDetailsView.Close()
	}
	a.cloudRunServiceDetailsView = nil
	a.cloudRunServiceEditView = nil
	a.formDemoView = nil

	// Clear view stack
	a.viewStack = nil

	// Clear selected resources
	a.selectedInstance = nil
	a.selectedDisk = nil
	a.selectedSnapshot = nil
	a.selectedImage = nil
	a.selectedBucket = nil
	a.selectedObject = nil
	a.selectedNetwork = nil
	a.selectedFirewall = nil
	a.selectedSQLInstance = nil
	a.selectedServiceAccount = nil
	a.selectedCustomRole = nil
	a.selectedCloudRunService = nil
}

// reloadCurrentView recreates or switches views for the new project ID.
// For ViewProjects, switches to ViewInstances. For other views, reloads in-place.
func (a *App) reloadCurrentView(projectID string) tea.Cmd {
	switch a.currentView {
	case ViewInstances, ViewInstanceDetails, ViewInstanceCreate, ViewInstanceConfigEdit:
		// Return to instances list
		a.currentView = ViewInstances
		a.instanceCreateView = nil
		a.instanceConfigEditView = nil
		a.instancesView = views.NewInstancesView(projectID)
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a.instancesView.Init()

	case ViewDisks, ViewDiskDetails:
		// Return to disks list
		a.currentView = ViewDisks
		a.disksView = views.NewDisksView(projectID)
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a.disksView.Init()

	case ViewSnapshots, ViewSnapshotDetails:
		// Return to snapshots list
		a.currentView = ViewSnapshots
		a.snapshotsView = views.NewSnapshotsView(projectID)
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a.snapshotsView.Init()

	case ViewImages, ViewImageDetails:
		// Return to images list
		a.currentView = ViewImages
		a.imagesView = views.NewImagesView(projectID)
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a.imagesView.Init()

	case ViewBuckets, ViewObjects, ViewObjectDetails:
		// Return to buckets list
		a.currentView = ViewBuckets
		a.bucketsView = views.NewBucketsView(projectID)
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a.bucketsView.Init()

	case ViewNetworks, ViewNetworkDetails:
		// Return to networks list on project switch
		a.currentView = ViewNetworks
		a.networksView = views.NewNetworksView(projectID)
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a.networksView.Init()

	case ViewFirewall, ViewFirewallDetails:
		// Return to firewalls list on project switch
		a.currentView = ViewFirewall
		a.firewallsView = views.NewFirewallsView(projectID)
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a.firewallsView.Init()

	case ViewSQLInstances, ViewSQLInstanceDetails:
		// Return to SQL instances list on project switch
		a.currentView = ViewSQLInstances
		a.sqlInstancesView = views.NewSQLInstancesView(projectID)
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a.sqlInstancesView.Init()

	case ViewServiceAccounts, ViewServiceAccountDetails, ViewServiceAccountCreate:
		// Return to service accounts list on project switch
		a.currentView = ViewServiceAccounts
		a.serviceAccountsView = views.NewServiceAccountsView(projectID)
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a.serviceAccountsView.Init()

	case ViewIAMPolicy:
		a.currentView = ViewIAMPolicy
		a.iamPolicyView = views.NewIAMPolicyView(projectID)
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a.iamPolicyView.Init()

	case ViewCustomRoles, ViewCustomRoleDetails:
		a.currentView = ViewCustomRoles
		a.customRolesView = views.NewCustomRolesView(projectID)
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a.customRolesView.Init()

	case ViewCloudRunServices, ViewCloudRunServiceDetails:
		a.currentView = ViewCloudRunServices
		a.cloudRunServicesView = views.NewCloudRunServicesView(projectID)
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a.cloudRunServicesView.Init()

	case ViewProjectMetadata:
		// Reload metadata view
		a.currentView = ViewProjectMetadata
		// Need compute client, so initialize instances view first if not present
		if a.instancesView == nil {
			a.instancesView = views.NewInstancesView(projectID)
		}
		a.projectMetadataView = views.NewProjectMetadataView(
			projectID,
			a.instancesView.GetComputeClient(),
		)
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a.projectMetadataView.Init()

	case ViewProjects:
		// Switch to instances view after selecting project
		a.currentView = ViewInstances
		a.instancesView = views.NewInstancesView(projectID)
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a.instancesView.Init()

	default:
		// Default to instances view
		a.currentView = ViewInstances
		a.instancesView = views.NewInstancesView(projectID)
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a.instancesView.Init()
	}
}

// handleNetworkSelected processes network selection and navigates to details view
//
//nolint:gocritic // hugeParam: message struct passed by value
func (a *App) handleNetworkSelected(msg views.NetworkSelectedMsg) tea.Cmd {
	network := msg.Network
	a.selectedNetwork = &network
	// Track recent network access
	a.recentTracker.Track("network", network.Name, network.Name)
	// Push current view onto stack for back navigation
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewNetworkDetails
	// Reuse compute client from whichever view emitted the message
	var computeClient *gcp.ComputeClient
	if a.networksView != nil {
		computeClient = a.networksView.GetComputeClient()
	} else if a.firewallDetailsView != nil {
		computeClient = a.firewallDetailsView.GetComputeClient()
	}
	a.networkDetailsView = views.NewNetworkDetailsView(
		a.selectedProject.ID,
		network.Name,
		computeClient,
	)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.networkDetailsView.Init()
}

// handleInstanceEditRequest processes request to open instance editor
func (a *App) handleInstanceEditRequest(msg views.InstanceEditRequestMsg) tea.Cmd {
	// Push current view onto stack for back navigation
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewInstanceEditor

	// Get compute client from instance details view or instances view
	var computeClient *gcp.ComputeClient
	if a.instanceDetailsView != nil {
		computeClient = a.instanceDetailsView.GetComputeClient()
	} else if a.instancesView != nil {
		computeClient = a.instancesView.GetComputeClient()
	}

	// Create editor view
	a.instanceEditorView = views.NewInstanceEditorView(
		msg.ProjectID,
		msg.Zone,
		msg.InstanceName,
		computeClient,
	)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.instanceEditorView.Init()
}

// handleInstanceEditComplete processes successful instance edit
func (a *App) handleInstanceEditComplete(msg views.InstanceEditCompleteMsg) tea.Cmd {
	// Pop back to previous view
	if len(a.viewStack) > 0 {
		lastViewIndex := len(a.viewStack) - 1
		a.currentView = a.viewStack[lastViewIndex]
		a.viewStack = a.viewStack[:lastViewIndex]
	}

	// Clean up editor view
	a.instanceEditorView = nil

	a.updateSidebarActiveView()

	// Refresh instance details if that's where we came from to show updated labels
	if a.instanceDetailsView != nil && a.currentView == ViewInstanceDetails {
		return a.instanceDetailsView.Init()
	}

	return nil
}

// handleInstanceEditCanceled processes canceled instance edit
func (a *App) handleInstanceEditCancelled() {
	// Pop back to previous view
	if len(a.viewStack) > 0 {
		lastViewIndex := len(a.viewStack) - 1
		a.currentView = a.viewStack[lastViewIndex]
		a.viewStack = a.viewStack[:lastViewIndex]
	}

	// Clean up editor view
	a.instanceEditorView = nil

	a.updateSidebarActiveView()
}

// handleBucketCreateRequest processes request to open bucket create view
func (a *App) handleBucketCreateRequest(msg views.BucketCreateRequestMsg) tea.Cmd {
	// Push current view onto stack for back navigation
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewBucketCreate

	// Get storage client from buckets view
	var storageClient *gcp.StorageClient
	if a.bucketsView != nil {
		storageClient = a.bucketsView.GetStorageClient()
	}

	// Create the view
	a.bucketCreateView = views.NewBucketCreateView(msg.ProjectID, storageClient)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.bucketCreateView.Init()
}

// handleBucketCreated processes successful bucket creation
func (a *App) handleBucketCreated(_ views.BucketCreatedMsg) tea.Cmd {
	// Pop back to previous view
	if len(a.viewStack) > 0 {
		lastViewIndex := len(a.viewStack) - 1
		a.currentView = a.viewStack[lastViewIndex]
		a.viewStack = a.viewStack[:lastViewIndex]
	}

	// Clean up create view
	a.bucketCreateView = nil

	a.updateSidebarActiveView()

	// Refresh buckets list to show new bucket
	if a.bucketsView != nil && a.currentView == ViewBuckets {
		return a.bucketsView.Init()
	}

	return nil
}

// handleBucketCreateCanceled processes canceled bucket creation
func (a *App) handleBucketCreateCanceled() {
	// Pop back to previous view
	if len(a.viewStack) > 0 {
		lastViewIndex := len(a.viewStack) - 1
		a.currentView = a.viewStack[lastViewIndex]
		a.viewStack = a.viewStack[:lastViewIndex]
	}

	// Clean up create view
	a.bucketCreateView = nil

	a.updateSidebarActiveView()
}

// handleDeleteDiskConfirmed processes confirmed disk deletion
func (a *App) handleDeleteDiskConfirmed(msg views.DeleteDiskConfirmedMsg) tea.Cmd {
	// Get compute client from the appropriate view
	var computeClient *gcp.ComputeClient
	if a.disksView != nil {
		computeClient = a.disksView.GetComputeClient()
	} else if a.diskDetailsView != nil {
		computeClient = a.diskDetailsView.GetComputeClient()
	}

	if computeClient == nil || a.selectedProject == nil {
		return nil
	}

	projectID := a.selectedProject.ID
	diskName := msg.DiskName
	zone := msg.Zone

	return func() tea.Msg {
		err := computeClient.DeleteDisk(gocontext.Background(), projectID, zone, diskName)
		return views.DiskActionResultMsg{
			Action:  "delete",
			Success: err == nil,
			Error:   err,
		}
	}
}

// handleSnapshotCreateRequest opens the snapshot creation form
func (a *App) handleSnapshotCreateRequest(msg views.SnapshotCreateRequestMsg) tea.Cmd {
	// Get compute client from the appropriate view
	var computeClient *gcp.ComputeClient
	if a.disksView != nil {
		computeClient = a.disksView.GetComputeClient()
	} else if a.diskDetailsView != nil {
		computeClient = a.diskDetailsView.GetComputeClient()
	}

	if computeClient == nil || a.selectedProject == nil {
		return nil
	}

	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewSnapshotCreate
	a.snapshotCreateView = views.NewSnapshotCreateView(
		a.selectedProject.ID,
		msg.DiskName,
		msg.Zone,
		msg.AttachedTo,
		computeClient,
	)
	a.updateViewSizes()
	return a.snapshotCreateView.Init()
}

// handleSnapshotCreateCanceled processes canceled snapshot creation
func (a *App) handleSnapshotCreateCanceled() {
	// Pop back to previous view
	if len(a.viewStack) > 0 {
		lastViewIndex := len(a.viewStack) - 1
		a.currentView = a.viewStack[lastViewIndex]
		a.viewStack = a.viewStack[:lastViewIndex]
	}

	// Clean up create view
	a.snapshotCreateView = nil

	a.updateSidebarActiveView()
}

// handleCreateSnapshotFromDisk processes snapshot creation from a disk
func (a *App) handleCreateSnapshotFromDisk(msg views.CreateSnapshotFromDiskMsg) tea.Cmd {
	// Get compute client from the snapshot create view or disk views
	var computeClient *gcp.ComputeClient
	switch {
	case a.snapshotCreateView != nil:
		computeClient = a.snapshotCreateView.GetComputeClient()
	case a.disksView != nil:
		computeClient = a.disksView.GetComputeClient()
	case a.diskDetailsView != nil:
		computeClient = a.diskDetailsView.GetComputeClient()
	}

	if computeClient == nil || a.selectedProject == nil {
		return nil
	}

	projectID := a.selectedProject.ID
	config := gcp.SnapshotCreateConfig{
		Name:            msg.SnapshotName,
		Description:     msg.Description,
		Labels:          msg.Labels,
		StorageLocation: msg.StorageLocation,
	}

	return func() tea.Msg {
		err := computeClient.CreateSnapshotFromDisk(
			gocontext.Background(),
			projectID,
			msg.Zone,
			msg.DiskName,
			config,
		)
		return views.DiskActionResultMsg{
			Action:  "snapshot",
			Success: err == nil,
			Error:   err,
		}
	}
}

// handleImageCreateRequest opens the image creation form
func (a *App) handleImageCreateRequest(msg views.ImageCreateRequestMsg) tea.Cmd {
	// Get compute client from the appropriate view
	var computeClient *gcp.ComputeClient
	if a.disksView != nil {
		computeClient = a.disksView.GetComputeClient()
	} else if a.diskDetailsView != nil {
		computeClient = a.diskDetailsView.GetComputeClient()
	}

	if computeClient == nil || a.selectedProject == nil {
		return nil
	}

	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewImageCreate
	a.imageCreateView = views.NewImageCreateView(
		a.selectedProject.ID,
		msg.DiskName,
		msg.Zone,
		msg.AttachedTo,
		computeClient,
	)
	a.updateViewSizes()
	return a.imageCreateView.Init()
}

// handleImageCreateCanceled processes canceled image creation
func (a *App) handleImageCreateCanceled() {
	// Pop back to previous view
	if len(a.viewStack) > 0 {
		lastViewIndex := len(a.viewStack) - 1
		a.currentView = a.viewStack[lastViewIndex]
		a.viewStack = a.viewStack[:lastViewIndex]
	}

	// Clean up create view
	a.imageCreateView = nil

	a.updateSidebarActiveView()
}

// handleCreateImageFromDisk processes image creation from a disk
func (a *App) handleCreateImageFromDisk(msg views.CreateImageFromDiskMsg) tea.Cmd {
	// Get compute client from the image create view or disk views
	var computeClient *gcp.ComputeClient
	switch {
	case a.imageCreateView != nil:
		computeClient = a.imageCreateView.GetComputeClient()
	case a.disksView != nil:
		computeClient = a.disksView.GetComputeClient()
	case a.diskDetailsView != nil:
		computeClient = a.diskDetailsView.GetComputeClient()
	}

	if computeClient == nil || a.selectedProject == nil {
		return nil
	}

	projectID := a.selectedProject.ID
	config := gcp.ImageCreateConfig{
		Name:            msg.ImageName,
		Description:     msg.Description,
		Family:          msg.Family,
		Labels:          msg.Labels,
		StorageLocation: msg.StorageLocation,
		ForceCreate:     msg.ForceCreate,
	}

	return func() tea.Msg {
		err := computeClient.CreateImageFromDisk(
			gocontext.Background(),
			projectID,
			msg.Zone,
			msg.DiskName,
			config,
		)
		return views.DiskActionResultMsg{
			Action:  "image",
			Success: err == nil,
			Error:   err,
		}
	}
}

// handleDiskActionResult processes the result of a disk action
func (a *App) handleDiskActionResult(msg views.DiskActionResultMsg) tea.Cmd {
	if msg.Error != nil {
		a.err = msg.Error
		if msg.Action == "snapshot" && a.currentView == ViewSnapshotCreate && a.snapshotCreateView != nil {
			a.snapshotCreateView.SetError(msg.Error)
		}
		if msg.Action == "image" && a.currentView == ViewImageCreate && a.imageCreateView != nil {
			a.imageCreateView.SetError(msg.Error)
		}
		return nil
	}

	// On successful delete, navigate back to disks list and refresh
	if msg.Action == "delete" {
		// If we're in disk details view, go back to disks list
		if a.currentView == ViewDiskDetails {
			// Pop back to disks view
			if len(a.viewStack) > 0 {
				lastViewIndex := len(a.viewStack) - 1
				a.currentView = a.viewStack[lastViewIndex]
				a.viewStack = a.viewStack[:lastViewIndex]
			}

			// Clean up disk details view
			a.diskDetailsView = nil
			a.selectedDisk = nil

			a.updateSidebarActiveView()
		}

		// Refresh disks list
		if a.disksView != nil {
			return a.disksView.Init()
		}
	}

	// For snapshot creation, navigate back from create view
	if msg.Action == "snapshot" {
		if a.currentView == ViewSnapshotCreate {
			// Pop back to previous view
			if len(a.viewStack) > 0 {
				lastViewIndex := len(a.viewStack) - 1
				a.currentView = a.viewStack[lastViewIndex]
				a.viewStack = a.viewStack[:lastViewIndex]
			}
			a.snapshotCreateView = nil
			a.updateSidebarActiveView()
		}
		a.err = nil
	}

	// For image creation, navigate back from create view
	if msg.Action == "image" {
		if a.currentView == ViewImageCreate {
			// Pop back to previous view
			if len(a.viewStack) > 0 {
				lastViewIndex := len(a.viewStack) - 1
				a.currentView = a.viewStack[lastViewIndex]
				a.viewStack = a.viewStack[:lastViewIndex]
			}
			a.imageCreateView = nil
			a.updateSidebarActiveView()
		}
		a.err = nil
	}

	return nil
}

// handleDeleteSnapshotConfirmed processes confirmed snapshot deletion
func (a *App) handleDeleteSnapshotConfirmed(msg views.DeleteSnapshotConfirmedMsg) tea.Cmd {
	// Get compute client from the appropriate view
	var computeClient *gcp.ComputeClient
	if a.snapshotsView != nil {
		computeClient = a.snapshotsView.GetComputeClient()
	} else if a.snapshotDetailsView != nil {
		computeClient = a.snapshotDetailsView.GetComputeClient()
	}

	if computeClient == nil || a.selectedProject == nil {
		return nil
	}

	projectID := a.selectedProject.ID
	snapshotName := msg.SnapshotName

	return func() tea.Msg {
		err := computeClient.DeleteSnapshot(gocontext.Background(), projectID, snapshotName)
		return views.SnapshotActionResultMsg{
			Action:  "delete",
			Success: err == nil,
			Error:   err,
		}
	}
}

// handleSnapshotActionResult processes the result of a snapshot action
func (a *App) handleSnapshotActionResult(msg views.SnapshotActionResultMsg) tea.Cmd {
	if msg.Error != nil {
		a.err = msg.Error
		if msg.Action == "create_disk" && a.currentView == ViewDiskCreate && a.diskCreateView != nil {
			a.diskCreateView.SetError(msg.Error)
		}
		if msg.Action == "image" && a.currentView == ViewImageCreate && a.imageCreateView != nil {
			a.imageCreateView.SetError(msg.Error)
		}
		return nil
	}

	// On successful delete, navigate back to snapshots list and refresh
	if msg.Action == "delete" {
		// If we're in snapshot details view, go back to snapshots list
		if a.currentView == ViewSnapshotDetails {
			// Pop back to snapshots view
			if len(a.viewStack) > 0 {
				lastViewIndex := len(a.viewStack) - 1
				a.currentView = a.viewStack[lastViewIndex]
				a.viewStack = a.viewStack[:lastViewIndex]
			}

			// Clean up snapshot details view
			a.snapshotDetailsView = nil
			a.selectedSnapshot = nil

			a.updateSidebarActiveView()
		}

		// Refresh snapshots list
		if a.snapshotsView != nil {
			return a.snapshotsView.Init()
		}
	}

	// On successful disk creation, navigate to disks view
	if msg.Action == "create_disk" {
		// Pop back from disk create view
		if a.currentView == ViewDiskCreate {
			if len(a.viewStack) > 0 {
				lastViewIndex := len(a.viewStack) - 1
				a.currentView = a.viewStack[lastViewIndex]
				a.viewStack = a.viewStack[:lastViewIndex]
			}
			a.diskCreateView = nil
			a.updateSidebarActiveView()
		}
		a.err = nil
	}

	// On successful image creation, navigate back from create view
	if msg.Action == "image" {
		if a.currentView == ViewImageCreate {
			// Pop back to previous view
			if len(a.viewStack) > 0 {
				lastViewIndex := len(a.viewStack) - 1
				a.currentView = a.viewStack[lastViewIndex]
				a.viewStack = a.viewStack[:lastViewIndex]
			}
			a.imageCreateView = nil
			a.updateSidebarActiveView()
		}
		a.err = nil
	}

	return nil
}

// handleDiskCreateFromSnapshotRequest opens the disk creation form
func (a *App) handleDiskCreateFromSnapshotRequest(msg views.DiskCreateFromSnapshotRequestMsg) tea.Cmd {
	// Get compute client from the appropriate view
	var computeClient *gcp.ComputeClient
	if a.snapshotsView != nil {
		computeClient = a.snapshotsView.GetComputeClient()
	} else if a.snapshotDetailsView != nil {
		computeClient = a.snapshotDetailsView.GetComputeClient()
	}

	if computeClient == nil || a.selectedProject == nil {
		return nil
	}

	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewDiskCreate
	a.diskCreateView = views.NewDiskCreateView(
		a.selectedProject.ID,
		msg.SnapshotName,
		msg.SnapshotSize,
		computeClient,
	)
	a.updateViewSizes()
	return a.diskCreateView.Init()
}

// handleDiskCreateCanceled processes canceled disk creation
func (a *App) handleDiskCreateCanceled() {
	// Pop back to previous view
	if len(a.viewStack) > 0 {
		lastViewIndex := len(a.viewStack) - 1
		a.currentView = a.viewStack[lastViewIndex]
		a.viewStack = a.viewStack[:lastViewIndex]
	}

	// Clean up create view
	a.diskCreateView = nil

	a.updateSidebarActiveView()
}

// handleCreateDiskFromSnapshot processes disk creation from a snapshot
func (a *App) handleCreateDiskFromSnapshot(msg views.CreateDiskFromSnapshotMsg) tea.Cmd {
	// Get compute client from the disk create view or snapshot views
	var computeClient *gcp.ComputeClient
	switch {
	case a.diskCreateView != nil:
		computeClient = a.diskCreateView.GetComputeClient()
	case a.snapshotsView != nil:
		computeClient = a.snapshotsView.GetComputeClient()
	case a.snapshotDetailsView != nil:
		computeClient = a.snapshotDetailsView.GetComputeClient()
	}

	if computeClient == nil || a.selectedProject == nil {
		return nil
	}

	projectID := a.selectedProject.ID
	config := gcp.DiskCreateConfig{
		Name:           msg.DiskName,
		Description:    msg.Description,
		Zone:           msg.Zone,
		Type:           msg.DiskType,
		SizeGB:         msg.SizeGB,
		SourceSnapshot: msg.SnapshotName,
		Labels:         msg.Labels,
	}

	return func() tea.Msg {
		err := computeClient.CreateDiskFromSnapshot(
			gocontext.Background(),
			projectID,
			config,
		)
		return views.SnapshotActionResultMsg{
			Action:  "create_disk",
			Success: err == nil,
			Error:   err,
		}
	}
}

// handleImageCreateFromSnapshotRequest opens the image creation form for a snapshot
func (a *App) handleImageCreateFromSnapshotRequest(msg views.ImageCreateFromSnapshotRequestMsg) tea.Cmd {
	// Get compute client from the appropriate view
	var computeClient *gcp.ComputeClient
	if a.snapshotsView != nil {
		computeClient = a.snapshotsView.GetComputeClient()
	} else if a.snapshotDetailsView != nil {
		computeClient = a.snapshotDetailsView.GetComputeClient()
	}

	if computeClient == nil || a.selectedProject == nil {
		return nil
	}

	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewImageCreate
	a.imageCreateView = views.NewImageCreateViewFromSnapshot(
		a.selectedProject.ID,
		msg.SnapshotName,
		computeClient,
	)
	a.updateViewSizes()
	return a.imageCreateView.Init()
}

// handleImageCreateFromSnapshotCanceled processes canceled image creation from snapshot
func (a *App) handleImageCreateFromSnapshotCanceled() {
	// Pop back to previous view
	if len(a.viewStack) > 0 {
		lastViewIndex := len(a.viewStack) - 1
		a.currentView = a.viewStack[lastViewIndex]
		a.viewStack = a.viewStack[:lastViewIndex]
	}

	// Clean up create view
	a.imageCreateView = nil

	a.updateSidebarActiveView()
}

// handleCreateImageFromSnapshot processes image creation from a snapshot
func (a *App) handleCreateImageFromSnapshot(msg views.CreateImageFromSnapshotMsg) tea.Cmd {
	// Get compute client from the image create view or snapshot views
	var computeClient *gcp.ComputeClient
	switch {
	case a.imageCreateView != nil:
		computeClient = a.imageCreateView.GetComputeClient()
	case a.snapshotsView != nil:
		computeClient = a.snapshotsView.GetComputeClient()
	case a.snapshotDetailsView != nil:
		computeClient = a.snapshotDetailsView.GetComputeClient()
	}

	if computeClient == nil || a.selectedProject == nil {
		return nil
	}

	projectID := a.selectedProject.ID
	config := gcp.ImageCreateConfig{
		Name:            msg.ImageName,
		Description:     msg.Description,
		Family:          msg.Family,
		Labels:          msg.Labels,
		StorageLocation: msg.StorageLocation,
		SourceSnapshot:  msg.SnapshotName,
	}

	return func() tea.Msg {
		err := computeClient.CreateImageFromSnapshot(
			gocontext.Background(),
			projectID,
			config,
		)
		return views.SnapshotActionResultMsg{
			Action:  "image",
			Success: err == nil,
			Error:   err,
		}
	}
}

// handleDeleteImageConfirmed processes confirmed image deletion
func (a *App) handleDeleteImageConfirmed(msg views.DeleteImageConfirmedMsg) tea.Cmd {
	// Get compute client from the appropriate view
	var computeClient *gcp.ComputeClient
	if a.imagesView != nil {
		computeClient = a.imagesView.GetComputeClient()
	} else if a.imageDetailsView != nil {
		computeClient = a.imageDetailsView.GetComputeClient()
	}

	if computeClient == nil || a.selectedProject == nil {
		return nil
	}

	projectID := a.selectedProject.ID
	imageName := msg.ImageName

	return func() tea.Msg {
		err := computeClient.DeleteImage(gocontext.Background(), projectID, imageName)
		return views.ImageActionResultMsg{
			Action:  "delete",
			Success: err == nil,
			Error:   err,
		}
	}
}

// handleImageActionResult processes the result of an image action
func (a *App) handleImageActionResult(msg views.ImageActionResultMsg) tea.Cmd {
	if msg.Error != nil {
		a.err = msg.Error
		if msg.Action == "create_disk" && a.currentView == ViewDiskCreate && a.diskCreateView != nil {
			a.diskCreateView.SetError(msg.Error)
		}
		return nil
	}

	// On successful delete, navigate back to images list and refresh
	if msg.Action == "delete" {
		// If we're in image details view, go back to images list
		if a.currentView == ViewImageDetails {
			// Pop back to images view
			if len(a.viewStack) > 0 {
				lastViewIndex := len(a.viewStack) - 1
				a.currentView = a.viewStack[lastViewIndex]
				a.viewStack = a.viewStack[:lastViewIndex]
			}

			// Clean up image details view
			a.imageDetailsView = nil
			a.selectedImage = nil

			a.updateSidebarActiveView()
		}

		// Refresh images list
		if a.imagesView != nil {
			return a.imagesView.Init()
		}
	}

	// On successful disk creation, navigate back from create view
	if msg.Action == "create_disk" {
		if a.currentView == ViewDiskCreate {
			// Pop back to previous view
			if len(a.viewStack) > 0 {
				lastViewIndex := len(a.viewStack) - 1
				a.currentView = a.viewStack[lastViewIndex]
				a.viewStack = a.viewStack[:lastViewIndex]
			}
			a.diskCreateView = nil
			a.updateSidebarActiveView()
		}
		a.err = nil
	}

	return nil
}

// handleDiskCreateFromImageRequest opens the disk creation form for an image
func (a *App) handleDiskCreateFromImageRequest(msg views.DiskCreateFromImageRequestMsg) tea.Cmd {
	// Get compute client from the appropriate view
	var computeClient *gcp.ComputeClient
	if a.imagesView != nil {
		computeClient = a.imagesView.GetComputeClient()
	} else if a.imageDetailsView != nil {
		computeClient = a.imageDetailsView.GetComputeClient()
	}

	if computeClient == nil || a.selectedProject == nil {
		return nil
	}

	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewDiskCreate
	a.diskCreateView = views.NewDiskCreateViewFromImage(
		a.selectedProject.ID,
		msg.ImageName,
		msg.ImageSize,
		computeClient,
	)
	a.updateViewSizes()
	return a.diskCreateView.Init()
}

// handleCreateDiskFromImage processes disk creation from an image
func (a *App) handleCreateDiskFromImage(msg views.CreateDiskFromImageMsg) tea.Cmd {
	// Get compute client from the disk create view or image views
	var computeClient *gcp.ComputeClient
	switch {
	case a.diskCreateView != nil:
		computeClient = a.diskCreateView.GetComputeClient()
	case a.imagesView != nil:
		computeClient = a.imagesView.GetComputeClient()
	case a.imageDetailsView != nil:
		computeClient = a.imageDetailsView.GetComputeClient()
	}

	if computeClient == nil || a.selectedProject == nil {
		return nil
	}

	projectID := a.selectedProject.ID
	config := gcp.DiskCreateConfig{
		Name:        msg.DiskName,
		Description: msg.Description,
		Zone:        msg.Zone,
		Type:        msg.DiskType,
		SizeGB:      msg.SizeGB,
		SourceImage: msg.ImageName,
		Labels:      msg.Labels,
	}

	return func() tea.Msg {
		err := computeClient.CreateDiskFromImage(
			gocontext.Background(),
			projectID,
			config,
		)
		return views.ImageActionResultMsg{
			Action:  "create_disk",
			Success: err == nil,
			Error:   err,
		}
	}
}

// handleFirewallSelected processes firewall rule selection and navigates to details view
//
//nolint:gocritic // hugeParam: message struct passed by value
func (a *App) handleFirewallSelected(msg views.FirewallSelectedMsg) tea.Cmd {
	fw := msg.Firewall
	a.selectedFirewall = &fw
	// Track recent firewall access
	a.recentTracker.Track("firewall", fw.Name, fw.Name)
	// Push current view onto stack for back navigation
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewFirewallDetails

	// Pass compute client from firewalls view to avoid re-initialization
	var computeClient *gcp.ComputeClient
	if a.firewallsView != nil {
		computeClient = a.firewallsView.GetComputeClient()
	}

	a.firewallDetailsView = views.NewFirewallDetailsView(
		a.selectedProject.ID,
		fw.Name,
		computeClient,
	)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.firewallDetailsView.Init()
}

// handleDeleteFirewallConfirmed processes confirmed firewall rule deletion
func (a *App) handleDeleteFirewallConfirmed(msg views.DeleteFirewallConfirmedMsg) tea.Cmd {
	// Get compute client from the appropriate view
	var computeClient *gcp.ComputeClient
	if a.firewallsView != nil {
		computeClient = a.firewallsView.GetComputeClient()
	} else if a.firewallDetailsView != nil {
		computeClient = a.firewallDetailsView.GetComputeClient()
	}

	if computeClient == nil || a.selectedProject == nil {
		return nil
	}

	projectID := a.selectedProject.ID
	ruleName := msg.RuleName

	return func() tea.Msg {
		err := computeClient.DeleteFirewallRule(gocontext.Background(), projectID, ruleName)
		return views.FirewallActionResultMsg{
			Action:  "delete",
			Success: err == nil,
			Error:   err,
		}
	}
}

// handleToggleFirewall processes firewall enable/disable toggle
func (a *App) handleToggleFirewall(msg views.ToggleFirewallMsg) tea.Cmd {
	// Get compute client from the appropriate view
	var computeClient *gcp.ComputeClient
	if a.firewallsView != nil {
		computeClient = a.firewallsView.GetComputeClient()
	} else if a.firewallDetailsView != nil {
		computeClient = a.firewallDetailsView.GetComputeClient()
	}

	if computeClient == nil || a.selectedProject == nil {
		return nil
	}

	projectID := a.selectedProject.ID
	ruleName := msg.RuleName
	disable := msg.Disable

	action := "enable"
	if disable {
		action = "disable"
	}

	return func() tea.Msg {
		err := computeClient.SetFirewallRuleDisabled(gocontext.Background(), projectID, ruleName, disable)
		return views.FirewallActionResultMsg{
			Action:  action,
			Success: err == nil,
			Error:   err,
		}
	}
}

// handleFirewallActionResult processes the result of a firewall action
func (a *App) handleFirewallActionResult(msg views.FirewallActionResultMsg) tea.Cmd {
	if msg.Error != nil {
		a.err = msg.Error
		return nil
	}

	// On successful delete, navigate back to firewalls list and refresh
	if msg.Action == "delete" {
		if a.currentView == ViewFirewallDetails {
			// Pop back to firewalls view
			if len(a.viewStack) > 0 {
				lastViewIndex := len(a.viewStack) - 1
				a.currentView = a.viewStack[lastViewIndex]
				a.viewStack = a.viewStack[:lastViewIndex]
			}

			// Clean up firewall details view
			a.firewallDetailsView = nil
			a.selectedFirewall = nil

			a.updateSidebarActiveView()
		}

		// Refresh firewalls list
		if a.firewallsView != nil {
			return a.firewallsView.Init()
		}
	}

	// On successful enable/disable, refresh both views so list stays in sync
	if msg.Action == "enable" || msg.Action == "disable" {
		var cmds []tea.Cmd
		if a.currentView == ViewFirewallDetails && a.firewallDetailsView != nil {
			if cmd := a.firewallDetailsView.Init(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if a.firewallsView != nil {
			if cmd := a.firewallsView.Init(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if len(cmds) > 0 {
			return tea.Batch(cmds...)
		}
	}

	return nil
}

// handleDeleteInstanceConfirmed processes confirmed instance deletion
func (a *App) handleDeleteInstanceConfirmed(msg views.DeleteInstanceConfirmedMsg) tea.Cmd {
	// Get compute client from the appropriate view
	var computeClient *gcp.ComputeClient
	if a.instancesView != nil {
		computeClient = a.instancesView.GetComputeClient()
	} else if a.instanceDetailsView != nil {
		computeClient = a.instanceDetailsView.GetComputeClient()
	}

	if computeClient == nil || a.selectedProject == nil {
		return nil
	}

	projectID := a.selectedProject.ID
	instanceName := msg.InstanceName
	zone := msg.Zone

	return func() tea.Msg {
		err := computeClient.DeleteInstance(gocontext.Background(), projectID, zone, instanceName)
		return views.InstanceActionResultMsg{
			Action:  "delete",
			Success: err == nil,
			Error:   err,
		}
	}
}

// handleInstanceActionResult processes the result of an instance action
func (a *App) handleInstanceActionResult(msg views.InstanceActionResultMsg) tea.Cmd {
	if msg.Error != nil {
		a.err = msg.Error
		return nil
	}

	// On successful delete, navigate back to instances list and refresh
	if msg.Action == "delete" {
		// If we're in instance details view, go back to instances list
		if a.currentView == ViewInstanceDetails {
			// Pop back to instances view
			if len(a.viewStack) > 0 {
				lastViewIndex := len(a.viewStack) - 1
				a.currentView = a.viewStack[lastViewIndex]
				a.viewStack = a.viewStack[:lastViewIndex]
			}

			// Clean up instance details view
			if a.instanceDetailsView != nil {
				a.instanceDetailsView.Close()
			}
			a.instanceDetailsView = nil
			a.selectedInstance = nil

			a.updateSidebarActiveView()
		}

		// Refresh instances list
		if a.instancesView != nil {
			return a.instancesView.Init()
		}
	}

	return nil
}

// handleSQLInstanceSelected processes SQL instance selection and navigates to details view
//
//nolint:gocritic // hugeParam: message struct passed by value
func (a *App) handleSQLInstanceSelected(msg views.SQLInstanceSelectedMsg) tea.Cmd {
	inst := msg.Instance
	a.selectedSQLInstance = &inst
	// Push current view onto stack for back navigation
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewSQLInstanceDetails

	// Pass SQL client from list view to avoid re-initialization
	var sqlClient *gcp.SQLClient
	if a.sqlInstancesView != nil {
		sqlClient = a.sqlInstancesView.GetSQLClient()
	}

	a.sqlInstanceDetailsView = views.NewSQLInstanceDetailsView(
		a.selectedProject.ID,
		inst.Name,
		sqlClient,
	)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.sqlInstanceDetailsView.Init()
}

// handleSQLInstanceAction processes SQL instance lifecycle actions (start/stop/restart)
func (a *App) handleSQLInstanceAction(msg views.SQLInstanceActionMsg) tea.Cmd {
	// Get SQL client from the appropriate view
	var sqlClient *gcp.SQLClient
	if a.sqlInstancesView != nil {
		sqlClient = a.sqlInstancesView.GetSQLClient()
	} else if a.sqlInstanceDetailsView != nil {
		sqlClient = a.sqlInstanceDetailsView.GetSQLClient()
	}

	if sqlClient == nil || a.selectedProject == nil {
		return nil
	}

	projectID := a.selectedProject.ID
	instanceName := msg.InstanceName
	action := msg.Action

	// Show progress in footer status bar
	taskID := "sql-" + action + "-" + instanceName
	actionLabels := map[string]string{"start": "Starting", "stop": "Stopping", "restart": "Restarting"}
	taskCmd := a.startTask(context.Task{
		ID:          taskID,
		Description: actionLabels[action] + " " + instanceName + "...",
	})

	apiCmd := func() tea.Msg {
		var err error
		switch action {
		case "start":
			err = sqlClient.StartInstance(gocontext.Background(), projectID, instanceName)
		case "stop":
			err = sqlClient.StopInstance(gocontext.Background(), projectID, instanceName)
		case "restart":
			err = sqlClient.RestartInstance(gocontext.Background(), projectID, instanceName)
		}
		return views.SQLInstanceActionResultMsg{
			InstanceName: instanceName,
			Action:       action,
			Success:      err == nil,
			Error:        err,
		}
	}

	return tea.Batch(taskCmd, apiCmd)
}

// handleDeleteSQLInstanceConfirmed processes confirmed SQL instance deletion
func (a *App) handleDeleteSQLInstanceConfirmed(msg views.DeleteSQLInstanceConfirmedMsg) tea.Cmd {
	// Get SQL client from the appropriate view
	var sqlClient *gcp.SQLClient
	if a.sqlInstancesView != nil {
		sqlClient = a.sqlInstancesView.GetSQLClient()
	} else if a.sqlInstanceDetailsView != nil {
		sqlClient = a.sqlInstanceDetailsView.GetSQLClient()
	}

	if sqlClient == nil || a.selectedProject == nil {
		return nil
	}

	projectID := a.selectedProject.ID
	instanceName := msg.InstanceName

	return func() tea.Msg {
		err := sqlClient.DeleteInstance(gocontext.Background(), projectID, instanceName)
		return views.SQLInstanceActionResultMsg{
			InstanceName: instanceName,
			Action:       "delete",
			Success:      err == nil,
			Error:        err,
		}
	}
}

// handleSQLInstanceActionResult processes the result of a SQL instance action
func (a *App) handleSQLInstanceActionResult(msg views.SQLInstanceActionResultMsg) tea.Cmd {
	// Clear the progress task from status bar
	taskID := "sql-" + msg.Action + "-" + msg.InstanceName
	delete(a.ctx.Tasks, taskID)

	if msg.Error != nil {
		a.err = msg.Error
		// Refresh views to sync with actual GCP state (e.g. instance may have
		// been stopped by a previous call and our cached state is stale)
		return a.refreshSQLViews()
	}

	// On successful delete, navigate back to SQL instances list and refresh
	if msg.Action == "delete" {
		if a.currentView == ViewSQLInstanceDetails {
			// Pop back to SQL instances view
			if len(a.viewStack) > 0 {
				lastViewIndex := len(a.viewStack) - 1
				a.currentView = a.viewStack[lastViewIndex]
				a.viewStack = a.viewStack[:lastViewIndex]
			}

			// Clean up SQL instance details view
			a.sqlInstanceDetailsView = nil
			a.selectedSQLInstance = nil

			a.updateSidebarActiveView()
		}

		// Refresh SQL instances list
		if a.sqlInstancesView != nil {
			return a.sqlInstancesView.Init()
		}
	}

	// On successful start/stop/restart, refresh both views
	if msg.Action == "start" || msg.Action == "stop" || msg.Action == "restart" {
		return a.refreshSQLViews()
	}

	return nil
}

// refreshSQLViews reloads data in both the SQL instances list and details views.
// Used after actions and on errors to keep UI in sync with actual GCP state.
func (a *App) refreshSQLViews() tea.Cmd {
	var cmds []tea.Cmd
	if a.currentView == ViewSQLInstanceDetails && a.sqlInstanceDetailsView != nil {
		if cmd := a.sqlInstanceDetailsView.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if a.sqlInstancesView != nil {
		if cmd := a.sqlInstancesView.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) > 0 {
		return tea.Batch(cmds...)
	}
	return nil
}

// handleCreateSQLBackup processes on-demand backup creation
func (a *App) handleCreateSQLBackup(msg views.CreateSQLBackupMsg) tea.Cmd {
	// Get SQL client from the appropriate view
	var sqlClient *gcp.SQLClient
	if a.sqlInstanceDetailsView != nil {
		sqlClient = a.sqlInstanceDetailsView.GetSQLClient()
	} else if a.sqlInstancesView != nil {
		sqlClient = a.sqlInstancesView.GetSQLClient()
	}

	if sqlClient == nil || a.selectedProject == nil {
		return nil
	}

	projectID := a.selectedProject.ID
	instanceName := msg.InstanceName
	description := msg.Description

	return func() tea.Msg {
		err := sqlClient.CreateBackupRun(gocontext.Background(), projectID, instanceName, description)
		return views.SQLBackupActionResultMsg{
			Action:  "create_backup",
			Success: err == nil,
			Error:   err,
		}
	}
}

// handleSQLBackupActionResult processes the result of a backup action
func (a *App) handleSQLBackupActionResult(msg views.SQLBackupActionResultMsg) tea.Cmd {
	if msg.Error != nil {
		a.err = msg.Error
		return nil
	}

	// On successful backup creation, refresh the details view to show new backup
	if msg.Action == "create_backup" && a.sqlInstanceDetailsView != nil {
		return a.sqlInstanceDetailsView.Init()
	}

	return nil
}

// --- IAM: Service Accounts ---

// handleServiceAccountSelected navigates to service account details view
//
//nolint:gocritic // hugeParam: message struct passed by value
func (a *App) handleServiceAccountSelected(msg views.ServiceAccountSelectedMsg) tea.Cmd {
	sa := msg.ServiceAccount
	a.selectedServiceAccount = &sa

	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewServiceAccountDetails

	// Reuse IAM client from list view
	var iamClient *gcp.IAMClient
	if a.serviceAccountsView != nil {
		iamClient = a.serviceAccountsView.GetIAMClient()
	}

	a.serviceAccountDetailsView = views.NewServiceAccountDetailsView(
		a.selectedProject.ID,
		sa.Email,
		iamClient,
	)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.serviceAccountDetailsView.Init()
}

// handleServiceAccountCreateRequest opens the service account creation form
func (a *App) handleServiceAccountCreateRequest(_ views.ServiceAccountCreateRequestMsg) tea.Cmd {
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewServiceAccountCreate

	// Reuse IAM client from list or details view
	var iamClient *gcp.IAMClient
	if a.serviceAccountsView != nil {
		iamClient = a.serviceAccountsView.GetIAMClient()
	} else if a.serviceAccountDetailsView != nil {
		iamClient = a.serviceAccountDetailsView.GetIAMClient()
	}

	a.serviceAccountCreateView = views.NewServiceAccountCreateView(
		a.selectedProject.ID,
		iamClient,
	)
	a.updateViewSizes()
	return a.serviceAccountCreateView.Init()
}

// handleServiceAccountCreateCanceled navigates back from create form
func (a *App) handleServiceAccountCreateCanceled() {
	if len(a.viewStack) > 0 {
		lastViewIndex := len(a.viewStack) - 1
		a.currentView = a.viewStack[lastViewIndex]
		a.viewStack = a.viewStack[:lastViewIndex]
	}
	a.serviceAccountCreateView = nil
	a.updateSidebarActiveView()
}

// handleCreateServiceAccount performs the actual API call to create a service account
func (a *App) handleCreateServiceAccount(msg views.CreateServiceAccountMsg) tea.Cmd {
	// Get IAM client from the create view or list view
	var iamClient *gcp.IAMClient
	switch {
	case a.serviceAccountCreateView != nil:
		iamClient = a.serviceAccountCreateView.GetIAMClient()
	case a.serviceAccountsView != nil:
		iamClient = a.serviceAccountsView.GetIAMClient()
	}

	if iamClient == nil || a.selectedProject == nil {
		// Return error so create view exits saving state
		return func() tea.Msg {
			return views.ServiceAccountActionResultMsg{
				Action:  "create",
				Success: false,
				Error:   uierrors.ErrIAMClientNotInitialized,
			}
		}
	}

	a.registerRunningTask("sa-create", "Creating service account...")
	return func() tea.Msg {
		_, err := iamClient.CreateServiceAccount(
			gocontext.Background(),
			msg.ProjectID,
			msg.AccountID,
			msg.DisplayName,
			msg.Description,
		)
		return views.ServiceAccountActionResultMsg{
			Action:  "create",
			Success: err == nil,
			Error:   err,
		}
	}
}

// handleDeleteServiceAccountConfirmed performs the delete API call
func (a *App) handleDeleteServiceAccountConfirmed(msg views.DeleteServiceAccountConfirmedMsg) tea.Cmd {
	var iamClient *gcp.IAMClient
	if a.serviceAccountsView != nil {
		iamClient = a.serviceAccountsView.GetIAMClient()
	} else if a.serviceAccountDetailsView != nil {
		iamClient = a.serviceAccountDetailsView.GetIAMClient()
	}

	if iamClient == nil {
		return func() tea.Msg {
			return views.ServiceAccountActionResultMsg{
				Action: "delete", Success: false, Error: uierrors.ErrIAMClientNotInitialized,
			}
		}
	}

	a.registerRunningTask("sa-delete", "Deleting service account...")
	email := msg.Email
	return func() tea.Msg {
		err := iamClient.DeleteServiceAccount(gocontext.Background(), email)
		return views.ServiceAccountActionResultMsg{
			Action:  "delete",
			Success: err == nil,
			Error:   err,
		}
	}
}

// handleToggleServiceAccount enables or disables a service account
func (a *App) handleToggleServiceAccount(msg views.ToggleServiceAccountMsg) tea.Cmd {
	var iamClient *gcp.IAMClient
	if a.serviceAccountsView != nil {
		iamClient = a.serviceAccountsView.GetIAMClient()
	} else if a.serviceAccountDetailsView != nil {
		iamClient = a.serviceAccountDetailsView.GetIAMClient()
	}

	if iamClient == nil {
		action := "enable"
		if msg.Disable {
			action = "disable"
		}
		return func() tea.Msg {
			return views.ServiceAccountActionResultMsg{
				Action: action, Success: false, Error: uierrors.ErrIAMClientNotInitialized,
			}
		}
	}

	email := msg.Email
	disable := msg.Disable
	action := "enable"
	taskDesc := "Enabling service account..."
	if disable {
		action = "disable"
		taskDesc = "Disabling service account..."
	}

	a.registerRunningTask("sa-toggle", taskDesc)
	return func() tea.Msg {
		var err error
		if disable {
			err = iamClient.DisableServiceAccount(gocontext.Background(), email)
		} else {
			err = iamClient.EnableServiceAccount(gocontext.Background(), email)
		}
		return views.ServiceAccountActionResultMsg{
			Action:  action,
			Success: err == nil,
			Error:   err,
		}
	}
}

// handleServiceAccountActionResult processes the result of a service account action
//
//nolint:cyclop // branches for each action type
func (a *App) handleServiceAccountActionResult(msg views.ServiceAccountActionResultMsg) tea.Cmd {
	// Clear in-progress task for the completed operation
	var taskID string
	switch msg.Action {
	case "create":
		taskID = "sa-create"
	case "delete":
		taskID = "sa-delete"
	case "enable", "disable":
		taskID = "sa-toggle"
	}

	var cmds []tea.Cmd
	if taskID != "" {
		if cmd := a.finishTask(taskID, msg.Error); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if msg.Error != nil {
		a.err = msg.Error
		// Propagate error to create view if active
		if msg.Action == "create" && a.currentView == ViewServiceAccountCreate && a.serviceAccountCreateView != nil {
			a.serviceAccountCreateView.SetError(msg.Error)
		}
		return tea.Batch(cmds...)
	}

	// On successful create, navigate back and refresh list
	if msg.Action == "create" {
		if a.currentView == ViewServiceAccountCreate {
			if len(a.viewStack) > 0 {
				lastViewIndex := len(a.viewStack) - 1
				a.currentView = a.viewStack[lastViewIndex]
				a.viewStack = a.viewStack[:lastViewIndex]
			}
			a.serviceAccountCreateView = nil
			a.updateSidebarActiveView()
		}
		if a.serviceAccountsView != nil {
			cmds = append(cmds, a.serviceAccountsView.Init())
		}
		return tea.Batch(cmds...)
	}

	// On successful delete, navigate back to list and refresh
	if msg.Action == "delete" {
		if a.currentView == ViewServiceAccountDetails {
			if len(a.viewStack) > 0 {
				lastViewIndex := len(a.viewStack) - 1
				a.currentView = a.viewStack[lastViewIndex]
				a.viewStack = a.viewStack[:lastViewIndex]
			}
			a.serviceAccountDetailsView = nil
			a.selectedServiceAccount = nil
			a.updateSidebarActiveView()
		}
		if a.serviceAccountsView != nil {
			cmds = append(cmds, a.serviceAccountsView.Init())
		}
		return tea.Batch(cmds...)
	}

	// On successful enable/disable, refresh both views to stay in sync
	if msg.Action == "enable" || msg.Action == "disable" {
		if a.currentView == ViewServiceAccountDetails && a.serviceAccountDetailsView != nil {
			cmds = append(cmds, a.serviceAccountDetailsView.Init())
		}
		if a.serviceAccountsView != nil {
			cmds = append(cmds, a.serviceAccountsView.Init())
		}
	}

	return tea.Batch(cmds...)
}

// handleCreateServiceAccountKey creates a new key for a service account
func (a *App) handleCreateServiceAccountKey(msg views.CreateServiceAccountKeyMsg) tea.Cmd {
	var iamClient *gcp.IAMClient
	if a.serviceAccountDetailsView != nil {
		iamClient = a.serviceAccountDetailsView.GetIAMClient()
	} else if a.serviceAccountsView != nil {
		iamClient = a.serviceAccountsView.GetIAMClient()
	}

	if iamClient == nil {
		return nil
	}

	a.registerRunningTask("sa-key-create", "Creating service account key...")
	email := msg.Email
	return func() tea.Msg {
		keyJSON, keyMeta, err := iamClient.CreateServiceAccountKey(gocontext.Background(), email)
		result := views.ServiceAccountKeyActionResultMsg{
			Action:  "create_key",
			Success: err == nil,
			Error:   err,
			KeyJSON: keyJSON,
		}
		if keyMeta != nil {
			result.KeyID = keyMeta.KeyID
		}
		return result
	}
}

// handleDeleteServiceAccountKey deletes a service account key
func (a *App) handleDeleteServiceAccountKey(msg views.DeleteServiceAccountKeyMsg) tea.Cmd {
	var iamClient *gcp.IAMClient
	if a.serviceAccountDetailsView != nil {
		iamClient = a.serviceAccountDetailsView.GetIAMClient()
	} else if a.serviceAccountsView != nil {
		iamClient = a.serviceAccountsView.GetIAMClient()
	}

	if iamClient == nil {
		return nil
	}

	a.registerRunningTask("sa-key-delete", "Deleting service account key...")
	keyName := msg.KeyName
	return func() tea.Msg {
		err := iamClient.DeleteServiceAccountKey(gocontext.Background(), keyName)
		return views.ServiceAccountKeyActionResultMsg{
			Action:  "delete_key",
			Success: err == nil,
			Error:   err,
		}
	}
}

// handleServiceAccountKeyActionResult processes the result of a key action
func (a *App) handleServiceAccountKeyActionResult(msg views.ServiceAccountKeyActionResultMsg) tea.Cmd {
	// Clear in-progress task
	var taskID string
	switch msg.Action {
	case "create_key":
		taskID = "sa-key-create"
	case "delete_key":
		taskID = "sa-key-delete"
	}

	var cmds []tea.Cmd
	if taskID != "" {
		if cmd := a.finishTask(taskID, msg.Error); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if msg.Error != nil {
		a.err = msg.Error
		return tea.Batch(cmds...)
	}

	// On successful key create, pass key data to view for explicit download
	if msg.Action == "create_key" && msg.KeyJSON != nil {
		if a.serviceAccountDetailsView != nil {
			a.serviceAccountDetailsView.SetPendingKey(msg.KeyJSON, msg.KeyID)
		}
	}

	// Refresh details view to show updated key list
	if a.serviceAccountDetailsView != nil {
		cmds = append(cmds, a.serviceAccountDetailsView.Init())
	}
	return tea.Batch(cmds...)
}

// handleDownloadServiceAccountKey saves a pending key to disk on user request
func (a *App) handleDownloadServiceAccountKey(msg views.DownloadServiceAccountKeyMsg) tea.Cmd {
	keyID := msg.KeyID
	if keyID == "" {
		keyID = "new-key"
	}

	savedPath, err := writeKeyFile(keyID, msg.KeyJSON)

	var cmds []tea.Cmd
	if err != nil {
		a.err = err
	} else {
		// Show saved path in footer — register as already-finished task
		a.registerRunningTask("key-saved", "Key saved to "+savedPath)
		if cmd := a.finishTask("key-saved", nil); cmd != nil {
			cmds = append(cmds, cmd)
		}
		// Clear pending key from view
		if a.serviceAccountDetailsView != nil {
			a.serviceAccountDetailsView.ClearPendingKey()
		}
	}
	return tea.Batch(cmds...)
}

// writeKeyFile saves key JSON bytes to CWD with conflict avoidance.
// Returns the full path of the saved file.
func writeKeyFile(keyID string, data []byte) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	filename := filepath.Join(cwd, keyID+".json")

	// Avoid overwriting existing files
	if _, err := os.Stat(filename); err == nil {
		found := false
		for i := 1; i < 1000; i++ {
			candidate := filepath.Join(cwd, fmt.Sprintf("%s (%d).json", keyID, i))
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				filename = candidate
				found = true
				break
			}
		}
		if !found {
			return "", errKeyFilenameExhausted
		}
	}

	if err := os.WriteFile(filename, data, 0600); err != nil {
		return "", err
	}
	return filename, nil
}

// --- IAM: Policy editing ---

// handleAddIAMBinding adds a member to a role binding via async GCP call
func (a *App) handleAddIAMBinding(msg views.AddIAMBindingMsg) tea.Cmd {
	iamClient := a.getIAMClient()
	if iamClient == nil {
		return nil
	}

	a.registerRunningTask("iam-policy-update", "Adding IAM binding...")
	return func() tea.Msg {
		policy, err := iamClient.AddMemberToRole(gocontext.Background(), msg.ProjectID, msg.Role, msg.ConditionTitle, msg.Member)
		return views.IAMPolicyUpdateResultMsg{
			Action: "add_binding",
			Error:  err,
			Policy: policy,
		}
	}
}

// handleRemoveIAMBinding removes a member from a role binding via async GCP call
func (a *App) handleRemoveIAMBinding(msg views.RemoveIAMBindingMsg) tea.Cmd {
	iamClient := a.getIAMClient()
	if iamClient == nil {
		return nil
	}

	a.registerRunningTask("iam-policy-update", "Removing IAM binding...")
	return func() tea.Msg {
		policy, err := iamClient.RemoveMemberFromRole(gocontext.Background(), msg.ProjectID, msg.Role, msg.ConditionTitle, msg.Member)
		return views.IAMPolicyUpdateResultMsg{
			Action: "remove_binding",
			Error:  err,
			Policy: policy,
		}
	}
}

// handleIAMPolicyUpdateResult processes the result of an IAM policy update
func (a *App) handleIAMPolicyUpdateResult(msg views.IAMPolicyUpdateResultMsg) tea.Cmd {
	cmd := a.finishTask("iam-policy-update", msg.Error)

	if msg.Error != nil {
		a.err = msg.Error
		if a.currentView == ViewIAMPolicy && a.iamPolicyView != nil {
			a.iamPolicyView.SetError(msg.Error)
		}
		return cmd
	}

	// Update the view with the new policy
	if a.currentView == ViewIAMPolicy && a.iamPolicyView != nil && msg.Policy != nil {
		a.iamPolicyView.UpdatePolicy(msg.Policy)
	}
	return cmd
}

// getIAMClient returns an IAM client from existing views, or nil
func (a *App) getIAMClient() *gcp.IAMClient {
	if a.iamPolicyView != nil {
		return a.iamPolicyView.GetIAMClient()
	}
	if a.serviceAccountsView != nil {
		return a.serviceAccountsView.GetIAMClient()
	}
	return nil
}

// --- IAM: Custom Roles ---

// handleCustomRoleSelected navigates to custom role details view
//
//nolint:gocritic // hugeParam: message struct passed by value
func (a *App) handleCustomRoleSelected(msg views.CustomRoleSelectedMsg) tea.Cmd {
	role := msg.Role
	a.selectedCustomRole = &role

	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewCustomRoleDetails

	// Reuse IAM client from list view
	var iamClient *gcp.IAMClient
	if a.customRolesView != nil {
		iamClient = a.customRolesView.GetIAMClient()
	}

	a.customRoleDetailsView = views.NewCustomRoleDetailsView(
		a.selectedProject.ID,
		role.RoleID,
		iamClient,
	)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.customRoleDetailsView.Init()
}

// handleCloudRunServiceSelected navigates to Cloud Run service details
//
//nolint:gocritic // hugeParam: message struct passed by value
func (a *App) handleCloudRunServiceSelected(msg views.CloudRunServiceSelectedMsg) tea.Cmd {
	svc := msg.Service
	a.selectedCloudRunService = &svc
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewCloudRunServiceDetails

	// Share Cloud Run client from list view
	var runClient *gcp.CloudRunClient
	if a.cloudRunServicesView != nil {
		runClient = a.cloudRunServicesView.GetCloudRunClient()
	}

	a.cloudRunServiceDetailsView = views.NewCloudRunServiceDetailsView(
		a.selectedProject.ID,
		svc.Name,
		svc.FullName,
		runClient,
		a.gcpClient,
	)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.cloudRunServiceDetailsView.Init()
}

// handleDeleteCloudRunServiceConfirmed processes confirmed Cloud Run service deletion
func (a *App) handleDeleteCloudRunServiceConfirmed(msg views.DeleteCloudRunServiceConfirmedMsg) tea.Cmd {
	var runClient *gcp.CloudRunClient
	if a.cloudRunServicesView != nil {
		runClient = a.cloudRunServicesView.GetCloudRunClient()
	} else if a.cloudRunServiceDetailsView != nil {
		runClient = a.cloudRunServiceDetailsView.GetCloudRunClient()
	}

	if runClient == nil {
		return nil
	}

	fullName := msg.FullName
	name := msg.Name

	taskID := "cloudrun-delete-" + name
	taskCmd := a.startTask(context.Task{
		ID:          taskID,
		Description: "Deleting " + name + "...",
	})

	apiCmd := func() tea.Msg {
		err := runClient.DeleteService(gocontext.Background(), fullName)
		return views.CloudRunServiceActionResultMsg{
			Name:    name,
			Action:  "delete",
			Success: err == nil,
			Error:   err,
		}
	}

	return tea.Batch(taskCmd, apiCmd)
}

// handleCloudRunServiceActionResult processes the result of a Cloud Run service action
func (a *App) handleCloudRunServiceActionResult(msg views.CloudRunServiceActionResultMsg) tea.Cmd {
	// Clear the progress task
	taskID := "cloudrun-" + msg.Action + "-" + msg.Name
	delete(a.ctx.Tasks, taskID)

	if msg.Error != nil {
		a.err = msg.Error
		// Propagate error to detail view if active
		if a.currentView == ViewCloudRunServiceDetails && a.cloudRunServiceDetailsView != nil {
			a.cloudRunServiceDetailsView.SetError(msg.Error)
		}
		return nil
	}

	// On successful delete, navigate back to list and refresh
	if msg.Action == "delete" {
		if a.currentView == ViewCloudRunServiceDetails {
			if len(a.viewStack) > 0 {
				lastViewIndex := len(a.viewStack) - 1
				a.currentView = a.viewStack[lastViewIndex]
				a.viewStack = a.viewStack[:lastViewIndex]
			}

			if a.cloudRunServiceDetailsView != nil {
				a.cloudRunServiceDetailsView.Close()
			}
			a.cloudRunServiceDetailsView = nil
			a.selectedCloudRunService = nil
			a.updateSidebarActiveView()
		}

		// Refresh Cloud Run services list
		if a.cloudRunServicesView != nil {
			return a.cloudRunServicesView.Init()
		}
	}

	// On successful traffic update, refresh details view
	if msg.Action == "update_traffic" {
		if a.cloudRunServiceDetailsView != nil {
			return a.cloudRunServiceDetailsView.Init()
		}
	}

	return nil
}

// handleCloudRunTrafficUpdate processes a Cloud Run traffic split update
func (a *App) handleCloudRunTrafficUpdate(msg views.CloudRunTrafficUpdateMsg) tea.Cmd {
	var runClient *gcp.CloudRunClient
	if a.cloudRunServicesView != nil {
		runClient = a.cloudRunServicesView.GetCloudRunClient()
	} else if a.cloudRunServiceDetailsView != nil {
		runClient = a.cloudRunServiceDetailsView.GetCloudRunClient()
	}

	if runClient == nil {
		return nil
	}

	fullName := msg.FullName
	targets := msg.Targets

	// Extract short name for task display
	name := fullName
	if a.cloudRunServiceDetailsView != nil {
		name = a.cloudRunServiceDetailsView.GetServiceName()
	}

	taskID := "cloudrun-update_traffic-" + name
	taskCmd := a.startTask(context.Task{
		ID:          taskID,
		Description: "Updating traffic for " + name + "...",
	})

	apiCmd := func() tea.Msg {
		err := runClient.UpdateTraffic(gocontext.Background(), fullName, targets)
		return views.CloudRunServiceActionResultMsg{
			Name:    name,
			Action:  "update_traffic",
			Success: err == nil,
			Error:   err,
		}
	}

	return tea.Batch(taskCmd, apiCmd)
}

func (a *App) handleCloudRunEditRequest(msg views.CloudRunEditRequestMsg) tea.Cmd {
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewCloudRunServiceEdit

	var runClient *gcp.CloudRunClient
	if a.cloudRunServiceDetailsView != nil {
		runClient = a.cloudRunServiceDetailsView.GetCloudRunClient()
	}

	a.cloudRunServiceEditView = views.NewCloudRunEditView(
		msg.ProjectID, msg.ServiceName, msg.FullName, runClient, false,
	)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.cloudRunServiceEditView.Init()
}

func (a *App) handleCloudRunCreateRequest(msg views.CloudRunCreateRequestMsg) tea.Cmd {
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewCloudRunServiceEdit

	var runClient *gcp.CloudRunClient
	if a.cloudRunServicesView != nil {
		runClient = a.cloudRunServicesView.GetCloudRunClient()
	}

	a.cloudRunServiceEditView = views.NewCloudRunEditView(
		msg.ProjectID, "", "", runClient, true,
	)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.cloudRunServiceEditView.Init()
}

func (a *App) handleCloudRunEditResult(msg views.CloudRunEditResultMsg) tea.Cmd {
	if msg.Error != nil {
		a.err = msg.Error
		if a.cloudRunServiceEditView != nil {
			a.cloudRunServiceEditView.SetError(msg.Error)
		}
		return nil
	}

	// Pop back to previous view and refresh
	if len(a.viewStack) > 0 {
		a.currentView = a.viewStack[len(a.viewStack)-1]
		a.viewStack = a.viewStack[:len(a.viewStack)-1]
	}
	a.cloudRunServiceEditView = nil
	a.updateSidebarActiveView()

	if a.currentView == ViewCloudRunServiceDetails && a.cloudRunServiceDetailsView != nil {
		return a.cloudRunServiceDetailsView.Init()
	}
	if a.currentView == ViewCloudRunServices && a.cloudRunServicesView != nil {
		return a.cloudRunServicesView.Init()
	}
	return nil
}

func (a *App) handleCloudRunEditCanceled() {
	if len(a.viewStack) > 0 {
		a.currentView = a.viewStack[len(a.viewStack)-1]
		a.viewStack = a.viewStack[:len(a.viewStack)-1]
	}
	a.cloudRunServiceEditView = nil
	a.updateSidebarActiveView()
}

// --- Instance Create/Edit handlers ---

// handleInstanceCreateRequest opens the instance creation form
func (a *App) handleInstanceCreateRequest(msg views.InstanceCreateRequestMsg) tea.Cmd {
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewInstanceCreate

	// Reuse the compute client from the instances list view
	var computeClient *gcp.ComputeClient
	if a.instancesView != nil {
		computeClient = a.instancesView.GetComputeClient()
	}

	a.instanceCreateView = views.NewInstanceCreateView(msg.ProjectID, computeClient)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.instanceCreateView.Init()
}

// handleCreateInstance performs the GCP API call to create a VM instance
func (a *App) handleCreateInstance(msg views.CreateInstanceMsg) tea.Cmd {
	a.registerRunningTask("instance-create", "Creating instance "+msg.Config.Name+"...")

	var computeClient *gcp.ComputeClient
	if a.instanceCreateView != nil {
		computeClient = a.instanceCreateView.GetComputeClient()
	}

	projectID := ""
	if a.selectedProject != nil {
		projectID = a.selectedProject.ID
	}

	config := msg.Config
	return func() tea.Msg {
		if computeClient == nil {
			return views.InstanceCreateResultMsg{
				Name:  config.Name,
				Error: uierrors.ErrClientNotInitialized,
			}
		}
		err := computeClient.CreateInstance(gocontext.Background(), projectID, config)
		if err != nil {
			return views.InstanceCreateResultMsg{
				Name:  config.Name,
				Error: err,
			}
		}
		return views.InstanceCreateResultMsg{
			Name:    config.Name,
			Success: true,
		}
	}
}

func (a *App) handleInstanceCreateResult(msg views.InstanceCreateResultMsg) tea.Cmd {
	cmd := a.finishTask("instance-create", msg.Error)

	if msg.Error != nil {
		a.err = msg.Error
		// Propagate error back to the view so it exits saving state
		if a.instanceCreateView != nil {
			a.instanceCreateView.SetError(msg.Error)
		}
		return cmd
	}

	// Pop back to instances list and refresh
	if len(a.viewStack) > 0 {
		a.currentView = a.viewStack[len(a.viewStack)-1]
		a.viewStack = a.viewStack[:len(a.viewStack)-1]
	}
	a.instanceCreateView = nil
	a.updateSidebarActiveView()

	if a.currentView == ViewInstances && a.instancesView != nil {
		return tea.Batch(cmd, a.instancesView.Init())
	}
	return cmd
}

func (a *App) handleInstanceCreateCanceled() {
	if len(a.viewStack) > 0 {
		a.currentView = a.viewStack[len(a.viewStack)-1]
		a.viewStack = a.viewStack[:len(a.viewStack)-1]
	}
	a.instanceCreateView = nil
	a.updateSidebarActiveView()
}

// handleInstanceConfigEditRequest opens the instance config edit form
func (a *App) handleInstanceConfigEditRequest(msg views.InstanceConfigEditRequestMsg) tea.Cmd {
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewInstanceConfigEdit

	// Reuse compute client from the instance details or instances list view
	var computeClient *gcp.ComputeClient
	if a.instanceDetailsView != nil {
		computeClient = a.instanceDetailsView.GetComputeClient()
	} else if a.instancesView != nil {
		computeClient = a.instancesView.GetComputeClient()
	}

	a.instanceConfigEditView = views.NewInstanceConfigEditView(
		msg.ProjectID, msg.InstanceName, msg.Zone, computeClient,
	)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.instanceConfigEditView.Init()
}

// handleInstanceConfigEditSubmit applies the config changes via GCP API calls.
// Each change (machine type, disk resize) is applied independently because they
// are separate API operations — partial failures are collected and reported.
//
//nolint:gocognit // Multiple independent API calls with partial error collection
func (a *App) handleInstanceConfigEditSubmit(msg views.InstanceConfigEditSubmitMsg) tea.Cmd {
	a.registerRunningTask("instance-config-edit", "Updating instance "+msg.InstanceName+"...")

	var computeClient *gcp.ComputeClient
	if a.instanceConfigEditView != nil {
		computeClient = a.instanceConfigEditView.GetComputeClient()
	}

	return func() tea.Msg {
		if computeClient == nil {
			return views.InstanceConfigEditResultMsg{
				Action: "config_edit",
				Error:  uierrors.ErrClientNotInitialized,
			}
		}

		ctx := gocontext.Background()
		var partialErrors []string

		for _, change := range msg.Changes {
			switch change.Field {
			case "machine_type":
				err := computeClient.SetMachineType(ctx, msg.ProjectID, msg.Zone, msg.InstanceName, change.NewValue)
				if err != nil {
					partialErrors = append(partialErrors, fmt.Sprintf("Machine type: %v", err))
				}
			case "disk_size":
				sizeGB, parseErr := strconv.ParseInt(change.NewValue, 10, 64)
				if parseErr != nil {
					partialErrors = append(partialErrors, fmt.Sprintf("Disk resize: invalid size %q", change.NewValue))
					continue
				}
				// Use actual boot disk name from instance details
				err := computeClient.ResizeBootDisk(ctx, msg.ProjectID, msg.Zone, msg.BootDiskName, sizeGB)
				if err != nil {
					partialErrors = append(partialErrors, fmt.Sprintf("Disk resize: %v", err))
				}
			}
		}

		if len(partialErrors) > 0 {
			return views.InstanceConfigEditResultMsg{
				Action:        "config_edit",
				Error:         fmt.Errorf("%w: %s", uierrors.ErrPartialConfigEditFailed, strings.Join(partialErrors, "; ")),
				PartialErrors: partialErrors,
			}
		}

		return views.InstanceConfigEditResultMsg{
			Action:  "config_edit",
			Success: true,
		}
	}
}

func (a *App) handleInstanceConfigEditResult(msg views.InstanceConfigEditResultMsg) tea.Cmd {
	cmd := a.finishTask("instance-config-edit", msg.Error)

	if msg.Error != nil {
		a.err = msg.Error
		if a.instanceConfigEditView != nil {
			a.instanceConfigEditView.SetError(msg.Error)
		}
		return cmd
	}

	// Pop back and refresh the previous view
	if len(a.viewStack) > 0 {
		a.currentView = a.viewStack[len(a.viewStack)-1]
		a.viewStack = a.viewStack[:len(a.viewStack)-1]
	}
	a.instanceConfigEditView = nil
	a.updateSidebarActiveView()

	// Refresh the parent view to show updated config
	if a.currentView == ViewInstanceDetails && a.instanceDetailsView != nil {
		return tea.Batch(cmd, a.instanceDetailsView.Init())
	}
	if a.currentView == ViewInstances && a.instancesView != nil {
		return tea.Batch(cmd, a.instancesView.Init())
	}
	return cmd
}

func (a *App) handleInstanceConfigEditCanceled() {
	if len(a.viewStack) > 0 {
		a.currentView = a.viewStack[len(a.viewStack)-1]
		a.viewStack = a.viewStack[:len(a.viewStack)-1]
	}
	a.instanceConfigEditView = nil
	a.updateSidebarActiveView()
}
