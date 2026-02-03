package ui

import (
	gocontext "context"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/sidebar"
	"github.com/slayer/gcon/internal/ui/views"
)

// handleMouseEvent processes mouse events and routes them to appropriate components.
// Uses region-based click handling for better maintainability and correctness.
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
		}

		// Check sidebar (if active and click is in sidebar area)
		if a.sidebarActive() && msg.X < sidebarWidth {
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
		} else {
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
//nolint:gocognit // Sidebar navigation routing - complexity 35
func (a *App) handleSidebarNavigation(msg sidebar.NavigateMsg) tea.Cmd {
	var cmd tea.Cmd

	// Map sidebar ViewType to app ViewType and navigate
	switch msg.ViewType {
	case sidebar.ViewInstances:
		if a.currentView != ViewInstances && a.currentView != ViewInstanceDetails {
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
		a.currentView = ViewNetworks
		// Placeholder - view not implemented yet
	case sidebar.ViewFirewall:
		a.currentView = ViewFirewall
		// Placeholder - view not implemented yet
	}

	a.updateSidebarActiveView()
	// Switch focus back to content after navigation
	a.focusedPanel = FocusContent
	a.sidebar.SetFocused(false)

	return cmd
}

// updateSidebarActiveView sets the active view highlight in sidebar based on current view
func (a *App) updateSidebarActiveView() {
	switch a.currentView {
	case ViewInstances, ViewInstanceDetails:
		a.sidebar.SetActiveView(sidebar.ViewInstances)
	case ViewDisks, ViewDiskDetails:
		a.sidebar.SetActiveView(sidebar.ViewDisks)
	case ViewSnapshots, ViewSnapshotDetails:
		a.sidebar.SetActiveView(sidebar.ViewSnapshots)
	case ViewImages, ViewImageDetails:
		a.sidebar.SetActiveView(sidebar.ViewImages)
	case ViewBuckets, ViewObjects, ViewObjectDetails:
		a.sidebar.SetActiveView(sidebar.ViewBuckets)
	case ViewNetworks:
		a.sidebar.SetActiveView(sidebar.ViewNetworks)
	case ViewFirewall:
		a.sidebar.SetActiveView(sidebar.ViewFirewall)
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
	a.bucketCreateView = nil
	a.snapshotCreateView = nil
	a.imageCreateView = nil
	a.diskCreateView = nil
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
}

// reloadCurrentView recreates the current view with the new project ID
func (a *App) reloadCurrentView(projectID string) tea.Cmd {
	switch a.currentView {
	case ViewInstances, ViewInstanceDetails:
		// Return to instances list
		a.currentView = ViewInstances
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
