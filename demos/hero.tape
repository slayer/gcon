# Hero GIF — Main showcase of gcon
# Shows: project selection, sidebar navigation, instance details, command palette
Output output/hero.gif

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

# Project list loaded — select a project
Down
Sleep 500ms
Enter
Sleep 2s

# Instances list loaded — browse
Down
Sleep 500ms
Down
Sleep 500ms

# Open instance details
Enter
Sleep 2s

# Scroll through details
Down
Down
Down
Sleep 1s

# Go back to list
Escape
Sleep 1s

# Focus sidebar
Type "["
Sleep 1s

# Navigate sidebar — browse resource categories
Down
Sleep 500ms
Down
Sleep 500ms
Down
Sleep 500ms
Down
Sleep 500ms
Down
Sleep 500ms

# Select a sidebar item (e.g., Cloud Storage)
Enter
Sleep 2s

# Return focus to content
Type "]"
Sleep 1s

# Open command palette
Type ":"
Sleep 500ms

# Search for logs
Type "logs"
Sleep 500ms
Enter
Sleep 2s

# Logs Explorer loaded — pause to show
Sleep 2s
