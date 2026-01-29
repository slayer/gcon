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

## Command Palette

| Command | Description |
|---------|-------------|
| Switch Project | Open project selector modal to switch between GCP projects |
| Refresh | Refresh current view |
| Toggle sidebar | Show/hide sidebar navigation |
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
| `s` | Start stopped instance |
| `x` | Stop running instance |
| `R` | Reset (hard reboot) |

## Instance Details

| Key | Action |
|-----|--------|
| `.` | Open action menu |
| `l` | Edit labels |
| `s` | Start instance (if stopped) |
| `x` | Stop instance (if running) |
| `R` | Reset instance |
| `r` | Refresh details |

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
| `/` | Filter images |
| `r` | Refresh list |

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
