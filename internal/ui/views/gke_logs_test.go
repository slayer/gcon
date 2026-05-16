package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestGKELogs_DefaultLQL(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	q := s.buildLQL()
	assert.Contains(t, q, `resource.type = "k8s_cluster"`)
	assert.Contains(t, q, `resource.labels.cluster_name = "prod"`)
	assert.Contains(t, q, `resource.labels.location = "us-central1"`)
	assert.Contains(t, q, `severity >= INFO`)
}

func TestGKELogs_SeverityToggle(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	s.infoOn = false
	s.warnOn = false
	q := s.buildLQL()
	assert.Contains(t, q, `severity >= ERROR`)
}

func TestGKELogs_ResourceTypeSwitch(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	s.resourceType = "k8s_pod"
	q := s.buildLQL()
	assert.Contains(t, q, `resource.type = "k8s_pod"`)
}

func TestGKELogs_AutoRefreshOffByDefault(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	assert.False(t, s.autoRefresh)
}

func TestGKELogs_StaleTickDropped(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	s.tabActive = false
	s.autoRefresh = true
	cmd := s.Update(gkeLogsRefreshTickMsg{})
	assert.Nil(t, cmd)
}

func TestGKELogs_KeyToggleInfo(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	assert.True(t, s.infoOn)
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'I'}})
	assert.False(t, s.infoOn)
}
