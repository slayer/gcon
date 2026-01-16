# TODO: Make Source Disk in Snapshot Details a Navigable Link

## Task Description

In the disk snapshot details view, the "Source Disk" field should be a link. When the user focuses on this link and presses Enter, the application should navigate to the details view for that source disk. Pressing `Esc` from the disk details view should return the user to the snapshot details view.

## Implementation Plan

1.  **Update `snapshot_details.go`:**
    *   Modify `SnapshotDetailsView` to manage focus on the "Source Disk" link.
    *   Update the `Update` method to handle the `tea.KeyEnter` message. When triggered, it should emit a new message (e.g., `DiskSelectedMsg`) with the source disk's name and zone.
    *   Update the `View` method to style the "Source Disk" field as a link when it is in focus.

2.  **Update `app.go`:**
    *   Modify the main `Update` method in `app.go` to handle the new `DiskSelectedMsg`.
    *   Upon receiving the message, the app should:
        *   Switch the `currentView` to the disk details view.
        *   Pass the required context (Project ID, Disk Name, Zone) to the disk details view.
        *   The existing navigation stack should handle the `Esc` key behavior automatically.

3.  **Verify GCP Data (`internal/gcp/compute.go`):**
    *   Ensure that the `compute.Snapshot` struct returned by the GCP client contains the necessary `SourceDisk` information (name and zone). It is expected to be present.

4.  **Testing:**
    *   Add or update unit tests in `snapshot_details_test.go` to verify:
        *   The view correctly renders the source disk link.
        *   Pressing Enter on the link sends the correct `DiskSelectedMsg`.
    *   Consider adding an integration test in `app_test.go` to verify the end-to-end navigation flow if existing patterns support it.
