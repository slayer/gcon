package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeUpgradeSubmit struct{ version string }

func TestGKEUpgradeDialog_CurrentMarked(t *testing.T) {
	d := NewGKEUpgradeDialog("Upgrade control plane", "1.30.5-gke.1014001",
		[]string{"1.31.1-gke.1", "1.30.5-gke.1014001"},
		func(v string) tea.Msg { return fakeUpgradeSubmit{version: v} })
	out := d.View()
	assert.Contains(t, out, "1.30.5-gke.1014001 (current)")
	assert.Contains(t, out, "1.31.1-gke.1")
}

func TestGKEUpgradeDialog_EnterSubmits(t *testing.T) {
	d := NewGKEUpgradeDialog("Upgrade pool: default", "1.30.5-gke.1014001",
		[]string{"1.31.1-gke.1", "1.30.5-gke.1014001"},
		func(v string) tea.Msg { return fakeUpgradeSubmit{version: v} })
	// Cursor starts at index 0 → "1.31.1-gke.1".
	cmd, _ := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	got, ok := msg.(fakeUpgradeSubmit)
	require.True(t, ok)
	assert.Equal(t, "1.31.1-gke.1", got.version)
}

func TestGKEUpgradeDialog_EscEmitsCancel(t *testing.T) {
	d := NewGKEUpgradeDialog("Upgrade", "v1", []string{"v2", "v1"}, func(string) tea.Msg { return nil })
	cmd, _ := d.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, cmd)
	_, ok := cmd().(GKEUpgradeCanceledMsg)
	assert.True(t, ok)
}
