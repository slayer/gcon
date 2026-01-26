package ui

import (
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
		// Already on projects view, just close modal
		return nil

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

// handleInstanceEditCancelled processes cancelled instance edit
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
