# Table Views Implementation

## Task Description
Convert list-based views to table-based views using `bubbles/table` component for better columnar data display.

## Implementation Plan

### Step 1: Create Reusable Table Component
- [x] Create `internal/ui/components/table/table.go` - Table wrapper with GCP styling
- [x] Create `internal/ui/components/table/styles.go` - Consistent styling
- [x] Add filtering support (since bubbles/table lacks built-in filtering)

### Step 2: Convert Instances View
- [x] Replace `bubbles/list` with table component
- [x] Columns: Name, Status, Zone, Internal IP, External IP, Machine Type
- [x] Preserve keybindings: s=start, x=stop, R=reset, r=refresh, Enter=details
- [x] Keep action feedback messages

### Step 3: Convert Projects View
- [x] Replace `bubbles/list` with table component
- [x] Columns: Name, Project ID, State
- [x] Implement filtering with `/` key

### Step 4: Convert Buckets View
- [x] Replace `bubbles/list` with table component
- [x] Columns: Name, Location, Storage Class, Created

### Step 5: Convert Objects View
- [x] Replace `bubbles/list` with table component
- [x] Columns: Name, Size, Content Type, Modified
- [x] Keep folder navigation intact

### Step 6: Testing & Verification
- [x] Run `make test`
- [x] Run `make lint`
- [x] Manual testing pending (build verified)

## Technical Notes

- Using official `charmbracelet/bubbles/table` (not gitkraken fork)
- Custom filtering implementation with textinput component
- GCP styling: Blue #4285F4 for selection, Gray #9AA0A6 for borders
