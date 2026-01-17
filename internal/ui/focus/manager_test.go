package focus

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	assert.NotNil(t, m)
	assert.Empty(t, m.regions)
	assert.Equal(t, 0, m.activeIndex)
}

func TestSetRegions(t *testing.T) {
	m := NewManager()
	regions := []Region{
		NewRegion("tabs", RegionTabs, "Tabs"),
		NewRegion("viewport", RegionViewport, "Content"),
	}
	m.SetRegions(regions)

	assert.Len(t, m.Regions(), 2)
	assert.Equal(t, "tabs", m.ActiveID())
}

func TestSetRegionsWithDisabledFirst(t *testing.T) {
	m := NewManager()
	// First region is disabled, so second should become active
	regions := []Region{
		NewDisabledRegion("disabled", RegionLinks, "Links"),
		NewRegion("viewport", RegionViewport, "Content"),
	}
	m.SetRegions(regions)

	assert.Equal(t, "viewport", m.ActiveID())
}

func TestActive(t *testing.T) {
	m := NewManager()

	// No regions - should return nil
	assert.Nil(t, m.Active())

	// Add regions
	m.SetRegions([]Region{
		NewRegion("tabs", RegionTabs, "Tabs"),
	})

	active := m.Active()
	require.NotNil(t, active)
	assert.Equal(t, "tabs", active.ID)
	assert.Equal(t, RegionTabs, active.Type)
}

func TestActiveType(t *testing.T) {
	m := NewManager()

	// No regions - should return default
	assert.Equal(t, RegionViewport, m.ActiveType())

	// Add regions
	m.SetRegions([]Region{
		NewRegion("tabs", RegionTabs, "Tabs"),
	})
	assert.Equal(t, RegionTabs, m.ActiveType())
}

func TestIsActive(t *testing.T) {
	m := NewManager()
	m.SetRegions([]Region{
		NewRegion("tabs", RegionTabs, "Tabs"),
		NewRegion("viewport", RegionViewport, "Content"),
	})

	assert.True(t, m.IsActive("tabs"))
	assert.False(t, m.IsActive("viewport"))
}

func TestSetActive(t *testing.T) {
	m := NewManager()
	m.SetRegions([]Region{
		NewRegion("tabs", RegionTabs, "Tabs"),
		NewRegion("links", RegionLinks, "Links"),
		NewRegion("viewport", RegionViewport, "Content"),
	})

	// Set active to links
	ok := m.SetActive("links")
	assert.True(t, ok)
	assert.Equal(t, "links", m.ActiveID())

	// Try to set to non-existent
	ok = m.SetActive("nonexistent")
	assert.False(t, ok)
	assert.Equal(t, "links", m.ActiveID()) // Should remain unchanged
}

func TestSetActiveDisabledRegion(t *testing.T) {
	m := NewManager()
	m.SetRegions([]Region{
		NewRegion("tabs", RegionTabs, "Tabs"),
		NewDisabledRegion("links", RegionLinks, "Links"),
	})

	// Try to set active to disabled region
	ok := m.SetActive("links")
	assert.False(t, ok)
	assert.Equal(t, "tabs", m.ActiveID())
}

func TestEnableRegion(t *testing.T) {
	m := NewManager()
	m.SetRegions([]Region{
		NewRegion("tabs", RegionTabs, "Tabs"),
		NewDisabledRegion("links", RegionLinks, "Links"),
	})

	// Enable links region
	m.EnableRegion("links")

	// Now we should be able to set it active
	ok := m.SetActive("links")
	assert.True(t, ok)
	assert.Equal(t, "links", m.ActiveID())
}

func TestDisableRegion(t *testing.T) {
	m := NewManager()
	m.SetRegions([]Region{
		NewRegion("tabs", RegionTabs, "Tabs"),
		NewRegion("links", RegionLinks, "Links"),
		NewRegion("viewport", RegionViewport, "Content"),
	})

	// Set active to links
	m.SetActive("links")
	assert.Equal(t, "links", m.ActiveID())

	// Disable links - should move to next enabled region
	m.DisableRegion("links")
	assert.NotEqual(t, "links", m.ActiveID())
}

func TestHandleKeyTab(t *testing.T) {
	m := NewManager()
	m.SetRegions([]Region{
		NewRegion("tabs", RegionTabs, "Tabs"),
		NewRegion("links", RegionLinks, "Links"),
		NewRegion("viewport", RegionViewport, "Content"),
	})

	// Initial state
	assert.Equal(t, "tabs", m.ActiveID())

	// Press Tab - should move to links
	msg := m.HandleKey(tea.KeyMsg{Type: tea.KeyTab})
	require.NotNil(t, msg)
	focusMsg, ok := msg.(FocusChangedMsg)
	require.True(t, ok)
	assert.Equal(t, "tabs", focusMsg.FromRegion.ID)
	assert.Equal(t, "links", focusMsg.ToRegion.ID)
	assert.Equal(t, "links", m.ActiveID())

	// Press Tab again - should move to viewport
	msg = m.HandleKey(tea.KeyMsg{Type: tea.KeyTab})
	require.NotNil(t, msg)
	assert.Equal(t, "viewport", m.ActiveID())

	// Press Tab again - should wrap to tabs
	msg = m.HandleKey(tea.KeyMsg{Type: tea.KeyTab})
	require.NotNil(t, msg)
	assert.Equal(t, "tabs", m.ActiveID())
}

func TestHandleKeyShiftTab(t *testing.T) {
	m := NewManager()
	m.SetRegions([]Region{
		NewRegion("tabs", RegionTabs, "Tabs"),
		NewRegion("links", RegionLinks, "Links"),
		NewRegion("viewport", RegionViewport, "Content"),
	})

	// Press Shift+Tab - should wrap to viewport
	msg := m.HandleKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	require.NotNil(t, msg)
	assert.Equal(t, "viewport", m.ActiveID())

	// Press Shift+Tab - should move to links
	msg = m.HandleKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	require.NotNil(t, msg)
	assert.Equal(t, "links", m.ActiveID())
}

func TestHandleKeySkipsDisabled(t *testing.T) {
	m := NewManager()
	m.SetRegions([]Region{
		NewRegion("tabs", RegionTabs, "Tabs"),
		NewDisabledRegion("links", RegionLinks, "Links"),
		NewRegion("viewport", RegionViewport, "Content"),
	})

	// Press Tab - should skip disabled links and go to viewport
	msg := m.HandleKey(tea.KeyMsg{Type: tea.KeyTab})
	require.NotNil(t, msg)
	assert.Equal(t, "viewport", m.ActiveID())
}

func TestHandleKeyNoRegions(t *testing.T) {
	m := NewManager()

	// Should not panic with no regions
	msg := m.HandleKey(tea.KeyMsg{Type: tea.KeyTab})
	assert.Nil(t, msg)
}

func TestHandleKeyUnhandledKey(t *testing.T) {
	m := NewManager()
	m.SetRegions([]Region{
		NewRegion("tabs", RegionTabs, "Tabs"),
	})

	// Non-tab keys should not be handled
	msg := m.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, msg)

	msg = m.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Nil(t, msg)
}

func TestHandleKeySingleRegion(t *testing.T) {
	m := NewManager()
	m.SetRegions([]Region{
		NewRegion("viewport", RegionViewport, "Content"),
	})

	// With only one region, Tab should return nil (no change)
	msg := m.HandleKey(tea.KeyMsg{Type: tea.KeyTab})
	assert.Nil(t, msg)
}
