package table

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newScrollableTable(rowCount int) Model {
	cols := []Column{
		{Title: "Name", Width: 20, Grow: true},
	}
	m := NewWithColumns(cols, "Test")

	rows := make([]Row, rowCount)
	for i := range rowCount {
		id := fmt.Sprintf("item-%d", i)
		rows[i] = Row{Data: []string{id}, FilterValue: id, ID: id}
	}
	m.SetRows(rows)
	m.SetSize(80, 20)
	return m
}

func TestNearBottom_FiresAtThreshold(t *testing.T) {
	m := newScrollableTable(20)
	m.SetNearBottomThreshold(3)

	// Move cursor near the bottom
	m.table.SetCursor(17) // 3 from end of 20

	cmd := m.checkNearBottom()
	require.NotNil(t, cmd)

	// Execute the command to get the message
	msg := cmd()
	_, ok := msg.(NearBottomMsg)
	assert.True(t, ok, "should emit NearBottomMsg")
}

func TestNearBottom_DoesNotFireWhenFarFromBottom(t *testing.T) {
	m := newScrollableTable(20)
	m.SetNearBottomThreshold(3)

	m.table.SetCursor(5) // Far from bottom

	cmd := m.checkNearBottom()
	assert.Nil(t, cmd)
}

func TestNearBottom_DoesNotFireAgainUntilReset(t *testing.T) {
	m := newScrollableTable(20)
	m.SetNearBottomThreshold(3)

	m.table.SetCursor(18)

	// First check should fire
	cmd := m.checkNearBottom()
	assert.NotNil(t, cmd)

	// Second check should not fire (already emitted)
	cmd = m.checkNearBottom()
	assert.Nil(t, cmd)

	// After reset, should fire again
	m.ResetNearBottom()
	cmd = m.checkNearBottom()
	assert.NotNil(t, cmd)
}

func TestNearBottom_DisabledWhenThresholdIsZero(t *testing.T) {
	m := newScrollableTable(20)
	// Threshold defaults to 0 (disabled)

	m.table.SetCursor(19)

	cmd := m.checkNearBottom()
	assert.Nil(t, cmd)
}

func TestNearBottom_NegativeThresholdTreatedAsZero(t *testing.T) {
	m := newScrollableTable(20)
	m.SetNearBottomThreshold(-5)

	m.table.SetCursor(19) // At the very bottom

	cmd := m.checkNearBottom()
	assert.Nil(t, cmd, "negative threshold should be treated as disabled")
}

func TestAppendRows_PreservesCursorPosition(t *testing.T) {
	m := newScrollableTable(10)
	m.table.SetCursor(5)

	newRows := []Row{
		{Data: []string{"new1"}, FilterValue: "new1", ID: "new1"},
		{Data: []string{"new2"}, FilterValue: "new2", ID: "new2"},
	}
	m.AppendRows(newRows)

	assert.Equal(t, 12, m.RowCount(), "total rows should increase")
	assert.Equal(t, 5, m.table.Cursor(), "cursor should be preserved")
}

func TestAppendRows_ResetsNearBottom(t *testing.T) {
	m := newScrollableTable(10)
	m.SetNearBottomThreshold(3)
	m.nearBottomEmitted = true

	newRows := []Row{
		{Data: []string{"new"}, FilterValue: "new", ID: "new"},
	}
	m.AppendRows(newRows)

	assert.False(t, m.nearBottomEmitted, "nearBottomEmitted should be reset")
}

func TestNearBottom_IntegratedWithUpdate(t *testing.T) {
	m := newScrollableTable(10)
	m.SetNearBottomThreshold(3)
	m.SetSize(80, 15)

	// Move cursor to within threshold of bottom
	m.table.SetCursor(7)

	// Update with down key - this moves cursor and triggers checkNearBottom
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	// After the update, the nearBottomEmitted flag should be set
	// since cursor 8 is within 3 of end (10)
	assert.True(t, m.nearBottomEmitted, "near-bottom should be detected after cursor moved near end")
}
