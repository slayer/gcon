# Disk Details Feature Documentation

## Summary of Changes

Added a detail view for Compute Engine disks. Users can now press Enter on a disk in the disks list to view comprehensive information about the disk including size, type, encryption, source image, and which instances it's attached to.

## Technical Details

### Files Modified

1. **internal/gcp/compute.go**
   - Added `DiskDetails` struct with comprehensive fields:
     - Basic info: Name, ID, Description, Status, Zone, CreatedAt, LastAttach, LastDetach, Labels
     - Size/Performance: SizeGB, Type, ProvisionedIOPS, ProvisionedTPUT, PhysicalBlockSizeB
     - Source: SourceImage, SourceSnapshot, SourceDisk
     - Encryption: DiskEncryptionKey (Google-managed, CMEK, CSEK)
     - Usage: Users (attached instances), ReplicaZones
   - Added `GetDiskDetails()` method to fetch disk via GCP API
   - Added `diskDetailsFromAPI()` helper to convert API response

2. **internal/ui/views/disk_details.go** (new file)
   - Created `DiskDetailsView` following the instance_details.go pattern
   - Uses viewport for scrollable content
   - Displays all disk information in organized sections
   - Exports `DiskSelectedMsg` for navigation

3. **internal/ui/views/disks.go**
   - Added Enter key binding to navigate to disk details
   - Added `findDiskByName()` helper
   - Added `GetComputeClient()` for client reuse
   - Updated help text to show Enter key

4. **internal/ui/app.go**
   - Added `ViewDiskDetails` view type
   - Added `diskDetailsView` field and `selectedDisk` context
   - Wired up navigation from disks list to details
   - Added back navigation from details to list
   - Added disk name to header breadcrumb

### Files Added

- `internal/ui/views/disk_details.go` - DiskDetailsView implementation
- `internal/ui/views/disk_details_test.go` - Unit tests
- `internal/gcp/compute_test.go` - Added `TestDiskDetailsFromAPI`

## Usage

1. Navigate to "Compute Engine" > "Disks" from the sidebar
2. Use `j/k` or arrows to select a disk
3. Press `Enter` to view disk details
4. Use `j/k` or arrows to scroll through details
5. Press `r` to refresh the disk information
6. Press `esc` to go back to the disks list

## Detail View Sections

The disk details view displays the following sections:

1. **Basic Information**
   - Name, ID, Description, Status, Zone
   - Creation timestamp
   - Last attach/detach timestamps

2. **Size and Performance**
   - Size in GB
   - Disk type (human-readable name)
   - Provisioned IOPS (for pd-extreme)
   - Provisioned throughput
   - Physical block size

3. **Source**
   - Source image (if created from image)
   - Source snapshot (if created from snapshot)
   - Source disk (if cloned)
   - Shows "Blank disk" if no source

4. **Encryption**
   - Google-managed (default)
   - Customer-managed (CMEK)
   - Customer-supplied (CSEK)

5. **Usage**
   - Attached instances
   - Replica zones (for regional disks)

## Key Bindings

| Key | Action |
|-----|--------|
| `↑/k` | Scroll up |
| `↓/j` | Scroll down |
| `r` | Refresh details |
| `esc` | Go back to list |
