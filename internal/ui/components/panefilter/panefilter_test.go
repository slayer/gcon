package panefilter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripANSI(t *testing.T) {
	in := "\x1b[31mhello\x1b[0m world\n\x1b[1;33mfoo\x1b[0m"
	got := stripANSI(in)
	assert.Equal(t, "hello world\nfoo", got)
}

func TestFindMatches_PlainText(t *testing.T) {
	content := "hello world\nhello again\nbye"
	matches := findMatches(content, "hello")
	require.Len(t, matches, 2)
	assert.Equal(t, 0, matches[0].Line)
	assert.Equal(t, 0, matches[0].VisStart)
	assert.Equal(t, 5, matches[0].VisEnd)
	assert.Equal(t, 1, matches[1].Line)
	assert.Equal(t, 12, matches[1].VisStart)
	assert.Equal(t, 17, matches[1].VisEnd)
}

func TestFindMatches_CaseInsensitive(t *testing.T) {
	matches := findMatches("HELLO World", "hello")
	require.Len(t, matches, 1)
	assert.Equal(t, 0, matches[0].VisStart)
	assert.Equal(t, 5, matches[0].VisEnd)
}

func TestFindMatches_IgnoresANSI(t *testing.T) {
	content := "\x1b[31mhello\x1b[0m \x1b[32mworld\x1b[0m"
	matches := findMatches(content, "hello world")
	require.Len(t, matches, 1, "should match across ANSI boundaries")
	// Visible content is "hello world" (11 bytes).
	assert.Equal(t, 0, matches[0].VisStart)
	assert.Equal(t, 11, matches[0].VisEnd)
}

func TestFindMatches_EmptyQuery(t *testing.T) {
	assert.Nil(t, findMatches("anything", ""))
}

func TestFindMatches_NoMatch(t *testing.T) {
	assert.Empty(t, findMatches("hello world", "zzz"))
}

func TestFindMatches_NonOverlapping(t *testing.T) {
	matches := findMatches("aaaa", "aa")
	require.Len(t, matches, 2)
	assert.Equal(t, 0, matches[0].VisStart)
	assert.Equal(t, 2, matches[0].VisEnd)
	assert.Equal(t, 2, matches[1].VisStart)
	assert.Equal(t, 4, matches[1].VisEnd)
}

func TestHighlightMatches_PlainText(t *testing.T) {
	content := "hello world hello"
	matches := findMatches(content, "hello")
	out := highlightMatches(content, matches, 0)
	// Both matches should be wrapped; the first uses the current-match SGR.
	assert.Contains(t, out, sgrCurrent+"hello"+sgrReset)
	assert.Contains(t, out, sgrMatch+"hello"+sgrReset)
	// Original text content should still be retrievable.
	assert.Equal(t, "hello world hello", stripANSI(out))
}

func TestHighlightMatches_PreservesActiveStyle(t *testing.T) {
	// Lipgloss-style: outer style covers the whole rendered chunk.
	content := "\x1b[31mthe quick brown fox\x1b[0m"
	matches := findMatches(content, "quick")
	require.Len(t, matches, 1)
	out := highlightMatches(content, matches, 0)

	// After highlight closes, the red style should be re-emitted so "brown fox"
	// remains styled.
	idx := strings.Index(out, sgrReset+"\x1b[31m")
	assert.GreaterOrEqual(t, idx, 0, "expected red style restored after match")
	// Stripped text must equal the original visible content.
	assert.Equal(t, "the quick brown fox", stripANSI(out))
}

func TestHighlightMatches_NoMatchesReturnsUnchanged(t *testing.T) {
	content := "hello world"
	out := highlightMatches(content, nil, 0)
	assert.Equal(t, content, out)
}

func TestApply_UpdatesMatchCount(t *testing.T) {
	m := New()
	m.Open() //nolint:errcheck
	m.input.SetValue("foo")
	out := m.Apply("foo bar foo baz foo")
	assert.Equal(t, 3, m.MatchCount())
	assert.Equal(t, 1, m.CurrentMatchIndex(), "cursor starts at first match")
	assert.NotEqual(t, "foo bar foo baz foo", out, "content should be highlighted")
}

func TestApply_EmptyQueryClearsState(t *testing.T) {
	m := New()
	m.Open() //nolint:errcheck
	m.input.SetValue("foo")
	m.Apply("foo bar")
	require.Equal(t, 1, m.MatchCount())

	m.input.SetValue("")
	out := m.Apply("foo bar")
	assert.Equal(t, "foo bar", out)
	assert.Equal(t, 0, m.MatchCount())
	assert.Equal(t, -1, m.MatchLine())
}

func TestNextPrev_Wraps(t *testing.T) {
	m := New()
	m.Open() //nolint:errcheck
	m.input.SetValue("a")
	m.Apply("a b a b a") // 3 matches
	require.Equal(t, 3, m.MatchCount())
	assert.Equal(t, 1, m.CurrentMatchIndex())

	m.Next()
	assert.Equal(t, 2, m.CurrentMatchIndex())
	m.Next()
	assert.Equal(t, 3, m.CurrentMatchIndex())
	m.Next()
	assert.Equal(t, 1, m.CurrentMatchIndex(), "should wrap")

	m.Prev()
	assert.Equal(t, 3, m.CurrentMatchIndex(), "Prev from first should wrap")
}

func TestMatchLine_TracksMultiLineMatches(t *testing.T) {
	m := New()
	m.Open() //nolint:errcheck
	m.input.SetValue("zone")
	m.Apply("Name: foo\nZone: us-central1-a\nState: RUNNING\nzone-info: extra")
	require.Equal(t, 2, m.MatchCount())
	assert.Equal(t, 1, m.MatchLine(), "first match on second line")
	m.Next()
	assert.Equal(t, 3, m.MatchLine(), "second match on fourth line")
}

func TestVisibleAndFocused(t *testing.T) {
	m := New()
	assert.False(t, m.Visible())
	assert.False(t, m.IsFocused())

	m.Open() //nolint:errcheck
	assert.True(t, m.Visible())
	assert.True(t, m.IsFocused())

	m.Close()
	assert.False(t, m.Visible())
	assert.False(t, m.IsFocused())
}

func TestClose_ResetsState(t *testing.T) {
	m := New()
	m.Open() //nolint:errcheck
	m.input.SetValue("foo")
	m.Apply("foo bar foo")
	require.Equal(t, 2, m.MatchCount())

	m.Close()
	assert.Empty(t, m.Query())
	assert.Equal(t, 0, m.MatchCount())
	assert.Equal(t, -1, m.MatchLine())
}

func TestView_ShowsMatchCount(t *testing.T) {
	m := New()
	m.Open() //nolint:errcheck
	m.input.SetValue("foo")
	m.Apply("foo bar foo")
	v := m.View()
	assert.Contains(t, stripANSI(v), "1/2")
	assert.Contains(t, stripANSI(v), "n/N")
}

func TestView_ShowsNoMatches(t *testing.T) {
	m := New()
	m.Open() //nolint:errcheck
	m.input.SetValue("zzz")
	m.Apply("hello world")
	v := m.View()
	assert.Contains(t, stripANSI(v), "no matches")
}

func TestView_HiddenWhenNotVisible(t *testing.T) {
	m := New()
	assert.Empty(t, m.View())
}
