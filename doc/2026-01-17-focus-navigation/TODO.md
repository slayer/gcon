# Universal Focus & Navigation System

## Task Description

Implement a focus management system to resolve key conflicts between multiple interactive components (viewport scrolling, link navigation, tab switching) in detail views.

## Problem

In instance details view, j/k keys are captured by the links component when available, preventing viewport scrolling. No unified focus management exists.

## Implementation Plan

### Phase 1: Create Focus Package
- [x] `internal/ui/focus/region.go` - RegionType enum and Region struct
- [x] `internal/ui/focus/manager.go` - FocusManager implementation
- [x] `internal/ui/focus/messages.go` - FocusChangedMsg
- [x] `internal/ui/focus/help.go` - Context-sensitive help bindings

### Phase 2: Integrate into Instance Details
- [x] Add focusMgr field to InstanceDetailsView
- [x] Configure regions on data load
- [x] Update key routing to use focus manager
- [x] Update help text based on active region

### Phase 3: Update Footer
- [x] Show context-sensitive hints based on focused region

### Phase 4: Testing & Cleanup
- [x] Write tests for focus package
- [x] Run full test suite
- [x] Run linter
- [x] Create documentation

## Key Design Decisions

1. Tab key cycles between focusable regions within content area
2. Regions are dynamic based on view content
3. Visual indicator shows which region has focus
4. Incremental adoption - views without multiple regions don't need FocusManager
