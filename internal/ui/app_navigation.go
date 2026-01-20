package ui

import (
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/projectdialog"
	"github.com/slayer/gcon/internal/ui/components/sidebar"
	"github.com/slayer/gcon/internal/ui/views"
)

// handleMouseEvent processes mouse events and routes them to appropriate components.
// Uses region-based click handling for better maintainability and correctness.
func (a *App) handleMouseEvent(msg tea.MouseMsg) tea.Cmd {
	// Fast path: ignore motion events entirely - they're too frequent and cause lag
	if msg.Action == tea.MouseActionMotion {
		return nil
	}

	// Get header height to adjust Y coordinate
	_, headerHeight := a.layout.HeaderSize()
	contentHeight := a.layout.ContentHeight()
	footerY := headerHeight + contentHeight

	// Check if click is in header area
	if msg.Y < headerHeight {
		return nil
	}

	// Check if click is in footer area (for project switcher)
	if msg.Y >= footerY && msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		// Check if click is on the project section (right side of footer)
		// Project is typically rendered in the rightmost section
		// We'll check if it's in the right 1/3 of the screen as a rough approximation
		if a.selectedProject != nil && msg.X > a.width*2/3 {
			return a.openProjectDialog(true)
		}
		return nil
	}

	// Calculate sidebar offset
	sidebarWidth := 0
	if a.sidebarActive() {
		sidebarWidth = a.sidebar.Width()
	}

	// Handle mouse clicks using region-based system
	// Only use regions for left-click press events to avoid performance overhead
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		// Check sidebar first (if active and click is in sidebar area)
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

// handleProjectDialogSelected handles project selection from the modal dialog.
// This resets all view state and navigates to the instances view with the new project.
func (a *App) handleProjectDialogSelected(msg projectdialog.ProjectDialogSelectedMsg) tea.Cmd {
	a.showProjectDialog = false
	a.projectDialog.Reset()

	project := msg.Project
	return a.switchToProject(project)
}

// switchToProject changes the current project, resetting all views.
// Used by both the project dialog and any other project switching mechanisms.
func (a *App) switchToProject(project gcp.Project) tea.Cmd {
	// Reset all views - they'll be recreated with the new project ID
	a.resetAllViews()

	// Set the new project
	a.selectedProject = &project

	// Track recent project access
	a.recentTracker.Track("project", project.ID, project.Name)

	// Navigate to instances view with sidebar
	a.currentView = ViewInstances
	a.viewStack = []ViewType{} // Clear navigation stack
	a.instancesView = views.NewInstancesView(project.ID)
	a.focusedPanel = FocusContent
	a.updateSidebarActiveView()
	a.updateViewSizes()
	a.syncContext()
	return a.instancesView.Init()
}

// resetAllViews clears all view instances to force recreation with new context.
// Called when switching projects to ensure views reload with new project data.
func (a *App) resetAllViews() {
	// Clean up views that need explicit cleanup
	if a.instanceDetailsView != nil {
		a.instanceDetailsView.Close()
	}
	if a.bucketsView != nil {
		_ = a.bucketsView.Close()
	}

	// Nil out all view references
	a.instancesView = nil
	a.instanceDetailsView = nil
	a.metadataView = nil
	a.projectMetadataView = nil
	a.disksView = nil
	a.diskDetailsView = nil
	a.snapshotsView = nil
	a.snapshotDetailsView = nil
	a.imagesView = nil
	a.imageDetailsView = nil
	a.bucketsView = nil
	a.objectsView = nil
	a.objectDetailsView = nil

	// Clear selected items
	a.selectedInstance = nil
	a.selectedDisk = nil
	a.selectedSnapshot = nil
	a.selectedImage = nil
	a.selectedBucket = nil
	a.selectedObject = nil
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
