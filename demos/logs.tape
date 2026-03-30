# Logs Explorer GIF — Log querying and filtering
# Shows: log entries, expansion, severity filter, colorization
Output output/logs.gif

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

# Open command palette and navigate to Logs Explorer
Type ":"
Sleep 300ms
Type "logs"
Sleep 300ms
Enter
Sleep 3s

# Log entries loaded — browse
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms

# Expand a log entry
Enter
Sleep 2s

# Scroll through expanded entry fields
Down
Down
Sleep 1s

# Collapse entry
Left
Sleep 1s

# Toggle WARNING severity filter
Type "W"
Sleep 2s

# Toggle colorization
Type "c"
Sleep 2s

# Pause on final state
Sleep 1s
