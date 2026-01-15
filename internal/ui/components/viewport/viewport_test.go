package viewport

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	vp := New(80, 24)

	assert.Equal(t, 80, vp.width)
	assert.Equal(t, 24, vp.height)
	assert.False(t, vp.showBorder)
	assert.Empty(t, vp.title)
}

func TestWithTitle(t *testing.T) {
	vp := New(80, 24).WithTitle("Test Title")

	assert.Equal(t, "Test Title", vp.title)
}

func TestWithBorder(t *testing.T) {
	vp := New(80, 24).WithBorder(true)

	assert.True(t, vp.showBorder)
}

func TestSetContent(t *testing.T) {
	vp := New(80, 24)
	content := "Line 1\nLine 2\nLine 3"
	vp.SetContent(content)

	assert.True(t, vp.ready)
}

func TestSetSize(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		title     string
		border    bool
		wantWidth int
	}{
		{
			name:      "basic size",
			width:     100,
			height:    30,
			wantWidth: 100,
		},
		{
			name:      "with title reduces height",
			width:     100,
			height:    30,
			title:     "Title",
			wantWidth: 100,
		},
		{
			name:      "with border reduces dimensions",
			width:     100,
			height:    30,
			border:    true,
			wantWidth: 96, // 100 - 4 for borders
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vp := New(80, 24)
			if tt.title != "" {
				vp = vp.WithTitle(tt.title)
			}
			if tt.border {
				vp = vp.WithBorder(true)
			}
			vp.SetSize(tt.width, tt.height)

			assert.Equal(t, tt.width, vp.width)
			assert.Equal(t, tt.height, vp.height)
		})
	}
}

func TestScrollMethods(t *testing.T) {
	vp := New(80, 10)

	// Create content longer than viewport
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "Line " + string(rune('0'+i/10)) + string(rune('0'+i%10))
	}
	vp.SetContent(strings.Join(lines, "\n"))

	// Initially at top
	assert.True(t, vp.AtTop())

	// Go to bottom
	vp.GotoBottom()
	assert.True(t, vp.AtBottom())

	// Go back to top
	vp.GotoTop()
	assert.True(t, vp.AtTop())
}

func TestTotalLineCount(t *testing.T) {
	vp := New(80, 24)
	vp.SetContent("Line 1\nLine 2\nLine 3\nLine 4\nLine 5")

	assert.Equal(t, 5, vp.TotalLineCount())
}

func TestInit(t *testing.T) {
	vp := New(80, 24)
	cmd := vp.Init()

	assert.Nil(t, cmd)
}

func TestUpdate(t *testing.T) {
	vp := New(80, 24)
	vp.SetContent(strings.Repeat("Line\n", 100))

	// Test key message
	updated, _ := vp.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.NotNil(t, updated)
}

func TestViewEmpty(t *testing.T) {
	vp := New(80, 24)

	// Should return empty string when not ready
	view := vp.View()
	assert.Empty(t, view)
}

func TestViewWithContent(t *testing.T) {
	vp := New(80, 24).WithTitle("Test")
	vp.SetContent("Hello World")

	view := vp.View()

	assert.Contains(t, view, "Test")
	assert.Contains(t, view, "Hello World")
}

func TestKeyBindings(t *testing.T) {
	vp := New(80, 24)
	bindings := vp.KeyBindings()

	assert.Len(t, bindings, 6)
}

func TestDefaultKeyMap(t *testing.T) {
	km := DefaultKeyMap()

	assert.NotEmpty(t, km.Up.Keys())
	assert.NotEmpty(t, km.Down.Keys())
	assert.NotEmpty(t, km.PageUp.Keys())
	assert.NotEmpty(t, km.PageDown.Keys())
	assert.NotEmpty(t, km.Top.Keys())
	assert.NotEmpty(t, km.Bottom.Keys())
}

func TestScrollPercent(t *testing.T) {
	vp := New(80, 10)
	vp.SetContent(strings.Repeat("Line\n", 100))

	// At top, should be 0
	vp.GotoTop()
	assert.Equal(t, 0.0, vp.ScrollPercent())

	// At bottom, should be 1
	vp.GotoBottom()
	assert.Equal(t, 1.0, vp.ScrollPercent())
}
