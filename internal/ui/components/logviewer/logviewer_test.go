package logviewer

import (
	"fmt"
	"testing"
	"time"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
)

func makeTestEntries(n int) []gcp.LogEntry {
	entries := make([]gcp.LogEntry, n)
	for i := range n {
		entries[i] = gcp.LogEntry{
			Timestamp:    time.Now().Add(-time.Duration(n-i) * time.Minute),
			Severity:     "INFO",
			Message:      fmt.Sprintf("message %d", i),
			ResourceType: "gce_instance",
			InsertID:     fmt.Sprintf("id-%d", i),
		}
	}
	return entries
}

func TestLogViewerSetEntries(t *testing.T) {
	m := New()
	m.SetSize(80, 24)

	entries := makeTestEntries(5)
	m.SetEntries(entries)

	assert.Equal(t, 5, m.EntryCount())
	assert.Equal(t, 0, m.Cursor())
}

func TestLogViewerExpandCollapse(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetEntries(makeTestEntries(5))

	assert.False(t, m.IsExpanded(0))
	m.ToggleExpand(0)
	assert.True(t, m.IsExpanded(0))
	m.ToggleExpand(0)
	assert.False(t, m.IsExpanded(0))
}

func TestLogViewerExpandAll(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetEntries(makeTestEntries(3))

	m.ExpandAll()
	for i := range 3 {
		assert.True(t, m.IsExpanded(i))
	}

	m.CollapseAll()
	for i := range 3 {
		assert.False(t, m.IsExpanded(i))
	}
}

func TestLogViewerNavigation(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetEntries(makeTestEntries(5))

	assert.Equal(t, 0, m.Cursor())
	m.MoveDown()
	assert.Equal(t, 1, m.Cursor())
	m.MoveDown()
	assert.Equal(t, 2, m.Cursor())
	m.MoveUp()
	assert.Equal(t, 1, m.Cursor())
}

func TestLogViewerNavigationBounds(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetEntries(makeTestEntries(3))

	m.MoveUp()
	assert.Equal(t, 0, m.Cursor())

	m.MoveDown()
	m.MoveDown()
	m.MoveDown()
	assert.Equal(t, 2, m.Cursor())
}

func TestLogViewerFieldCursor(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	entries := makeTestEntries(1)
	entries[0].ResourceType = "gce_instance"
	entries[0].InsertID = "abc"
	m.SetEntries(entries)

	m.ToggleExpand(0)
	assert.True(t, m.IsExpanded(0))
	assert.Equal(t, -1, m.FieldCursor())

	m.EnterFieldNav()
	assert.Equal(t, 0, m.FieldCursor())

	m.FieldDown()
	assert.Equal(t, 1, m.FieldCursor())

	m.ExitFieldNav()
	assert.Equal(t, -1, m.FieldCursor())
}

func TestLogViewerSelectedField(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	entries := []gcp.LogEntry{{
		ResourceType: "gce_instance",
		InsertID:     "abc",
	}}
	m.SetEntries(entries)
	m.ToggleExpand(0)
	m.EnterFieldNav()

	field := m.SelectedField()
	assert.NotNil(t, field)
}

func TestLogViewerNeedsMore(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetEntries(makeTestEntries(20))
	m.SetHasMore(true)

	for range 18 {
		m.MoveDown()
	}

	assert.True(t, m.NeedsMore())
}

func TestLogViewerNeedsMoreFalseWhenNoMore(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetEntries(makeTestEntries(20))
	m.SetHasMore(false)

	for range 18 {
		m.MoveDown()
	}

	assert.False(t, m.NeedsMore())
}

func TestLogViewerView(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetEntries(makeTestEntries(3))

	view := m.View()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "message 0")
}

func TestLogViewerViewEmpty(t *testing.T) {
	m := New()
	m.SetSize(80, 24)

	view := m.View()
	assert.Contains(t, view, "No log entries")
}

func TestLogViewerAppendEntries(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetEntries(makeTestEntries(5))
	assert.Equal(t, 5, m.EntryCount())

	m.AppendEntries(makeTestEntries(3))
	assert.Equal(t, 8, m.EntryCount())
}

func TestLogViewerPrependEntries(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetEntries(makeTestEntries(5))

	// Move cursor to entry 2 and expand it
	m.MoveDown()
	m.MoveDown()
	assert.Equal(t, 2, m.Cursor())
	m.ToggleExpand(2)
	assert.True(t, m.IsExpanded(2))

	// Prepend 3 new entries
	m.PrependEntries(makeTestEntries(3))

	assert.Equal(t, 8, m.EntryCount())
	// Cursor should shift by 3 to stay on the same logical entry
	assert.Equal(t, 5, m.Cursor())
	// Expanded map should shift: old index 2 → new index 5
	assert.False(t, m.IsExpanded(2), "old index should no longer be expanded")
	assert.True(t, m.IsExpanded(5), "shifted index should be expanded")
}

func TestLogViewerPrependEntriesEmpty(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetEntries(makeTestEntries(3))
	m.MoveDown()
	assert.Equal(t, 1, m.Cursor())

	// Prepending zero entries should be a no-op
	m.PrependEntries(nil)
	assert.Equal(t, 3, m.EntryCount())
	assert.Equal(t, 1, m.Cursor())
}

func TestLogViewerSetEntriesResetsState(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetEntries(makeTestEntries(5))

	// Move cursor and expand an entry
	m.MoveDown()
	m.MoveDown()
	m.ToggleExpand(2)
	m.EnterFieldNav()

	assert.Equal(t, 2, m.Cursor())
	assert.True(t, m.IsExpanded(2))
	assert.GreaterOrEqual(t, m.FieldCursor(), 0)

	// SetEntries should reset everything
	m.SetEntries(makeTestEntries(3))
	assert.Equal(t, 0, m.Cursor())
	assert.Equal(t, -1, m.FieldCursor())
	assert.False(t, m.IsExpanded(2))
}

func TestLogViewerMoveDownDelegatesFieldNav(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	entries := makeTestEntries(2)
	entries[0].ResourceType = "gce_instance"
	entries[0].InsertID = "abc"
	m.SetEntries(entries)

	m.ToggleExpand(0)
	m.EnterFieldNav()
	assert.Equal(t, 0, m.FieldCursor())

	// MoveDown while in field nav should advance field cursor, not entry cursor
	m.MoveDown()
	assert.Equal(t, 0, m.Cursor(), "entry cursor should stay at 0 during field nav")
	assert.Equal(t, 1, m.FieldCursor())
}

func TestLogViewerMoveUpExitsFieldNav(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	entries := makeTestEntries(2)
	entries[0].ResourceType = "gce_instance"
	entries[0].InsertID = "abc"
	m.SetEntries(entries)

	m.ToggleExpand(0)
	m.EnterFieldNav()
	assert.Equal(t, 0, m.FieldCursor())

	// MoveUp at field 0 should exit field nav
	m.MoveUp()
	assert.Equal(t, -1, m.FieldCursor())
	assert.Equal(t, 0, m.Cursor())
}

func TestLogViewerEnterFieldNavOnCollapsedEntry(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetEntries(makeTestEntries(1))

	// Not expanded, so entering field nav should be a no-op
	m.EnterFieldNav()
	assert.Equal(t, -1, m.FieldCursor())
}

func TestLogViewerSelectedFieldNilWhenNotInFieldNav(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	entries := []gcp.LogEntry{{
		ResourceType: "gce_instance",
		InsertID:     "abc",
	}}
	m.SetEntries(entries)

	// Not in field nav
	assert.Nil(t, m.SelectedField())

	// Expanded but not in field nav
	m.ToggleExpand(0)
	assert.Nil(t, m.SelectedField())
}

func TestLogViewerCollapseResetsFieldCursor(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	entries := makeTestEntries(1)
	entries[0].ResourceType = "gce_instance"
	entries[0].InsertID = "abc"
	m.SetEntries(entries)

	m.ToggleExpand(0)
	m.EnterFieldNav()
	assert.GreaterOrEqual(t, m.FieldCursor(), 0)

	// Collapsing should reset field cursor
	m.ToggleExpand(0)
	assert.Equal(t, -1, m.FieldCursor())
}

func TestLogViewerViewHasMoreIndicator(t *testing.T) {
	m := New()
	m.SetSize(80, 50) // tall enough to show all entries + indicator
	m.SetEntries(makeTestEntries(3))
	m.SetHasMore(true)

	view := m.View()
	assert.Contains(t, view, "Loading more...")
}

func TestLogViewerNeedsMoreEmptyEntries(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetHasMore(true)

	// No entries, should not need more even if hasMore is true
	assert.False(t, m.NeedsMore())
}

func TestLogViewerFieldCursorVisibleDuringFieldNav(t *testing.T) {
	// When in field navigation mode (fieldCur >= 0), the field cursor
	// indicator [+f] must be visible in the rendered output.
	m := New()
	m.SetSize(120, 40)
	entries := []gcp.LogEntry{{
		Timestamp:    makeTestEntries(1)[0].Timestamp,
		Severity:     "ERROR",
		Message:      "test error",
		ResourceType: "gce_instance",
		InsertID:     "abc-123",
	}}
	m.SetEntries(entries)

	m.ToggleExpand(0)
	m.EnterFieldNav()
	assert.GreaterOrEqual(t, m.FieldCursor(), 0, "should be in field nav mode")

	view := m.View()
	assert.Contains(t, view, "[+f]", "field cursor hint should be visible during field navigation")
}
