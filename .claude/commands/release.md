# Create Release

Create a new version tag and push it to trigger the release workflow.

## Arguments

- `$ARGUMENTS` - Version number (e.g., "0.1.0" or "v0.1.0")

## Steps

1. Validate the version argument is provided
2. Normalize version format (ensure it starts with "v")
3. Check that we're on the master branch
4. Check that there are no uncommitted changes
5. Fetch latest tags and verify the version doesn't already exist
6. Create an annotated tag with the version
7. Push the tag to origin
8. Report success with link to the GitHub Actions workflow
