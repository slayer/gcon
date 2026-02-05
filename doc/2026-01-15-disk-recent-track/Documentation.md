# Disk Recent Tracking Feature Documentation

## Summary of Changes

Integrated disk details with the command palette's recent item tracking system. When a user views disk details, the disk now appears in the "Recent" section of the command palette for quick navigation.

## Technical Details

### Files Modified

1. **internal/ui/components/commandpalette/recent.go**
   - Added `RecentTypeDisk` constant to track recently viewed disks
   - Added case for `RecentTypeDisk` in `Commands()` switch statement to set `ViewType = ViewDisks`

2. **internal/ui/app.go**
   - Added `recentTracker.Track()` call when `DiskSelectedMsg` is handled
   - Added `case "disk"` in `handleRecentCommand()` to navigate to disks view

## Usage

1. Navigate to "Compute Engine" > "Disks" from the sidebar
2. Select a disk and press `Enter` to view details
3. The disk is now tracked in recent items
4. Press `:` or `Ctrl+K` to open command palette
5. Recent disks appear in the "Recent" section
6. Select a recent disk to navigate back to the Disks view

## How Recent Tracking Works

The command palette maintains an in-memory list of recently accessed resources:
- Projects, Buckets, Instances, and now Disks
- Maximum 5 recent items tracked
- Most recently accessed items appear first
- Selecting a recent item navigates to the corresponding view
