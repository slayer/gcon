# Command Palette GIF — Quick navigation
# Shows: fuzzy search, navigating between features
Output output/command-palette.gif

Set Theme "Catppuccin Mocha"
Set FontSize 14
Set Width 1200
Set Height 700
Set Padding 20
# Set up auth (hidden — gcon takes over the screen immediately)
Hide
Type "export GOOGLE_APPLICATION_CREDENTIALS=gcon-sa.json CLOUDSDK_CONFIG=/tmp/gcon-demo-gcloud"
Enter
Type "clear"
Enter
Sleep 500ms
Show

# Launch gcon
Type "gcon"
Enter
Sleep 3s

# Select project
Down
Sleep 500ms
Enter
Sleep 2s

# Open command palette
Type ":"
Sleep 500ms

# Search for Cloud Run
Type "cloud run"
Sleep 1s
Enter
Sleep 2s

# Cloud Run services loaded — pause to show
Sleep 1s

# Open command palette again
Type ":"
Sleep 500ms

# Search for buckets
Type "buckets"
Sleep 1s
Enter
Sleep 2s

# Buckets loaded — pause to show
Sleep 1s

# One more navigation — to networks
Type ":"
Sleep 500ms
Type "networks"
Sleep 1s
Enter
Sleep 2s

# Pause on final state
Sleep 1s
