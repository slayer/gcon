package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGKENodePoolResizeDialog_ManualSubmit(t *testing.T) {
	d := NewGKENodePoolResizeDialog("default", 3, false, 0, 0)
	// In manual mode (default). Type "5" then Enter.
	d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	cmd, _ := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	req, ok := msg.(GKENodePoolResizeSubmitMsg)
	require.True(t, ok)
	assert.Equal(t, GKENodePoolResizeManual, req.Mode)
	assert.Equal(t, int64(5), req.NodeCount)
}

func TestGKENodePoolResizeDialog_TabSwitchesMode(t *testing.T) {
	d := NewGKENodePoolResizeDialog("default", 3, false, 0, 0)
	require.Equal(t, GKENodePoolResizeManual, d.Mode())
	d.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, GKENodePoolResizeAutoscale, d.Mode())
}

func TestGKENodePoolResizeDialog_EscCancels(t *testing.T) {
	d := NewGKENodePoolResizeDialog("default", 3, false, 0, 0)
	cmd, _ := d.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, cmd)
	_, ok := cmd().(GKENodePoolResizeCanceledMsg)
	assert.True(t, ok)
}
