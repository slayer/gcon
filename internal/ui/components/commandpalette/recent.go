package commandpalette

import (
	"sync"
	"time"
)

const (
	// Default maximum number of recent items to track
	defaultMaxRecentItems = 5
)

// RecentItemType identifies what kind of resource was accessed
type RecentItemType string

const (
	RecentTypeProject  RecentItemType = "project"
	RecentTypeBucket   RecentItemType = "bucket"
	RecentTypeInstance RecentItemType = "instance"
	RecentTypeDisk     RecentItemType = "disk"
	RecentTypeSnapshot RecentItemType = "snapshot"
	RecentTypeImage    RecentItemType = "image"
)

// RecentItem represents a recently accessed resource
type RecentItem struct {
	Type      RecentItemType
	ID        string    // Resource ID
	Label     string    // Display name
	Timestamp time.Time // When it was accessed
}

// RecentTracker manages recently accessed items (in-memory)
type RecentTracker struct {
	mu       sync.RWMutex
	items    []RecentItem
	maxItems int
}

// NewRecentTracker creates a new recent items tracker
func NewRecentTracker() *RecentTracker {
	return &RecentTracker{
		items:    make([]RecentItem, 0, defaultMaxRecentItems),
		maxItems: defaultMaxRecentItems,
	}
}

// Track adds or updates a recent item. If the item already exists, it moves to the top.
func (r *RecentTracker) Track(itemType RecentItemType, id, label string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove existing entry if present
	for i, item := range r.items {
		if item.Type == itemType && item.ID == id {
			r.items = append(r.items[:i], r.items[i+1:]...)
			break
		}
	}

	// Add new entry at the beginning (most recent first)
	newItem := RecentItem{
		Type:      itemType,
		ID:        id,
		Label:     label,
		Timestamp: time.Now(),
	}
	r.items = append([]RecentItem{newItem}, r.items...)

	// Trim to max size
	if len(r.items) > r.maxItems {
		r.items = r.items[:r.maxItems]
	}
}

// Items returns a copy of the recent items (most recent first)
func (r *RecentTracker) Items() []RecentItem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]RecentItem, len(r.items))
	copy(result, r.items)
	return result
}

// Commands converts recent items to Command entries for the palette
func (r *RecentTracker) Commands() []Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	commands := make([]Command, 0, len(r.items))
	for _, item := range r.items {
		cmd := Command{
			ID:      "recent:" + string(item.Type) + ":" + item.ID,
			Label:   "Recent: " + item.Label,
			Icon:    IconRecent,
			Type:    CommandTypeRecent,
			Enabled: true,
		}

		// Set ViewType for navigation commands
		switch item.Type {
		case RecentTypeBucket:
			cmd.ViewType = ViewBuckets
		case RecentTypeInstance:
			cmd.ViewType = ViewInstances
		case RecentTypeDisk:
			cmd.ViewType = ViewDisks
		case RecentTypeSnapshot:
			cmd.ViewType = ViewSnapshots
		case RecentTypeImage:
			cmd.ViewType = ViewImages
		case RecentTypeProject:
			// Projects navigate back to project list, no ViewType needed
		}

		commands = append(commands, cmd)
	}

	return commands
}

// Clear removes all recent items
func (r *RecentTracker) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = r.items[:0]
}

// SetMaxItems sets the maximum number of recent items to track
func (r *RecentTracker) SetMaxItems(maxItems int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.maxItems = maxItems
	if len(r.items) > maxItems {
		r.items = r.items[:maxItems]
	}
}
