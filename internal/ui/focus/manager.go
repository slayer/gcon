package focus

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Manager tracks which region has focus within a view and handles
// Tab key cycling between regions.
type Manager struct {
	regions     []Region
	activeIndex int
}

// NewManager creates a new focus manager with no regions.
// Use SetRegions to configure the available regions.
func NewManager() *Manager {
	return &Manager{
		regions:     nil,
		activeIndex: 0,
	}
}

// SetRegions configures the focusable regions for this manager.
// The first enabled region becomes active by default.
func (m *Manager) SetRegions(regions []Region) {
	m.regions = regions
	m.activeIndex = 0
	// Find first enabled region
	for i, r := range regions {
		if r.Enabled {
			m.activeIndex = i
			break
		}
	}
}

// Regions returns all configured regions.
func (m *Manager) Regions() []Region {
	return m.regions
}

// Active returns the currently focused region, or nil if no regions exist.
func (m *Manager) Active() *Region {
	if len(m.regions) == 0 {
		return nil
	}
	if m.activeIndex >= len(m.regions) {
		m.activeIndex = 0
	}
	return &m.regions[m.activeIndex]
}

// ActiveType returns the type of the currently focused region.
// Returns RegionViewport as a safe default if no regions exist.
func (m *Manager) ActiveType() RegionType {
	if active := m.Active(); active != nil {
		return active.Type
	}
	return RegionViewport
}

// ActiveID returns the ID of the currently focused region, or empty string.
func (m *Manager) ActiveID() string {
	if active := m.Active(); active != nil {
		return active.ID
	}
	return ""
}

// IsActive returns true if the region with the given ID has focus.
func (m *Manager) IsActive(id string) bool {
	return m.ActiveID() == id
}

// SetActive sets focus to the region with the given ID.
// Returns true if the region was found and enabled, false otherwise.
func (m *Manager) SetActive(id string) bool {
	for i, r := range m.regions {
		if r.ID == id && r.Enabled {
			m.activeIndex = i
			return true
		}
	}
	return false
}

// EnableRegion enables the region with the given ID.
// If no region is currently active, the enabled region becomes active.
func (m *Manager) EnableRegion(id string) {
	for i := range m.regions {
		if m.regions[i].ID == id {
			m.regions[i].Enabled = true
			// If no active enabled region, make this one active
			if !m.hasActiveEnabled() {
				m.activeIndex = i
			}
			return
		}
	}
}

// DisableRegion disables the region with the given ID.
// If this region was active, focus moves to the next enabled region.
func (m *Manager) DisableRegion(id string) {
	for i := range m.regions {
		if m.regions[i].ID == id {
			m.regions[i].Enabled = false
			// If this was active, move to next enabled
			if m.activeIndex == i {
				m.cycleNext()
			}
			return
		}
	}
}

// HandleKey processes Tab and Shift+Tab for cycling between regions.
// Returns a FocusChangedMsg if focus changed, or nil if the key wasn't handled.
func (m *Manager) HandleKey(msg tea.KeyMsg) tea.Msg {
	if len(m.regions) == 0 {
		return nil
	}

	switch msg.String() {
	case "tab":
		return m.cycleNext()
	case "shift+tab":
		return m.cyclePrev()
	}
	return nil
}

// cycleNext moves focus to the next enabled region.
// Returns FocusChangedMsg if focus changed.
func (m *Manager) cycleNext() tea.Msg {
	if len(m.regions) == 0 {
		return nil
	}

	from := m.Active()
	startIndex := m.activeIndex

	// Find next enabled region, wrapping around
	for i := range len(m.regions) {
		next := (startIndex + 1 + i) % len(m.regions)
		if m.regions[next].Enabled {
			m.activeIndex = next
			if m.activeIndex != startIndex {
				return FocusChangedMsg{
					FromRegion: from,
					ToRegion:   m.Active(),
				}
			}
			return nil
		}
	}
	return nil
}

// cyclePrev moves focus to the previous enabled region.
// Returns FocusChangedMsg if focus changed.
func (m *Manager) cyclePrev() tea.Msg {
	if len(m.regions) == 0 {
		return nil
	}

	from := m.Active()
	startIndex := m.activeIndex

	// Find previous enabled region, wrapping around
	for i := range len(m.regions) {
		prev := (startIndex - 1 - i + len(m.regions)) % len(m.regions)
		if m.regions[prev].Enabled {
			m.activeIndex = prev
			if m.activeIndex != startIndex {
				return FocusChangedMsg{
					FromRegion: from,
					ToRegion:   m.Active(),
				}
			}
			return nil
		}
	}
	return nil
}

// hasActiveEnabled returns true if the current active region is enabled.
func (m *Manager) hasActiveEnabled() bool {
	if len(m.regions) == 0 {
		return false
	}
	if m.activeIndex >= len(m.regions) {
		return false
	}
	return m.regions[m.activeIndex].Enabled
}
