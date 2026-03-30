# Observability GIF — Real-time metrics and charts
# Shows: instance observability tab, CPU/memory charts, time range
Output output/observability.gif

Set Theme "Catppuccin Mocha"
Set FontSize 14
Set Width 1400
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

# Open first instance details (tabs focused by default)
Enter
Sleep 2s

# Switch to Observability tab — "2" selects 2nd tab when tabs focused
Type "2"
Sleep 3s

# Metrics loaded — pause to show charts
Sleep 2s

# Focus content area for time range keys
Tab
Sleep 500ms

# Change time range to 6 hours
Type "2"
Sleep 2s

# Change time range to 24 hours
Type "3"
Sleep 2s

# Toggle auto-refresh
Type "a"
Sleep 1s

# Pause on final state
Sleep 2s
