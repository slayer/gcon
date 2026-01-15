# Disk Recent Tracking Feature

## Task Description
Integrate disk details with the command palette's recent item tracking system. When a user views disk details, the disk should appear in the "Recent" section of the command palette for quick navigation.

## Implementation Plan

### 1. Recent Tracker Integration
- [x] Add `RecentTypeDisk` constant to `recent.go`
- [x] Handle disk type in `Commands()` switch statement to set `ViewType`

### 2. App Integration
- [x] Track disk access in `app.go` when `DiskSelectedMsg` is handled
- [x] Handle recent disk command in `handleRecentCommand()`

### 3. Testing
- [x] Run existing tests to verify no regressions
- [x] Run linter

## Requirements
- Recently viewed disks appear in command palette's Recent section
- Selecting a recent disk navigates to the Disks view
- Follow existing patterns from instance/bucket recent tracking
