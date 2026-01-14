package textarea

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	ta := New()

	assert.NotNil(t, ta.textarea)
	assert.Empty(t, ta.title)
	assert.False(t, ta.readOnly)
	assert.False(t, ta.focused)
}

func TestWithTitle(t *testing.T) {
	ta := New().WithTitle("Editor")

	assert.Equal(t, "Editor", ta.title)
}

func TestWithPlaceholder(t *testing.T) {
	ta := New().WithPlaceholder("Type here...")

	assert.Equal(t, "Type here...", ta.textarea.Placeholder)
}

func TestWithCharLimit(t *testing.T) {
	ta := New().WithCharLimit(100)

	assert.Equal(t, 100, ta.textarea.CharLimit)
}

func TestWithLineNumbers(t *testing.T) {
	ta := New().WithLineNumbers(false)

	assert.False(t, ta.textarea.ShowLineNumbers)

	ta = ta.WithLineNumbers(true)
	assert.True(t, ta.textarea.ShowLineNumbers)
}

func TestReadOnly(t *testing.T) {
	ta := New().ReadOnly(true)

	assert.True(t, ta.readOnly)

	ta = ta.ReadOnly(false)
	assert.False(t, ta.readOnly)
}

func TestSetValueAndValue(t *testing.T) {
	ta := New()
	ta.SetValue("Hello\nWorld")

	assert.Equal(t, "Hello\nWorld", ta.Value())
}

func TestSetSize(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		title  string
	}{
		{
			name:   "basic size",
			width:  80,
			height: 24,
		},
		{
			name:   "with title",
			width:  80,
			height: 24,
			title:  "Title",
		},
		{
			name:   "minimum size",
			width:  5,
			height: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ta := New()
			if tt.title != "" {
				ta = ta.WithTitle(tt.title)
			}
			ta.SetSize(tt.width, tt.height)

			assert.Equal(t, tt.width, ta.width)
			assert.Equal(t, tt.height, ta.height)
		})
	}
}

func TestFocusAndBlur(t *testing.T) {
	ta := New()

	assert.False(t, ta.Focused())

	ta.Focus()
	assert.True(t, ta.Focused())

	ta.Blur()
	assert.False(t, ta.Focused())
}

func TestLength(t *testing.T) {
	ta := New()
	ta.SetValue("Hello")

	assert.Equal(t, 5, ta.Length())
}

func TestLineCount(t *testing.T) {
	ta := New()
	ta.SetValue("Line 1\nLine 2\nLine 3")

	assert.Equal(t, 3, ta.LineCount())
}

func TestLine(t *testing.T) {
	ta := New()
	ta.SetValue("First\nSecond\nThird")

	assert.Equal(t, "First", ta.Line(0))
	assert.Equal(t, "Second", ta.Line(1))
	assert.Equal(t, "Third", ta.Line(2))
}

func TestInit(t *testing.T) {
	ta := New()
	cmd := ta.Init()

	// Should return blink command
	assert.NotNil(t, cmd)
}

func TestUpdateReadOnly(t *testing.T) {
	ta := New().ReadOnly(true)

	// In read-only mode, updates should be ignored
	_, cmd := ta.Update(nil)
	assert.Nil(t, cmd)
}

func TestView(t *testing.T) {
	ta := New().WithTitle("Editor")
	ta.SetValue("Test content")

	view := ta.View()

	assert.Contains(t, view, "Editor")
}

func TestViewReadOnly(t *testing.T) {
	ta := New().ReadOnly(true)
	ta.SetValue("Read only content")

	view := ta.View()

	assert.Contains(t, view, "[READ-ONLY]")
}

func TestKeyBindings(t *testing.T) {
	ta := New()
	bindings := ta.KeyBindings()

	assert.Len(t, bindings, 2)
}

func TestDefaultKeyMap(t *testing.T) {
	km := DefaultKeyMap()

	assert.NotEmpty(t, km.Submit.Keys())
	assert.NotEmpty(t, km.Cancel.Keys())
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{123, "123"},
		{-5, "-5"},
	}

	for _, tt := range tests {
		result := itoa(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}
