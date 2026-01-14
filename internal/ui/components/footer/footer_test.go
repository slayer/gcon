package footer

import (
	"testing"
	"time"

	"github.com/slayer/gcon/internal/ui/context"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	ctx := context.New()
	m := New(ctx)

	assert.NotNil(t, m.ctx)
	assert.Equal(t, "? help • q quit", m.helpText)
	assert.False(t, m.showConfirmQuit)
}

func TestSetLeftSection(t *testing.T) {
	ctx := context.New()
	m := New(ctx)

	m.SetLeftSection("3 instances")
	assert.Equal(t, "3 instances", m.leftSection)
}

func TestSetRightSection(t *testing.T) {
	ctx := context.New()
	m := New(ctx)

	m.SetRightSection("Last refresh: 5m ago")
	assert.Equal(t, "Last refresh: 5m ago", m.rightSection)
}

func TestSetShowConfirmQuit(t *testing.T) {
	ctx := context.New()
	m := New(ctx)
	m.SetWidth(100)

	m.SetShowConfirmQuit(true)
	view := m.View()

	assert.Contains(t, view, "Really quit?")
}

func TestViewEmpty(t *testing.T) {
	ctx := context.New()
	m := New(ctx)

	// Width 0 returns empty
	view := m.View()
	assert.Equal(t, "", view)
}

func TestViewWithSections(t *testing.T) {
	ctx := context.New()
	m := New(ctx)
	m.SetWidth(100)
	m.SetLeftSection("5 disks")
	m.SetHelpText("? help")

	view := m.View()

	assert.Contains(t, view, "5 disks")
	assert.Contains(t, view, "? help")
}

func TestRenderActiveTask(t *testing.T) {
	ctx := context.New()
	ctx.Tasks["load"] = context.Task{
		ID:          "load",
		Description: "Loading instances...",
		State:       context.TaskRunning,
		StartTime:   time.Now().Add(-5 * time.Second),
	}

	m := New(ctx)
	m.SetWidth(100)

	view := m.View()

	assert.Contains(t, view, "Loading instances...")
}

func TestFormatResourceCount(t *testing.T) {
	tests := []struct {
		resourceType string
		count        int
		expected     string
	}{
		{"instances", 0, "0 instances"},
		{"instances", 1, "1 instance"},
		{"instances", 5, "5 instances"},
		{"disks", 1, "1 disk"},
		{"bucket", 1, "1 bucket"},
		{"bucket", 3, "3 bucket"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatResourceCount(tt.resourceType, tt.count)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatLastRefresh(t *testing.T) {
	// Zero time
	assert.Equal(t, "", FormatLastRefresh(time.Time{}))

	// Just now (within 1 minute)
	assert.Equal(t, "just now", FormatLastRefresh(time.Now().Add(-30*time.Second)))

	// Minutes ago
	result := FormatLastRefresh(time.Now().Add(-5 * time.Minute))
	assert.Equal(t, "5 mins ago", result)

	// 1 minute ago
	result = FormatLastRefresh(time.Now().Add(-1 * time.Minute))
	assert.Equal(t, "1 min ago", result)
}
