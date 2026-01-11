package sidebar

// MenuItemType distinguishes between category headers and leaf items
type MenuItemType int

const (
	MenuItemCategory MenuItemType = iota // Has children, drill-down expands
	MenuItemLeaf                         // Navigates to a view
)

// ViewType mirrors ui.ViewType to avoid import cycle
// App will map these to actual ViewType values
type ViewType int

const (
	ViewInstances ViewType = iota
	ViewDisks
	ViewBuckets
	ViewNetworks
	ViewFirewall
)

// MenuItem represents a single menu entry
type MenuItem struct {
	ID       string       // Unique identifier (e.g., "compute", "vm-instances")
	Label    string       // Full text display (e.g., "VM instances")
	Icon     string       // Icon for collapsed mode
	Type     MenuItemType // Category or Leaf
	ViewType ViewType     // Target view for leaf items
	Children []MenuItem   // Child items for categories
}

// DefaultMenu returns the GCP Console-like menu structure
func DefaultMenu() []MenuItem {
	return []MenuItem{
		{
			ID:    "compute",
			Label: "Compute Engine",
			Icon:  "",
			Type:  MenuItemCategory,
			Children: []MenuItem{
				{ID: "vm-instances", Label: "VM instances", Icon: "", Type: MenuItemLeaf, ViewType: ViewInstances},
				{ID: "disks", Label: "Disks", Icon: "", Type: MenuItemLeaf, ViewType: ViewDisks},
			},
		},
		{
			ID:    "storage",
			Label: "Cloud Storage",
			Icon:  "",
			Type:  MenuItemCategory,
			Children: []MenuItem{
				{ID: "buckets", Label: "Buckets", Icon: "", Type: MenuItemLeaf, ViewType: ViewBuckets},
			},
		},
		{
			ID:    "networking",
			Label: "VPC Network",
			Icon:  "",
			Type:  MenuItemCategory,
			Children: []MenuItem{
				{ID: "networks", Label: "VPC networks", Icon: "", Type: MenuItemLeaf, ViewType: ViewNetworks},
				{ID: "firewall", Label: "Firewall", Icon: "", Type: MenuItemLeaf, ViewType: ViewFirewall},
			},
		},
	}
}
