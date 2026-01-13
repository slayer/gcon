package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLayout_New(t *testing.T) {
	l := New()
	assert.NotNil(t, l)
	assert.NotNil(t, l.root)
	assert.NotNil(t, l.header)
	assert.NotNil(t, l.content)
	assert.NotNil(t, l.sidebar)
	assert.NotNil(t, l.main)
	assert.NotNil(t, l.footer)
}

func TestLayout_SetSize(t *testing.T) {
	l := New()
	l.SetSize(100, 50)

	// Header should have fixed height
	hW, hH := l.HeaderSize()
	assert.Equal(t, 100, hW)
	assert.Equal(t, HeaderHeight, hH)

	// Footer should have fixed height
	fW, fH := l.FooterSize()
	assert.Equal(t, 100, fW)
	assert.Equal(t, FooterHeight, fH)

	// Content should fill remaining space
	cW, cH := l.ContentSize()
	assert.Equal(t, 100, cW)
	assert.Equal(t, 50-HeaderHeight-FooterHeight, cH)
}

func TestLayout_SidebarInactive(t *testing.T) {
	l := New()
	l.SetSize(100, 50)
	l.SetSidebarActive(false)

	// When sidebar inactive, main content should have full width available
	// The ContentWidth helper handles this case
	mW := l.ContentWidth()
	assert.Equal(t, 100, mW)

	// Content height should be consistent
	_, cH := l.ContentSize()
	assert.Equal(t, 50-HeaderHeight-FooterHeight, cH)
}

func TestLayout_SidebarActive(t *testing.T) {
	l := New()
	l.SetSize(100, 50)
	l.SetSidebarWidth(26)
	l.SetSidebarActive(true)

	// Sidebar should have specified width
	sW, sH := l.SidebarSize()
	assert.Equal(t, 26, sW)
	expectedContentHeight := 50 - HeaderHeight - FooterHeight
	assert.Equal(t, expectedContentHeight, sH)

	// Main should take remaining width
	mW, mH := l.MainSize()
	assert.Equal(t, 100-26, mW)
	assert.Equal(t, expectedContentHeight, mH)
}

func TestLayout_SidebarAndMainHeightsMatch(t *testing.T) {
	// Critical test: sidebar and main must have matching heights
	l := New()
	l.SetSize(120, 40)
	l.SetSidebarWidth(26)
	l.SetSidebarActive(true)

	_, sH := l.SidebarSize()
	_, mH := l.MainSize()
	assert.Equal(t, sH, mH, "Sidebar and main content must have identical heights for JoinHorizontal")
}

func TestLayout_DimensionConsistency(t *testing.T) {
	// Verify total dimensions add up correctly
	l := New()
	l.SetSize(120, 60)
	l.SetSidebarWidth(30)
	l.SetSidebarActive(true)

	hW, hH := l.HeaderSize()
	cW, cH := l.ContentSize()
	fW, fH := l.FooterSize()

	// Widths should all equal root width
	assert.Equal(t, 120, hW)
	assert.Equal(t, 120, cW)
	assert.Equal(t, 120, fW)

	// Heights should sum to root height
	assert.Equal(t, 60, hH+cH+fH)

	// Sidebar + main should equal content width
	sW, sH := l.SidebarSize()
	mW, mH := l.MainSize()
	assert.Equal(t, cW, sW+mW)
	assert.Equal(t, cH, sH)
	assert.Equal(t, cH, mH)
}
