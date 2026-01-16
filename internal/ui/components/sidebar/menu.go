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
	ViewSnapshots
	ViewImages
	ViewProjectMetadata
	ViewBuckets
	ViewNetworks
	ViewFirewall
)

// Icons for menu items - using simple Unicode box/geometric symbols
// All from the same Unicode block for consistent single-width rendering
const (
	// Category icons (hollow shapes)
	IconCompute = "□" // Compute Engine
	IconStorage = "○" // Cloud Storage
	IconNetwork = "◇" // VPC Network

	// Leaf item icons (filled shapes)
	IconVM       = "■" // VM instances
	IconDisk     = "●" // Disks
	IconImage    = "◉" // Images
	IconMetadata = "◐" // Metadata
	IconBucket   = "▪" // Buckets
	IconVPC      = "◆" // VPC networks
	IconFirewall = "▲" // Firewall
)

// MenuItem represents a single menu entry
type MenuItem struct {
	ID       string       // Unique identifier (e.g., "compute", "vm-instances")
	Label    string       // Full text display (e.g., "VM instances")
	Icon     string       // Icon for display
	Hotkey   rune         // Keyboard shortcut (case-sensitive, e.g., 'c' for Compute)
	Type     MenuItemType // Category or Leaf
	ViewType ViewType     // Target view for leaf items
	Children []MenuItem   // Child items for categories
}

// DefaultMenu returns the GCP Console-like menu structure
// Hotkeys are case-sensitive: 'c' for Compute, 'V' for VPC Network
func DefaultMenu() []MenuItem {
	return []MenuItem{
		{
			ID:     "compute",
			Label:  "Compute Engine",
			Icon:   IconCompute,
			Hotkey: 'c',
			Type:   MenuItemCategory,
			Children: []MenuItem{
				{ID: "vm-instances", Label: "VM instances", Icon: IconVM, Hotkey: 'v', Type: MenuItemLeaf, ViewType: ViewInstances},
				{ID: "disks", Label: "Disks", Icon: IconDisk, Hotkey: 'd', Type: MenuItemLeaf, ViewType: ViewDisks},
				{ID: "snapshots", Label: "Snapshots", Icon: IconDisk, Hotkey: 'S', Type: MenuItemLeaf, ViewType: ViewSnapshots},
				{ID: "images", Label: "Images", Icon: IconImage, Hotkey: 'I', Type: MenuItemLeaf, ViewType: ViewImages},
				{ID: "metadata", Label: "Metadata", Icon: IconMetadata, Hotkey: 'M', Type: MenuItemLeaf, ViewType: ViewProjectMetadata},
			},
		},
		{
			ID:     "storage",
			Label:  "Cloud Storage",
			Icon:   IconStorage,
			Hotkey: 's',
			Type:   MenuItemCategory,
			Children: []MenuItem{
				{ID: "buckets", Label: "Buckets", Icon: IconBucket, Hotkey: 'b', Type: MenuItemLeaf, ViewType: ViewBuckets},
			},
		},
		{
			ID:     "networking",
			Label:  "VPC Network",
			Icon:   IconNetwork,
			Hotkey: 'V',
			Type:   MenuItemCategory,
			Children: []MenuItem{
				{ID: "networks", Label: "VPC networks", Icon: IconVPC, Hotkey: 'n', Type: MenuItemLeaf, ViewType: ViewNetworks},
				{ID: "firewall", Label: "Firewall", Icon: IconFirewall, Hotkey: 'f', Type: MenuItemLeaf, ViewType: ViewFirewall},
			},
		},
	}
}
