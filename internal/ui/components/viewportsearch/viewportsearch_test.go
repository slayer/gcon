package viewportsearch

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	m := New()
	require.NotNil(t, m)
	assert.False(t, m.IsActive())
	assert.Equal(t, -1, m.CurrentMatchLine())
	assert.Equal(t, 0, m.MatchCount())
}

func TestOpenClose(t *testing.T) {
	m := New()

	m.Open()
	assert.True(t, m.IsActive())
	assert.True(t, m.HasTextInputFocused())
	assert.Equal(t, -1, m.CurrentMatchLine())

	m.Close()
	assert.False(t, m.IsActive())
	assert.False(t, m.HasTextInputFocused())
}

func TestSetContent_StripANSI(t *testing.T) {
	m := New()
	m.Open()

	// Content with ANSI escape codes
	content := "\x1b[32mGreen text\x1b[0m\nSecond line\n\x1b[31mRed text\x1b[0m"
	m.SetContent(content)

	// Searching for "green" should match the first line (stripped)
	m.input.SetValue("green")
	m.rebuildMatches()

	assert.Equal(t, 1, m.MatchCount())
	assert.Equal(t, 0, m.CurrentMatchLine()) // first line
}

func TestSetContent_MatchLines(t *testing.T) {
	m := New()
	m.Open()

	content := "alpha\nbeta\nalpha beta\ngamma"
	m.SetContent(content)

	// No query yet
	assert.Equal(t, 0, m.MatchCount())
	assert.Equal(t, -1, m.CurrentMatchLine())

	// Set query to "alpha"
	m.input.SetValue("alpha")
	m.rebuildMatches()

	assert.Equal(t, 2, m.MatchCount())
	assert.Equal(t, 0, m.CurrentMatchLine()) // first match on line 0
	assert.Equal(t, 1, m.CurrentMatchIndex())
}

func TestNavigation(t *testing.T) {
	m := New()
	m.Open()

	content := "foo\nbar\nfoo bar\nbaz\nfoo"
	m.SetContent(content)

	m.input.SetValue("foo")
	m.rebuildMatches()

	// Matches on lines 0, 2, 4
	assert.Equal(t, 3, m.MatchCount())
	assert.Equal(t, 0, m.CurrentMatchLine())

	m.NextMatch()
	assert.Equal(t, 2, m.CurrentMatchLine())

	m.NextMatch()
	assert.Equal(t, 4, m.CurrentMatchLine())

	// Wrap around
	m.NextMatch()
	assert.Equal(t, 0, m.CurrentMatchLine())

	// Prev from start wraps to last
	m.PrevMatch()
	assert.Equal(t, 4, m.CurrentMatchLine())
}

func TestCaseInsensitive(t *testing.T) {
	m := New()
	m.Open()

	content := "Hello World\nhello world\nHELLO WORLD"
	m.SetContent(content)

	m.input.SetValue("hello")
	m.rebuildMatches()

	assert.Equal(t, 3, m.MatchCount())
}

func TestNoMatches(t *testing.T) {
	m := New()
	m.Open()

	content := "foo\nbar\nbaz"
	m.SetContent(content)

	m.input.SetValue("xyz")
	m.rebuildMatches()

	assert.Equal(t, 0, m.MatchCount())
	assert.Equal(t, -1, m.CurrentMatchLine())
	assert.Equal(t, 0, m.CurrentMatchIndex())
}

func TestEmptyQuery(t *testing.T) {
	m := New()
	m.Open()

	content := "foo\nbar"
	m.SetContent(content)

	m.input.SetValue("")
	m.rebuildMatches()

	assert.Equal(t, 0, m.MatchCount())
	assert.Equal(t, -1, m.CurrentMatchLine())
}

func TestUpdate_CloseOnEsc(t *testing.T) {
	m := New()
	m.Open()

	content := "foo\nbar"
	m.SetContent(content)

	// Send escape key
	cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Nil(t, cmd)
	assert.False(t, m.IsActive())
}

func TestUpdate_NextMatchOnEnter(t *testing.T) {
	m := New()
	m.Open()

	content := "foo\nbar\nfoo"
	m.SetContent(content)
	m.input.SetValue("foo")
	m.rebuildMatches()

	assert.Equal(t, 0, m.CurrentMatchLine())

	// Enter navigates to next match
	_ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, 2, m.CurrentMatchLine())
}

func TestUpdate_NotActiveDoesNothing(t *testing.T) {
	m := New()
	// Don't open - should be inactive

	content := "foo\nbar"
	m.SetContent(content)

	// Keys should be ignored when inactive
	cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	assert.Nil(t, cmd)
	assert.False(t, m.IsActive())
}

func TestView_ActiveRendersSearchBar(t *testing.T) {
	m := New()
	m.SetSize(80)

	assert.Empty(t, m.View())

	m.Open()
	view := m.View()
	assert.NotEmpty(t, view)
}

func TestMatchIndexPreservedOnContentUpdate(t *testing.T) {
	m := New()
	m.Open()

	content := "foo\nbar\nfoo"
	m.SetContent(content)
	m.input.SetValue("foo")
	m.rebuildMatches()

	// Move to second match
	m.NextMatch()
	assert.Equal(t, 2, m.CurrentMatchLine())
	assert.Equal(t, 2, m.CurrentMatchIndex())

	// Update content with same query — index should clamp if needed.
	// m.current was 1 (0-based) which is still valid in 4 matches, so it's preserved.
	// CurrentMatchIndex() returns m.current+1 = 2.
	m.SetContent("foo\nfoo\nfoo\nfoo")
	assert.Equal(t, 2, m.CurrentMatchIndex())
}
