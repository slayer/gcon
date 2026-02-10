---
description: Keyboard shortcuts reference for all views
---

# Key Bindings

## Global

| Key | Action |
|-----|--------|
| `q`, `Ctrl+C` | Quit |
| `?` | Toggle help |
| `Esc` | Go back |
| `j/k` or `↓/↑` | Navigate list |
| `r` | Refresh current view |
| `/` | Search/Filter |
| `Enter` | Select/Confirm |
| `:` or `Ctrl+K` | Open command palette |
| `[` | Focus sidebar (expands if auto-hidden) |
| `]` | Focus content (collapses sidebar if auto-hide) |
| `{` | Pin/unpin sidebar (toggle auto-hide/always-open) |

## Command Palette

| Command | Description |
|---------|-------------|
| Switch Project | Open project selector modal to switch between GCP projects |
| Refresh | Refresh current view |
| Toggle sidebar | Pin/unpin sidebar (auto-hide/always-open) |
| Help | Toggle help display |
| Quit | Exit application |

## Project Selector Modal

| Key | Action |
|-----|--------|
| `↑`/`k` or `Ctrl-P` | Move up |
| `↓`/`j` or `Ctrl-N` | Move down |
| `Enter` | Select project |
| `Esc` | Cancel |
| Type to filter | Filter projects by name or ID |
| `r` | Retry loading (if error) |

## Instances View

| Key | Action |
|-----|--------|
| `Enter` | View instance details |
| `.` | Open action menu |
| `s` | Start stopped instance |
| `x` | Stop running instance |
| `z` | Suspend running instance |
| `Z` | Resume suspended instance |
| `R` | Reset (hard reboot) |
| `D` | Delete instance (with type-to-confirm) |
| `/` | Filter instances |
| `r` | Refresh list |
| `Esc` | Go back |

## Instance Details

| Key | Action |
|-----|--------|
| `.` | Open action menu |
| `l` | Edit labels |
| `s` | Start instance (if stopped) |
| `x` | Stop instance (if running) |
| `z` | Suspend instance (if running) |
| `Z` | Resume instance (if suspended) |
| `R` | Reset instance |
| `D` | Delete instance (with type-to-confirm) |
| `r` | Refresh details |
| `↑/↓` | Scroll content |
| `Tab` | Switch focus (tabs/links/content) |
| `Esc` | Go back |

## Label Editor

| Key | Action |
|-----|--------|
| `↑/k` | Move up |
| `↓/j` | Move down |
| `a` | Add new label |
| `e`/`Enter` | Edit selected label |
| `x`/`Delete` | Delete label |
| `Tab` | Switch key/value input |
| `Ctrl+S` | Preview changes |
| `Esc` | Back (global: navigate to previous view) |

## Instance Details - Observability Tab

| Key | Action |
|-----|--------|
| `1` | Set time range to 1 hour |
| `2` | Set time range to 6 hours |
| `3` | Set time range to 24 hours |
| `4` | Set time range to 7 days |
| `5` | Set time range to 30 days |
| `a` | Toggle auto-refresh (30s interval, on by default) |
| `r` | Manual refresh metrics |

## Disks View

| Key | Action |
|-----|--------|
| `Enter` | View disk details |
| `.` | Open action menu |
| `s` | Create snapshot from disk |
| `i` | Create image from disk |
| `D` | Delete disk (if detached) |
| `/` | Filter disks |
| `r` | Refresh list |
| `Esc` | Go back |

## Disk Details View

| Key | Action |
|-----|--------|
| `.` | Open action menu |
| `s` | Create snapshot from disk |
| `i` | Create image from disk |
| `D` | Delete disk (if detached) |
| `r` | Refresh details |
| `↑/↓` | Scroll content |
| `Esc` | Go back |

## Snapshots View

| Key | Action |
|-----|--------|
| `Enter` | View snapshot details |
| `.` | Open action menu |
| `c` | Create disk from snapshot |
| `i` | Create image from snapshot |
| `D` | Delete snapshot (with confirmation) |
| `/` | Filter snapshots |
| `r` | Refresh list |
| `Esc` | Go back |

## Snapshot Details View

| Key | Action |
|-----|--------|
| `.` | Open action menu |
| `c` | Create disk from snapshot |
| `i` | Create image from snapshot |
| `D` | Delete snapshot (with confirmation) |
| `r` | Refresh details |
| `↑/↓` | Scroll content |
| `Tab` | Switch focus (links/content) |
| `Enter` | Navigate to source disk (when link focused) |
| `Esc` | Go back |

## Buckets View

| Key | Action |
|-----|--------|
| `Enter` | Browse bucket contents |
| `c` | Create new bucket |
| `/` | Filter buckets |
| `r` | Refresh list |
| `Esc` | Go back |

## Images View

| Key | Action |
|-----|--------|
| `Enter` | View image details |
| `.` | Open action menu |
| `c` | Create disk from image |
| `D` | Delete image (with confirmation) |
| `/` | Filter images |
| `r` | Refresh list |
| `Esc` | Go back |

## Image Details View

| Key | Action |
|-----|--------|
| `.` | Open action menu |
| `c` | Create disk from image |
| `D` | Delete image (with confirmation) |
| `r` | Refresh details |
| `↑/↓` | Scroll content |
| `Esc` | Go back |

## Objects View (GCS Browser)

| Key | Action |
|-----|--------|
| `Enter` | Open folder / View file details |
| `d` | Download file/folder |
| `u` | Upload files |
| `D` | Delete file/folder (with confirmation) |
| `.` | Open action menu |
| `n` | Next page |
| `p` | Previous page |

## Networks View

| Key | Action |
|-----|--------|
| `Enter` | View network details (future) |
| `/` | Filter networks |
| `r` | Refresh list |
| `Esc` | Go back |

## Object Details View

| Key | Action |
|-----|--------|
| `v` | Preview text file (< 500KB) |
| `o` | Download and open with default app |
| `d` | Download to current directory |
| `D` | Delete (with confirmation) |
| `.` | Open action menu |
| `r` | Refresh metadata |
| `Tab` | Switch between tabs and content |
| `h/l` or `1-2` | Switch tabs (Details/Preview) |
| `Esc` | Back to objects list |
