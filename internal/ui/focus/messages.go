package focus

// FocusChangedMsg is emitted when focus moves to a different region.
// Views can use this to update visual indicators or help text.
type FocusChangedMsg struct {
	// FromRegion is the previously focused region (nil if first focus)
	FromRegion *Region
	// ToRegion is the newly focused region
	ToRegion *Region
}
