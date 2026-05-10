package table

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/stretchr/testify/assert"
)

func TestNewWithColumns(t *testing.T) {
	cols := []Column{
		{Title: "Status", Width: 3, Hidden: false, Grow: false},
		{Title: "Name", Width: 20, Hidden: false, Grow: true},
		{Title: "Zone", Width: 15, Hidden: false, Grow: false},
	}

	m := NewWithColumns(cols, "Test Table")

	assert.Equal(t, "Test Table", m.title)
	assert.Equal(t, 3, len(m.columns))
	assert.Equal(t, 3, len(m.colDefs))
	assert.Equal(t, "No items", m.emptyText)
}

func TestNewWithColumnsHidden(t *testing.T) {
	cols := []Column{
		{Title: "Status", Width: 3, Hidden: false},
		{Title: "Name", Width: 20, Hidden: true}, // Hidden
		{Title: "Zone", Width: 15, Hidden: false},
	}

	m := NewWithColumns(cols, "Test Table")

	// Only visible columns should be in the table
	assert.Equal(t, 2, len(m.columns))
	assert.Equal(t, 3, len(m.colDefs)) // All definitions preserved
}

func TestSetColumnHidden(t *testing.T) {
	cols := []Column{
		{Title: "Status", Width: 3},
		{Title: "Name", Width: 20},
		{Title: "Zone", Width: 15},
	}

	m := NewWithColumns(cols, "Test")
	assert.Equal(t, 3, m.GetVisibleColumnCount())

	m.SetColumnHidden("Name", true)
	assert.False(t, m.columnsComputed) // Should invalidate cache
	assert.True(t, m.colDefs[1].Hidden)
}

// Regression: SetColumnHidden called before SetSize must still rebuild
// the visible-column list pushed to the underlying bubbles table.
// Previously recalcColumns() was a no-op until lastWidth > 0, so a
// construction-time call to hide a column updated colDefs but left
// bubbles thinking all columns were visible until the first resize.
func TestSetColumnHidden_PropagatesBeforeSetSize(t *testing.T) {
	cols := []Column{
		{Title: "Sel", Width: 4},
		{Title: "Name", Width: 20, Grow: true},
		{Title: "Size", Width: 10},
	}
	m := NewWithColumns(cols, "T")
	// Hide Sel before any SetSize. GetVisibleColumnCount queries the
	// underlying bubbles columns slice, so this verifies the rebuild
	// reached bubbles, not just colDefs.
	m.SetColumnHidden("Sel", true)
	assert.Equal(t, 2, m.GetVisibleColumnCount(),
		"Sel should be hidden in the bubbles column list even pre-SetSize")
	// Reverse direction also works pre-SetSize.
	m.SetColumnHidden("Sel", false)
	assert.Equal(t, 3, m.GetVisibleColumnCount())
}

// Regression: hiding a column after rows are loaded must not panic when
// bubbles' renderer iterates over each row's cells to redraw the viewport.
// The bug: row.Data had a cell for the now-hidden column, but
// m.table.SetColumns had reduced the visible-column count, so renderRow
// indexed past the end of the column slice. Repro covers both directions
// (hide then show then hide again).
func TestSetColumnHidden_DoesNotPanicOnRender(t *testing.T) {
	cols := []Column{
		{Title: "Sel", Width: 4, Hidden: true},
		{Title: "Name", Width: 20, Grow: true},
		{Title: "Size", Width: 10},
		{Title: "Modified", Width: 12},
	}
	m := NewWithColumns(cols, "Test")
	m.SetSize(80, 20)
	m.SetRows([]Row{
		{ID: "a", Data: []string{"", "alpha.txt", "1 KB", "2026-05-10"}},
		{ID: "b", Data: []string{"", "beta.txt", "2 KB", "2026-05-10"}},
	})

	// View() drives bubbles' UpdateViewport → renderRow path. Toggling
	// column visibility in either direction must not panic on the
	// subsequent View().
	assert.NotPanics(t, func() { _ = m.View() })

	m.SetColumnHidden("Sel", false)
	assert.NotPanics(t, func() { _ = m.View() })

	m.SetColumnHidden("Sel", true)
	assert.NotPanics(t, func() { _ = m.View() })

	m.SetColumnHidden("Sel", false)
	assert.NotPanics(t, func() { _ = m.View() })
}

// Regression: a hidden column placed before a target column shifts the
// visible-vs-underlying index alignment. parseFilter must store the
// underlying colDef index (not a "visible-only" position) so that
// matchesFilterSpec reads the right cell.
func TestParseFilter_HiddenColumnDoesNotMisalignFieldIndex(t *testing.T) {
	cols := []Column{
		{Title: "Sel", Width: 4, Hidden: true},
		{Title: "Name", Width: 20, Grow: true, FilterKeys: []string{"name"}},
		{Title: "Class", Width: 10, FilterKeys: []string{"class"}},
	}
	m := NewWithColumns(cols, "T")
	spec := m.parseFilter("name:foo")
	assert.Len(t, spec.fieldFilters, 1)
	// Underlying colDef index for "Name" is 1 — even though it's at visible
	// index 0 (Sel is hidden). Storing 0 would cause matchesFilterSpec to
	// read row.Data[0] (the Sel cell) instead of the Name cell.
	assert.Equal(t, "foo", spec.fieldFilters[1])
}

// Regression: sortRows must translate the public visible-column index
// into the underlying colDef index before reading row.Data, otherwise
// it sorts on the wrong cell when a hidden column comes before the sort
// column.
func TestSortRows_RespectsHiddenColumnOffset(t *testing.T) {
	cols := []Column{
		{Title: "Sel", Width: 4, Hidden: true},
		{Title: "Name", Width: 20, Grow: true},
		{Title: "Size", Width: 10},
	}
	m := NewWithColumns(cols, "T")
	m.SetSize(60, 10)
	// Cells in row.Data are indexed by colDef position: [Sel, Name, Size].
	m.SetRows([]Row{
		{ID: "a", Data: []string{"", "zeta", "1"}},
		{ID: "b", Data: []string{"", "alpha", "2"}},
		{ID: "c", Data: []string{"", "mike", "3"}},
	})
	// Sort by visible column 0 = Name (Sel is hidden, so visible 0 maps
	// to underlying 1). Ascending should yield alpha, mike, zeta.
	m.SortBy(0, true)
	rows := m.VisibleRows()
	if assert.Len(t, rows, 3) {
		assert.Equal(t, "alpha", rows[0].Data[1])
		assert.Equal(t, "mike", rows[1].Data[1])
		assert.Equal(t, "zeta", rows[2].Data[1])
	}
}

// VisibleRows reflects the filter, while Rows returns the unfiltered
// allRows. Bulk-action callers ("select all") rely on this distinction.
func TestVisibleRows_ReflectsActiveFilter(t *testing.T) {
	cols := []Column{
		{Title: "Name", Width: 20, Grow: true, FilterKeys: []string{"name"}},
	}
	m := NewWithColumns(cols, "T")
	m.SetSize(60, 10)
	m.SetRows([]Row{
		{ID: "a", Data: []string{"alpha"}, FilterValue: "alpha"},
		{ID: "b", Data: []string{"alpaca"}, FilterValue: "alpaca"},
		{ID: "c", Data: []string{"zeta"}, FilterValue: "zeta"},
	})
	// Apply a free-text filter that matches only "alp*" rows.
	m.filter.SetValue("alp")
	m.applyFilter()

	all := m.Rows()
	visible := m.VisibleRows()
	assert.Len(t, all, 3, "Rows() should return all rows")
	assert.Len(t, visible, 2, "VisibleRows() should drop the filtered-out row")
}

// SetRows (and the filter / sort pipelines that call it internally) must
// also feed bubbles only the cells matching its current visible-column
// count, otherwise renderRow panics on the next viewport refresh.
func TestSetRows_ShapesCellsToVisibleColumns(t *testing.T) {
	cols := []Column{
		{Title: "Sel", Width: 4, Hidden: true},
		{Title: "Name", Width: 20, Grow: true},
		{Title: "Size", Width: 10},
	}
	m := NewWithColumns(cols, "Test")
	m.SetSize(60, 10)
	// Rows have data for all defined columns including the hidden one.
	m.SetRows([]Row{{ID: "a", Data: []string{"[ ]", "alpha", "1 KB"}}})
	assert.NotPanics(t, func() { _ = m.View() })

	// Toggle the hidden column on; rows must now expose all cells.
	m.SetColumnHidden("Sel", false)
	assert.NotPanics(t, func() { _ = m.View() })

	// And back off again — the dangerous direction (shrink columns).
	m.SetColumnHidden("Sel", true)
	assert.NotPanics(t, func() { _ = m.View() })
}

func TestGetVisibleColumnCount(t *testing.T) {
	cols := []Column{
		{Title: "A", Width: 5, Hidden: false},
		{Title: "B", Width: 5, Hidden: true},
		{Title: "C", Width: 5, Hidden: false},
	}

	m := NewWithColumns(cols, "Test")
	assert.Equal(t, 2, m.GetVisibleColumnCount())
}

func TestSetLoading(t *testing.T) {
	cols := []table.Column{{Title: "Name", Width: 20}}
	m := New(cols, "Test")

	assert.False(t, m.IsLoading())

	m.SetLoading(true, "Loading instances...")
	assert.True(t, m.IsLoading())
	assert.Equal(t, "Loading instances...", m.loadingText)

	m.SetLoading(true, "")
	assert.Equal(t, "Loading...", m.loadingText) // Default text

	m.SetLoading(false, "")
	assert.False(t, m.IsLoading())
}

func TestSetEmptyText(t *testing.T) {
	cols := []table.Column{{Title: "Name", Width: 20}}
	m := New(cols, "Test")

	assert.Equal(t, "No items", m.emptyText)

	m.SetEmptyText("No instances found")
	assert.Equal(t, "No instances found", m.emptyText)
}

func TestColumnWidthCaching(t *testing.T) {
	cols := []Column{
		{Title: "Name", Width: 20, Grow: true},
		{Title: "Status", Width: 10},
	}

	m := NewWithColumns(cols, "Test")

	// First SetSize should calculate
	m.SetSize(100, 20)
	assert.True(t, m.columnsComputed)
	assert.Equal(t, 100, m.lastWidth)

	// Same width should not recalculate
	m.columnsComputed = true
	m.SetSize(100, 25) // Different height, same width
	assert.True(t, m.columnsComputed)

	// Different width should recalculate
	m.SetSize(120, 20)
	assert.Equal(t, 120, m.lastWidth)
}

func TestAdjustColumnsWithGrow(t *testing.T) {
	cols := []Column{
		{Title: "Status", Width: 5, Grow: false}, // Fixed
		{Title: "Name", Width: 20, Grow: true},   // Grows
		{Title: "Zone", Width: 15, Grow: false},  // Fixed
	}

	m := NewWithColumns(cols, "Test")
	m.SetSize(100, 20)

	// Name column should have grown
	assert.Greater(t, m.colDefs[1].ComputedWidth, 20)

	// Fixed columns should keep their width
	assert.Equal(t, 5, m.colDefs[0].ComputedWidth)
	assert.Equal(t, 15, m.colDefs[2].ComputedWidth)
}

func TestBackwardCompatibility(t *testing.T) {
	// Old-style column definition should still work
	cols := []table.Column{
		{Title: "Name", Width: 20},
		{Title: "Status", Width: 10},
	}

	m := New(cols, "Test")
	assert.Equal(t, 2, len(m.columns))
	assert.Equal(t, 0, len(m.colDefs)) // No enhanced defs

	m.SetSize(100, 20)
	// Should not panic, should use legacy width adjustment
	assert.Equal(t, 2, m.GetVisibleColumnCount()) // Falls back to len(columns)
}

func TestSetRows(t *testing.T) {
	cols := []table.Column{{Title: "Name", Width: 20}}
	m := New(cols, "Test")

	rows := []Row{
		{Data: []string{"Item 1"}, FilterValue: "item 1", ID: "1"},
		{Data: []string{"Item 2"}, FilterValue: "item 2", ID: "2"},
	}

	m.SetRows(rows)
	assert.Equal(t, 2, m.TotalRowCount())
	assert.Equal(t, 2, m.RowCount())
}
