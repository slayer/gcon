package commandpalette

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRecentTracker(t *testing.T) {
	tracker := NewRecentTracker()

	assert.NotNil(t, tracker)
	assert.Equal(t, 0, len(tracker.Items()))
}

func TestTrack(t *testing.T) {
	tracker := NewRecentTracker()

	tracker.Track(RecentTypeProject, "proj-1", "Project One")

	items := tracker.Items()
	assert.Equal(t, 1, len(items))
	assert.Equal(t, RecentTypeProject, items[0].Type)
	assert.Equal(t, "proj-1", items[0].ID)
	assert.Equal(t, "Project One", items[0].Label)
}

func TestTrackMostRecentFirst(t *testing.T) {
	tracker := NewRecentTracker()

	tracker.Track(RecentTypeProject, "proj-1", "Project One")
	tracker.Track(RecentTypeBucket, "bucket-1", "Bucket One")
	tracker.Track(RecentTypeInstance, "inst-1", "Instance One")

	items := tracker.Items()
	assert.Equal(t, 3, len(items))
	// Most recent should be first
	assert.Equal(t, "inst-1", items[0].ID)
	assert.Equal(t, "bucket-1", items[1].ID)
	assert.Equal(t, "proj-1", items[2].ID)
}

func TestTrackDuplicateMovesToTop(t *testing.T) {
	tracker := NewRecentTracker()

	tracker.Track(RecentTypeProject, "proj-1", "Project One")
	tracker.Track(RecentTypeBucket, "bucket-1", "Bucket One")
	tracker.Track(RecentTypeInstance, "inst-1", "Instance One")

	// Track proj-1 again - should move to top
	tracker.Track(RecentTypeProject, "proj-1", "Project One Updated")

	items := tracker.Items()
	assert.Equal(t, 3, len(items))
	assert.Equal(t, "proj-1", items[0].ID)
	assert.Equal(t, "Project One Updated", items[0].Label) // Label should be updated
	assert.Equal(t, "inst-1", items[1].ID)
	assert.Equal(t, "bucket-1", items[2].ID)
}

func TestTrackMaxItems(t *testing.T) {
	tracker := NewRecentTracker()
	tracker.SetMaxItems(3)

	tracker.Track(RecentTypeProject, "proj-1", "Project One")
	tracker.Track(RecentTypeProject, "proj-2", "Project Two")
	tracker.Track(RecentTypeProject, "proj-3", "Project Three")
	tracker.Track(RecentTypeProject, "proj-4", "Project Four")

	items := tracker.Items()
	assert.Equal(t, 3, len(items))
	// Oldest should be dropped
	assert.Equal(t, "proj-4", items[0].ID)
	assert.Equal(t, "proj-3", items[1].ID)
	assert.Equal(t, "proj-2", items[2].ID)
}

func TestCommands(t *testing.T) {
	tracker := NewRecentTracker()

	tracker.Track(RecentTypeProject, "proj-1", "Project One")
	tracker.Track(RecentTypeBucket, "bucket-1", "Bucket One")

	commands := tracker.Commands()
	assert.Equal(t, 2, len(commands))

	// Check first command (most recent - bucket)
	assert.Equal(t, "recent:bucket:bucket-1", commands[0].ID)
	assert.Equal(t, "Recent: Bucket One", commands[0].Label)
	assert.Equal(t, IconRecent, commands[0].Icon)
	assert.Equal(t, CommandTypeRecent, commands[0].Type)
	assert.True(t, commands[0].Enabled)
	assert.Equal(t, ViewBuckets, commands[0].ViewType)

	// Check second command (project)
	assert.Equal(t, "recent:project:proj-1", commands[1].ID)
}

func TestClear(t *testing.T) {
	tracker := NewRecentTracker()

	tracker.Track(RecentTypeProject, "proj-1", "Project One")
	tracker.Track(RecentTypeBucket, "bucket-1", "Bucket One")

	tracker.Clear()

	assert.Equal(t, 0, len(tracker.Items()))
}

func TestSetMaxItems(t *testing.T) {
	tracker := NewRecentTracker()

	// Add 5 items
	for i := 1; i <= 5; i++ {
		tracker.Track(RecentTypeProject, "proj-"+string(rune('0'+i)), "Project")
	}

	assert.Equal(t, 5, len(tracker.Items()))

	// Reduce max to 2
	tracker.SetMaxItems(2)

	items := tracker.Items()
	assert.Equal(t, 2, len(items))
	// Should keep most recent
	assert.Equal(t, "proj-5", items[0].ID)
	assert.Equal(t, "proj-4", items[1].ID)
}

func TestItemsConcurrency(t *testing.T) {
	tracker := NewRecentTracker()

	// Test that Items returns a copy, not a reference
	tracker.Track(RecentTypeProject, "proj-1", "Project One")

	items1 := tracker.Items()
	items2 := tracker.Items()

	// Modify items1
	items1[0].Label = "Modified"

	// items2 should be unaffected
	assert.Equal(t, "Project One", items2[0].Label)
}
