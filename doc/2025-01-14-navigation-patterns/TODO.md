# Navigation Patterns Design Task

## Task Description
Design comprehensive navigation patterns, common hotkeys, and design requirements for current and future views in gcon TUI application.

## Status: Partially Implemented

## Deliverables - Design Phase

- [x] Key binding taxonomy (global, navigation, action keys)
- [x] View type patterns (simple list, list with actions, hierarchical, detail, filter/query)
- [x] State machine diagrams (mermaid)
- [x] Focus management design
- [x] Modal interaction patterns
- [x] Help system design
- [x] Future view recommendations (Logging, GKE, Cloud Run, IAM, Disks, Networks)
- [x] Implementation architecture (registry, focus manager, key routing)
- [x] Implementation timeline (6 phases)
- [x] Gap analysis

## Deliverables - Implementation Phase

### Sidebar Hotkeys
- [x] Add `Hotkey rune` field to MenuItem struct
- [x] Implement case-sensitive hotkey handling
- [x] Add hotkey highlighting in sidebar labels
- [x] Hotkey assignments: c=Compute, s=Storage, V=VPC, v=VM, d=Disks, b=Buckets, n=Networks, f=Firewall

### Action Menu Component
- [x] Create `internal/ui/components/actionmenu/actionmenu.go` - Main component
- [x] Create `internal/ui/components/actionmenu/styles.go` - Lipgloss styles
- [x] Create `internal/ui/components/actionmenu/actionmenu_test.go` - Unit tests
- [x] Integrate with `instances.go` (VM list view)
- [x] Integrate with `instance_details.go` (VM details view)
- [x] Add `.` key binding to open action menu
- [x] Support j/k navigation, direct hotkey selection, Enter/Esc

### Instance View Enhancements
- [x] Widen status column by +2 characters

## Key Design Decisions

1. **Case sensitivity convention**: lowercase = safe, uppercase = destructive
2. **Vim-style navigation**: j/k/h/l for movement
3. **Focus management**: 4 states (Content, Sidebar, Modal, Filter)
4. **Key routing**: Modal > Global > Focus-specific
5. **HelpProvider interface**: Views implement ContextHelp() and FullHelp()
6. **Action menu**: Modal popup triggered by `.` key

## Files Created

- `doc/2025-01-14-navigation-patterns/navigation.md` - Full design document
- `doc/2025-01-14-navigation-patterns/TODO.md` - This file
- `internal/ui/components/actionmenu/actionmenu.go` - Action menu component
- `internal/ui/components/actionmenu/styles.go` - Action menu styles
- `internal/ui/components/actionmenu/actionmenu_test.go` - Action menu tests

## Files Modified

- `internal/ui/components/sidebar/menu.go` - Added Hotkey field to MenuItem
- `internal/ui/components/sidebar/sidebar.go` - Added hotkey handling and highlighting
- `internal/ui/components/sidebar/styles.go` - Added Hotkey style
- `internal/ui/views/instances.go` - Integrated action menu, widened status column
- `internal/ui/views/instance_details.go` - Integrated action menu
- `internal/ui/keys.go` - Added ActionMenu key binding

## Next Steps (Remaining Implementation)

See Section 9 "Implementation Timeline" in navigation.md for full task breakdown.

Remaining phases:
- Phase 1: Keybindings Package (centralize key definitions)
- Phase 2: Focus Manager (formal focus state management)
- Phase 3: Help System (unified, contextual help)
- Phase 4: Modal Standardization
- Phase 5: Visual Focus Indicators
- Phase 6: Documentation
