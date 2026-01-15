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
