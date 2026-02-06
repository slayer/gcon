package views

import (
	"github.com/slayer/gcon/internal/ui/components"
)

// Compile-time interface checks: all list views satisfy Clickable via TableClickDelegate.
var (
	_ components.Clickable = (*ProjectsView)(nil)
	_ components.Clickable = (*InstancesView)(nil)
	_ components.Clickable = (*DisksView)(nil)
	_ components.Clickable = (*SnapshotsView)(nil)
	_ components.Clickable = (*ImagesView)(nil)
	_ components.Clickable = (*BucketsView)(nil)
	_ components.Clickable = (*ObjectsView)(nil)
)
