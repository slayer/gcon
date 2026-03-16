package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// renderHeader creates the header with breadcrumb navigation
//
//nolint:gocognit // Header rendering with breadcrumbs - complexity 35
func (a *App) renderHeader() string {
	// Update header state based on current view
	if a.selectedProject != nil {
		a.header.SetProject(a.selectedProject.ID)
	} else {
		a.header.SetProject("")
	}

	// Determine category
	category := ""
	if cat := a.sidebar.GetCurrentCategory(); cat != "" {
		category = cat
	} else {
		// Determine category based on current view
		switch a.currentView {
		case ViewInstances, ViewInstanceDetails, ViewInstanceEditor, ViewInstanceCreate, ViewInstanceConfigEdit, ViewDisks, ViewDiskDetails, ViewSnapshots, ViewSnapshotDetails, ViewImages, ViewImageDetails:
			category = "Compute Engine"
		case ViewBuckets, ViewObjects, ViewObjectDetails:
			category = "Cloud Storage"
		case ViewNetworks, ViewNetworkDetails, ViewFirewall, ViewFirewallDetails:
			category = "VPC Network"
		case ViewSQLInstances, ViewSQLInstanceDetails:
			category = "Databases"
		case ViewServiceAccounts, ViewServiceAccountDetails, ViewServiceAccountCreate, ViewIAMPolicy, ViewCustomRoles, ViewCustomRoleDetails:
			category = "IAM & Admin"
		case ViewCloudRunServices, ViewCloudRunServiceDetails, ViewCloudRunServiceEdit:
			category = "Cloud Run"
		}
	}
	a.header.SetCategory(category)

	// Build resources list based on current view
	// This creates hierarchical breadcrumbs showing navigation context
	resources := []string{}
	switch a.currentView {
	case ViewInstanceDetails:
		if a.selectedInstance != nil {
			resources = append(resources, a.selectedInstance.Name)
		}
	case ViewDiskDetails:
		// Show parent instance if we came from instance details
		if a.instanceDetailsView != nil && len(a.viewStack) > 0 {
			lastView := a.viewStack[len(a.viewStack)-1]
			if lastView == ViewInstanceDetails {
				// Show: instance-name → disk-name
				resources = append(resources, a.instanceDetailsView.GetInstanceName())
			}
		}
		// Always add disk name from the view (whether navigated from instance or directly)
		if a.diskDetailsView != nil {
			resources = append(resources, a.diskDetailsView.GetDiskName())
		}
	case ViewSnapshotDetails:
		// Show parent disk if we came from disk details
		if a.diskDetailsView != nil && len(a.viewStack) > 0 {
			lastView := a.viewStack[len(a.viewStack)-1]
			if lastView == ViewDiskDetails {
				// Show: disk-name → snapshot-name
				resources = append(resources, a.diskDetailsView.GetDiskName())
			}
		}
		// Always add snapshot name from the view
		if a.snapshotDetailsView != nil {
			resources = append(resources, a.snapshotDetailsView.GetSnapshotName())
		}
	case ViewImageDetails:
		if a.selectedImage != nil {
			resources = append(resources, a.selectedImage.Name)
		}
	case ViewObjects:
		if a.objectsView != nil {
			resources = append(resources, a.objectsView.GetBucketName())
			if path := a.objectsView.GetCurrentPath(); path != "" {
				resources = append(resources, path)
			}
		}
	case ViewObjectDetails:
		// Show bucket name and object name when viewing object details
		if a.objectDetailsView != nil {
			resources = append(resources, a.objectDetailsView.GetBucketName())
		}
	case ViewInstanceEditor:
		// Show instance name and "Edit Labels" when editing
		if a.selectedInstance != nil {
			resources = append(resources, a.selectedInstance.Name, "Edit Labels")
		}
	case ViewNetworkDetails:
		if a.selectedNetwork != nil {
			resources = append(resources, a.selectedNetwork.Name)
		}
	case ViewFirewallDetails:
		if a.selectedFirewall != nil {
			resources = append(resources, a.selectedFirewall.Name)
		}
	case ViewSQLInstanceDetails:
		if a.selectedSQLInstance != nil {
			resources = append(resources, a.selectedSQLInstance.Name)
		}
	case ViewServiceAccountDetails:
		if a.serviceAccountDetailsView != nil {
			resources = append(resources, a.serviceAccountDetailsView.GetEmail())
		}
	case ViewServiceAccountCreate:
		resources = append(resources, "Create")
	case ViewCustomRoleDetails:
		if a.customRoleDetailsView != nil {
			resources = append(resources, a.customRoleDetailsView.GetRoleID())
		}
	case ViewCloudRunServiceDetails:
		if a.cloudRunServiceDetailsView != nil {
			resources = append(resources, a.cloudRunServiceDetailsView.GetServiceName())
		}
	case ViewCloudRunServiceEdit:
		if a.cloudRunServiceEditView != nil && a.cloudRunServiceEditView.IsCreate() {
			resources = append(resources, "Create")
		} else {
			if a.cloudRunServiceDetailsView != nil {
				resources = append(resources, a.cloudRunServiceDetailsView.GetServiceName())
			}
			resources = append(resources, "Edit")
		}
	case ViewInstanceCreate:
		resources = append(resources, "Create Instance")
	case ViewInstanceConfigEdit:
		if a.instanceConfigEditView != nil {
			resources = append(resources, a.instanceConfigEditView.GetInstanceName(), "Edit Configuration")
		}
	}
	a.header.SetResources(resources)

	return a.header.View()
}

// renderCurrentView renders the content area based on current view
//
//nolint:gocognit // View routing and rendering - complexity 32
func (a *App) renderCurrentView() string {
	// Show loading message while fetching initial project from config
	if a.loadingInitialProject {
		return "Loading project " + a.initialProjectID + "..."
	}

	switch a.currentView {
	case ViewProjects:
		if a.projectView != nil {
			return a.projectView.View()
		}
	case ViewInstances:
		if a.instancesView != nil {
			return a.instancesView.View()
		}
	case ViewInstanceDetails:
		if a.instanceDetailsView != nil {
			return a.instanceDetailsView.View()
		}
	case ViewMetadata:
		if a.metadataView != nil {
			return a.metadataView.View()
		}
	case ViewProjectMetadata:
		if a.projectMetadataView != nil {
			return a.projectMetadataView.View()
		}
	case ViewDisks:
		if a.disksView != nil {
			return a.disksView.View()
		}
	case ViewDiskDetails:
		if a.diskDetailsView != nil {
			return a.diskDetailsView.View()
		}
	case ViewSnapshots:
		if a.snapshotsView != nil {
			return a.snapshotsView.View()
		}
	case ViewSnapshotDetails:
		if a.snapshotDetailsView != nil {
			return a.snapshotDetailsView.View()
		}
	case ViewImages:
		if a.imagesView != nil {
			return a.imagesView.View()
		}
	case ViewImageDetails:
		if a.imageDetailsView != nil {
			return a.imageDetailsView.View()
		}
	case ViewBuckets:
		if a.bucketsView != nil {
			return a.bucketsView.View()
		}
	case ViewObjects:
		if a.objectsView != nil {
			return a.objectsView.View()
		}
	case ViewObjectDetails:
		if a.objectDetailsView != nil {
			return a.objectDetailsView.View()
		}
	case ViewInstanceEditor:
		if a.instanceEditorView != nil {
			return a.instanceEditorView.View()
		}
	case ViewBucketCreate:
		if a.bucketCreateView != nil {
			return a.bucketCreateView.View()
		}
	case ViewSnapshotCreate:
		if a.snapshotCreateView != nil {
			return a.snapshotCreateView.View()
		}
	case ViewImageCreate:
		if a.imageCreateView != nil {
			return a.imageCreateView.View()
		}
	case ViewDiskCreate:
		if a.diskCreateView != nil {
			return a.diskCreateView.View()
		}
	case ViewNetworks:
		if a.networksView != nil {
			return a.networksView.View()
		}
	case ViewNetworkDetails:
		if a.networkDetailsView != nil {
			return a.networkDetailsView.View()
		}
	case ViewFirewall:
		if a.firewallsView != nil {
			return a.firewallsView.View()
		}
	case ViewFirewallDetails:
		if a.firewallDetailsView != nil {
			return a.firewallDetailsView.View()
		}
	case ViewSQLInstances:
		if a.sqlInstancesView != nil {
			return a.sqlInstancesView.View()
		}
	case ViewSQLInstanceDetails:
		if a.sqlInstanceDetailsView != nil {
			return a.sqlInstanceDetailsView.View()
		}
	case ViewServiceAccounts:
		if a.serviceAccountsView != nil {
			return a.serviceAccountsView.View()
		}
	case ViewServiceAccountDetails:
		if a.serviceAccountDetailsView != nil {
			return a.serviceAccountDetailsView.View()
		}
	case ViewServiceAccountCreate:
		if a.serviceAccountCreateView != nil {
			return a.serviceAccountCreateView.View()
		}
	case ViewIAMPolicy:
		if a.iamPolicyView != nil {
			return a.iamPolicyView.View()
		}
	case ViewCustomRoles:
		if a.customRolesView != nil {
			return a.customRolesView.View()
		}
	case ViewCustomRoleDetails:
		if a.customRoleDetailsView != nil {
			return a.customRoleDetailsView.View()
		}
	case ViewCloudRunServices:
		if a.cloudRunServicesView != nil {
			return a.cloudRunServicesView.View()
		}
	case ViewCloudRunServiceDetails:
		if a.cloudRunServiceDetailsView != nil {
			return a.cloudRunServiceDetailsView.View()
		}
	case ViewCloudRunServiceEdit:
		if a.cloudRunServiceEditView != nil {
			return a.cloudRunServiceEditView.View()
		}
	case ViewInstanceCreate:
		if a.instanceCreateView != nil {
			return a.instanceCreateView.View()
		}
	case ViewInstanceConfigEdit:
		if a.instanceConfigEditView != nil {
			return a.instanceConfigEditView.View()
		}
	case ViewFormDemo:
		if a.formDemoView != nil {
			return a.formDemoView.View()
		}
	}
	return "View not implemented"
}

// renderPlaceholder renders a placeholder for unimplemented views
func (a *App) renderPlaceholder(name string) string {
	return a.styles.Muted.Render("\n  " + name + " view - not implemented yet\n\n  Use sidebar to navigate to VM instances.")
}

// renderWithSidebar creates the two-panel layout with guaranteed matching heights
func (a *App) renderWithSidebar() string {
	// Sidebar already has its own styling (border, width, height) applied internally
	sidebarView := a.sidebar.View()
	contentView := a.renderCurrentView()

	// Calculate safe width accounting for emojis in BOTH sidebar and content.
	// JoinHorizontal combines lines side-by-side, so emojis from both views
	// end up on the same terminal line. We need to account for the maximum
	// combined emoji count on any single line.
	sidebarMaxEmojis := maxLineEmojiCount(sidebarView)
	contentMaxEmojis := maxLineEmojiCount(contentView)
	totalMaxEmojis := sidebarMaxEmojis + contentMaxEmojis

	// Debug: show emoji count per sidebar line
	sidebarLines := strings.Split(sidebarView, "\n")
	for i, line := range sidebarLines {
		ec := countWideEmojis(line)
		if ec > 0 {
			debugLog("  sidebar line %d: %d emojis, width=%d", i, ec, lipgloss.Width(line))
		}
	}

	mainWidth := a.layout.ContentWidth() - totalMaxEmojis
	if mainWidth < 10 {
		mainWidth = 10
	}
	// Use MaxWidth to constrain content that may be wider than available space.
	// Width() sets minimum width, MaxWidth() sets maximum and truncates/wraps.
	mainStyle := lipgloss.NewStyle().Width(mainWidth).MaxWidth(mainWidth)
	styledContent := mainStyle.Render(contentView)

	debugLog("renderWithSidebar: sidebar=%d, contentWidth=%d, mainWidth=%d, emojis=%d+%d",
		lipgloss.Width(sidebarView), a.layout.ContentWidth(), mainWidth,
		sidebarMaxEmojis, contentMaxEmojis)

	// Join horizontally - parent View() will enforce overall height
	result := lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebarView,
		styledContent,
	)

	// Debug: show width breakdown after join
	resultLines := strings.Split(result, "\n")
	expectedWidth := a.layout.ContentWidth()
	for i, line := range resultLines[:min(10, len(resultLines))] {
		lw := lipgloss.Width(line)
		tw := TerminalWidth(line)
		// Only log lines where width differs from expected layout width
		if lw != expectedWidth || tw > a.width {
			debugLog("  result line %d: lipgloss=%d, terminal=%d, emojis=%d", i, lw, tw, countWideEmojis(line))
		}
	}
	debugLog("renderWithSidebar: result maxLineWidth=%d", MaxLineWidth(result))
	return result
}

// renderWithCommandPalette overlays the command palette on top of the content
func (a *App) renderWithCommandPalette(background string) string {
	// Get the command palette view
	paletteView := a.commandPalette.View()

	// Split background into lines
	bgLines := strings.Split(background, "\n")

	// Split palette into lines
	paletteLines := strings.Split(paletteView, "\n")

	// Find max palette width for consistent positioning
	maxPaletteWidth := 0
	for _, line := range paletteLines {
		w := lipgloss.Width(line)
		if w > maxPaletteWidth {
			maxPaletteWidth = w
		}
	}

	// Calculate position (centered horizontally, 1/4 down vertically)
	leftPad := (a.width - maxPaletteWidth) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	topPad := a.height / 4
	if topPad < 2 {
		topPad = 2
	}

	// Overlay the palette on the background, preserving content on both sides
	result := make([]string, len(bgLines))
	copy(result, bgLines)

	// Right side always starts at the same position for alignment
	rightStart := leftPad + maxPaletteWidth

	for i, paletteLine := range paletteLines {
		bgIndex := topPad + i
		if bgIndex < len(result) {
			bgLine := result[bgIndex]
			bgWidth := lipgloss.Width(bgLine)

			// Build the overlayed line:
			// 1. Left part of background (truncated to leftPad width)
			// 2. Palette line (padded to maxPaletteWidth)
			// 3. Right part of background (from rightStart onwards)
			var newLine strings.Builder

			// Left part: truncate background to leftPad characters
			if leftPad > 0 {
				leftPart := truncateRight(bgLine, leftPad)
				newLine.WriteString(leftPart)
				// Pad if background was shorter than leftPad
				leftWidth := lipgloss.Width(leftPart)
				if leftWidth < leftPad {
					newLine.WriteString(strings.Repeat(" ", leftPad-leftWidth))
				}
			}

			// Middle: the palette line, padded to consistent width
			newLine.WriteString(paletteLine)
			paletteLineWidth := lipgloss.Width(paletteLine)
			if paletteLineWidth < maxPaletteWidth {
				newLine.WriteString(strings.Repeat(" ", maxPaletteWidth-paletteLineWidth))
			}

			// Right part: skip first rightStart characters of background
			if rightStart < bgWidth {
				rightPart := truncateLeft(bgLine, rightStart)
				newLine.WriteString(rightPart)
			}

			result[bgIndex] = newLine.String()
		}
	}

	return strings.Join(result, "\n")
}

// truncateRight keeps the first n visible columns of an ANSI string.
// Uses runewidth to correctly handle wide characters (e.g., CJK, some symbols).
func truncateRight(s string, n int) string {
	if n <= 0 {
		return ""
	}

	var result strings.Builder
	var visibleWidth int
	inEscape := false

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			result.WriteRune(r)
			continue
		}
		if inEscape {
			result.WriteRune(r)
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}

		rw := runewidth.RuneWidth(r)
		if visibleWidth+rw > n {
			break
		}
		result.WriteRune(r)
		visibleWidth += rw
	}

	return result.String()
}

// truncateLeft removes the first n visible columns from an ANSI string.
// Uses runewidth to correctly handle wide characters.
func truncateLeft(s string, n int) string {
	width := lipgloss.Width(s)
	if n >= width {
		return ""
	}

	var result strings.Builder
	var visibleWidth int
	inEscape := false

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			if visibleWidth >= n {
				result.WriteRune(r)
			}
			continue
		}
		if inEscape {
			if visibleWidth >= n {
				result.WriteRune(r)
			}
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}

		rw := runewidth.RuneWidth(r)
		if visibleWidth >= n {
			result.WriteRune(r)
		}
		visibleWidth += rw
	}

	return result.String()
}

// truncateToHeight truncates content to exactly maxLines by splitting on newlines.
// This is ANSI-safe because escape sequences don't span line boundaries.
func truncateToHeight(content string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}

	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}

	// Take only the first maxLines lines
	return strings.Join(lines[:maxLines], "\n")
}

// renderWithProjectSelector overlays the project selector modal on the background
func (a *App) renderWithProjectSelector(background string) string {
	// Get the project selector view
	selectorView := a.projectSelector.View()

	// Split background into lines
	bgLines := strings.Split(background, "\n")

	// Split selector into lines
	selectorLines := strings.Split(selectorView, "\n")

	// Find max selector width for consistent positioning
	maxSelectorWidth := 0
	for _, line := range selectorLines {
		w := lipgloss.Width(line)
		if w > maxSelectorWidth {
			maxSelectorWidth = w
		}
	}

	// Calculate position (centered horizontally and vertically)
	leftPad := (a.width - maxSelectorWidth) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	topPad := (a.height - len(selectorLines)) / 2
	if topPad < 2 {
		topPad = 2
	}

	// Overlay the selector on the background
	result := make([]string, len(bgLines))
	copy(result, bgLines)

	// Right side always starts at the same position for alignment
	rightStart := leftPad + maxSelectorWidth

	for i, selectorLine := range selectorLines {
		bgIndex := topPad + i
		if bgIndex < len(result) {
			bgLine := result[bgIndex]
			bgWidth := lipgloss.Width(bgLine)

			// Build the overlayed line:
			// 1. Left part of background (truncated to leftPad width)
			// 2. Selector line (padded to maxSelectorWidth)
			// 3. Right part of background (from rightStart onwards)
			var newLine strings.Builder

			// Left part: truncate background to leftPad characters
			if leftPad > 0 {
				leftPart := truncateRight(bgLine, leftPad)
				newLine.WriteString(leftPart)
				// Pad if background was shorter than leftPad
				leftWidth := lipgloss.Width(leftPart)
				if leftWidth < leftPad {
					newLine.WriteString(strings.Repeat(" ", leftPad-leftWidth))
				}
			}

			// Middle: the selector line, padded to consistent width
			newLine.WriteString(selectorLine)
			selectorLineWidth := lipgloss.Width(selectorLine)
			if selectorLineWidth < maxSelectorWidth {
				newLine.WriteString(strings.Repeat(" ", maxSelectorWidth-selectorLineWidth))
			}

			// Right part: skip first rightStart characters of background
			if rightStart < bgWidth {
				rightPart := truncateLeft(bgLine, rightStart)
				newLine.WriteString(rightPart)
			}

			result[bgIndex] = newLine.String()
		}
	}

	return strings.Join(result, "\n")
}
