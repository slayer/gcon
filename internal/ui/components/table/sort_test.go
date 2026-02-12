package table

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSortableTable() Model {
	cols := []Column{
		{Title: "Name", Width: 20, Grow: true, Sortable: true},
		{Title: "Zone", Width: 15, Sortable: true},
		{Title: "Size", Width: 10, Sortable: true},
	}
	m := NewWithColumns(cols, "Test")

	rows := []Row{
		{Data: []string{"● charlie", "us-east1-b", "100 GB"}, FilterValue: "charlie", ID: "charlie"},
		{Data: []string{"● alpha", "us-central1-a", "50 GB"}, FilterValue: "alpha", ID: "alpha"},
		{Data: []string{"○ bravo", "europe-west1-c", "200 GB"}, FilterValue: "bravo", ID: "bravo"},
	}
	m.SetRows(rows)
	return m
}

func TestSortBy_StringAscending(t *testing.T) {
	m := newSortableTable()

	m.SortBy(0, true) // Sort by Name ascending

	require.Equal(t, 3, m.RowCount())
	// Sorted by stripped name: alpha, bravo, charlie
	assert.Equal(t, "● alpha", m.rows[0].Data[0])
	assert.Equal(t, "○ bravo", m.rows[1].Data[0])
	assert.Equal(t, "● charlie", m.rows[2].Data[0])
}

func TestSortBy_StringDescending(t *testing.T) {
	m := newSortableTable()

	m.SortBy(0, false) // Sort by Name descending

	assert.Equal(t, "● charlie", m.rows[0].Data[0])
	assert.Equal(t, "○ bravo", m.rows[1].Data[0])
	assert.Equal(t, "● alpha", m.rows[2].Data[0])
}

func TestSortBy_Numeric(t *testing.T) {
	m := newSortableTable()

	m.SortBy(2, true) // Sort by Size ascending

	// 50 GB < 100 GB < 200 GB
	assert.Equal(t, "50 GB", m.rows[0].Data[2])
	assert.Equal(t, "100 GB", m.rows[1].Data[2])
	assert.Equal(t, "200 GB", m.rows[2].Data[2])
}

func TestSortBy_NumericDescending(t *testing.T) {
	m := newSortableTable()

	m.SortBy(2, false) // Sort by Size descending

	assert.Equal(t, "200 GB", m.rows[0].Data[2])
	assert.Equal(t, "100 GB", m.rows[1].Data[2])
	assert.Equal(t, "50 GB", m.rows[2].Data[2])
}

func TestSort_PersistsAcrossFilter(t *testing.T) {
	m := newSortableTable()

	// Sort by name ascending (alpha, bravo, charlie)
	m.SortBy(0, true)

	// Filter to "al" — matches only "alpha" and "charlie" (both contain "al"... check)
	// Actually: alpha contains "al", bravo doesn't, charlie doesn't
	// Use "us" to match "us-east1-b" and "us-central1-a" (charlie and alpha)
	m.filter.SetValue("alpha")
	m.applyFilter()

	require.Equal(t, 1, m.RowCount())
	assert.Equal(t, "alpha", m.rows[0].ID)

	// Broader filter: all contain "a" in FilterValue
	m.filter.SetValue("a")
	m.applyFilter()

	// All 3 match, and sort order should persist (alpha, bravo, charlie)
	require.Equal(t, 3, m.RowCount())
	assert.Equal(t, "alpha", m.rows[0].ID)
	assert.Equal(t, "bravo", m.rows[1].ID)
	assert.Equal(t, "charlie", m.rows[2].ID)
}

func TestSort_ResetsOnSetRows(t *testing.T) {
	m := newSortableTable()

	m.SortBy(0, true)
	assert.Equal(t, 0, m.sortColumn)

	// SetRows should reset sort
	m.SetRows([]Row{
		{Data: []string{"x", "y", "z"}, FilterValue: "x", ID: "x"},
	})
	assert.Equal(t, -1, m.sortColumn)
}

func TestClearSort(t *testing.T) {
	m := newSortableTable()

	m.SortBy(0, true)
	assert.Equal(t, 0, m.sortColumn)

	m.ClearSort()
	assert.Equal(t, -1, m.sortColumn)
}

func TestIsSortMenuOpen(t *testing.T) {
	m := newSortableTable()
	assert.False(t, m.IsSortMenuOpen())

	m.openSortMenu()
	assert.True(t, m.IsSortMenuOpen())
}

func TestParseNumericValue(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
		ok       bool
	}{
		{"100", 100, true},
		{"100 GB", 100e9, true},
		{"50.5%", 50.5, true},
		{"1,024", 1024, true},
		{"1,024 MB", 1024e6, true},
		{"-", 0, false},
		{"", 0, false},
		{"abc", 0, false},
		{"100 TB", 100e12, true},
		{"0", 0, true},
		{"5 KB", 5e3, true},
		{"1 B", 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			val, ok := parseNumericValue(tt.input)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.InDelta(t, tt.expected, val, 0.1)
			}
		})
	}
}

func TestCompareValues_Numeric(t *testing.T) {
	assert.Less(t, compareValues("50 GB", "100 GB"), 0)
	assert.Greater(t, compareValues("200 GB", "100 GB"), 0)
	assert.Equal(t, 0, compareValues("100 GB", "100 GB"))
}

func TestCompareValues_CrossUnit(t *testing.T) {
	// 100 MB should be less than 50 GB (unit magnitude matters)
	assert.Less(t, compareValues("100 MB", "50 GB"), 0)
	assert.Greater(t, compareValues("1 TB", "500 GB"), 0)
	assert.Less(t, compareValues("1024 KB", "2 MB"), 0)
}

func TestCompareValues_String(t *testing.T) {
	assert.Less(t, compareValues("alpha", "bravo"), 0)
	assert.Greater(t, compareValues("charlie", "bravo"), 0)
	assert.Equal(t, 0, compareValues("alpha", "alpha"))
}

func TestCompareValues_CaseInsensitive(t *testing.T) {
	assert.Equal(t, 0, compareValues("Alpha", "alpha"))
}

func TestStripLeadingSymbols(t *testing.T) {
	assert.Equal(t, "vm-web", stripLeadingSymbols("● vm-web"))
	assert.Equal(t, "vm-db", stripLeadingSymbols("○ vm-db"))
	assert.Equal(t, "plain", stripLeadingSymbols("plain"))
	assert.Equal(t, "123", stripLeadingSymbols("  123"))
}

func TestSortIndicator_AppearsInHeader(t *testing.T) {
	m := newSortableTable()
	m.SetSize(80, 20)

	m.SortBy(0, true)

	// Header column title should include ascending indicator
	view := m.View()
	assert.Contains(t, view, "Name ▲", "ascending indicator should appear in header")
	assert.NotContains(t, view, "Zone ▲", "unsorted column should not have indicator")
	assert.NotContains(t, view, "Zone ▼", "unsorted column should not have indicator")

	// Toggle to descending
	m.SortBy(0, false)
	view = m.View()
	assert.Contains(t, view, "Name ▼", "descending indicator should appear in header")
}

func TestSortIndicator_RemovedOnClearSort(t *testing.T) {
	m := newSortableTable()
	m.SetSize(80, 20)

	m.SortBy(0, true)
	assert.Contains(t, m.View(), "Name ▲")

	m.ClearSort()
	view := m.View()
	assert.NotContains(t, view, "▲", "indicator should be removed after ClearSort")
	assert.NotContains(t, view, "▼", "indicator should be removed after ClearSort")
}

func TestSortIndicator_RemovedOnSetRows(t *testing.T) {
	m := newSortableTable()
	m.SetSize(80, 20)

	m.SortBy(0, true)
	assert.Contains(t, m.View(), "Name ▲")

	m.SetRows([]Row{
		{Data: []string{"x", "y", "z"}, FilterValue: "x", ID: "x"},
	})
	view := m.View()
	assert.NotContains(t, view, "▲", "indicator should be removed after SetRows")
	assert.NotContains(t, view, "▼", "indicator should be removed after SetRows")
}

func TestOpenSortMenu_NoSortableColumns(t *testing.T) {
	cols := []Column{
		{Title: "Name", Width: 20},
		{Title: "Zone", Width: 15},
	}
	m := NewWithColumns(cols, "Test")

	m.openSortMenu()
	// No sortable columns -> menu should not open
	assert.False(t, m.sortMenuOpen)
}

func TestSortBy_OutOfBoundsColumnIndex(t *testing.T) {
	m := newSortableTable()
	m.SetSize(80, 20)

	// Sorting by out-of-bounds column should be a no-op
	m.SortBy(99, true)
	// Rows should remain in original order
	assert.Equal(t, "charlie", m.rows[0].ID)
}

func TestAppendRows_PreservesCursorWithSort(t *testing.T) {
	m := newSortableTable()
	m.SetSize(80, 20)

	// Sort ascending by name: alpha, bravo, charlie
	m.SortBy(0, true)
	require.Equal(t, "alpha", m.rows[0].ID)

	// Move cursor to "bravo" (index 1)
	m.table.SetCursor(1)
	require.Equal(t, "bravo", m.rows[m.table.Cursor()].ID)

	// Append a row that sorts before bravo
	m.AppendRows([]Row{
		{Data: []string{"● aardvark", "us-west1-a", "10 GB"}, FilterValue: "aardvark", ID: "aardvark"},
	})

	// Cursor should still point to bravo despite resorting
	require.Equal(t, 4, m.RowCount())
	selected := m.rows[m.table.Cursor()]
	assert.Equal(t, "bravo", selected.ID, "cursor should follow the same row after AppendRows + re-sort")
}
