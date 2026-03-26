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
| `/` | Search/Filter (supports `field:value` syntax) |
| `S` | Open sort menu (list views) |
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
| `S` | Open sort menu |
| `c` | Create new instance |
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
| `e` | Edit instance configuration |
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

## Instance Create/Edit View

| Key | Action |
|-----|--------|
| `Ctrl+S` | Submit / Preview changes (show diff) |
| `Enter` | Confirm deploy (from diff view) |
| `Tab/↓` | Next field |
| `Shift+Tab/↑` | Previous field |
| `Esc` | Back (diff → form) / Cancel (form → back) |

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
| `S` | Open sort menu |
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
| `S` | Open sort menu |
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
| `S` | Open sort menu |
| `c` | Create new bucket |
| `/` | Filter buckets |
| `r` | Refresh list |
| `Esc` | Go back |

## Images View

| Key | Action |
|-----|--------|
| `Enter` | View image details |
| `.` | Open action menu |
| `S` | Open sort menu |
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
| `/` | Filter objects |
| `r` | Refresh list |
| `Esc` | Go back |

## Networks View

| Key | Action |
|-----|--------|
| `Enter` | View network details |
| `S` | Open sort menu |
| `/` | Filter networks |
| `r` | Refresh list |
| `Esc` | Go back |

## Network Details View

| Key | Action |
|-----|--------|
| `.` | Open action menu |
| `r` | Refresh details and subnets |
| `Tab` | Switch focus (tabs/links/content) |
| `h/l` or `1/2` | Switch tabs (Details/Subnets) |
| `j/k` or `↓/↑` | Navigate subnet links or scroll content |
| `Esc` | Go back |

## Firewalls View

| Key | Action |
|-----|--------|
| `Enter` | View firewall rule details |
| `.` | Open action menu |
| `S` | Open sort menu |
| `t` | Enable/disable firewall rule |
| `D` | Delete firewall rule (with confirmation) |
| `/` | Filter rules |
| `r` | Refresh list |
| `Esc` | Go back |

## Firewall Details View

| Key | Action |
|-----|--------|
| `.` | Open action menu |
| `t` | Enable/disable firewall rule |
| `D` | Delete firewall rule (with confirmation) |
| `r` | Refresh details |
| `Tab` | Switch focus (tabs/links/content) |
| `h/l` or `1/2` | Switch tabs (Details/Rules) |
| `Enter` | Navigate to VPC network (when link focused) |
| `Esc` | Go back |

## SQL Instances View

| Key | Action |
|-----|--------|
| `Enter` | View instance details |
| `.` | Open action menu |
| `S` | Open sort menu |
| `s` | Start stopped instance |
| `x` | Stop running instance |
| `R` | Restart instance |
| `D` | Delete instance (with type-to-confirm) |
| `/` | Filter instances |
| `r` | Refresh list |
| `Esc` | Go back |

## SQL Instance Details View

| Key | Action |
|-----|--------|
| `.` | Open action menu |
| `s` | Start instance (if stopped) |
| `x` | Stop instance (if running) |
| `R` | Restart instance |
| `D` | Delete instance (with type-to-confirm) |
| `b` | Create on-demand backup (Backups tab) |
| `r` | Refresh all tabs |
| `Tab` | Switch focus (tabs/content) |
| `h/l` or `1/2/3` | Switch tabs (Details/Databases/Backups) |
| `j/k` or `↓/↑` | Scroll content |
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

## Service Accounts View

| Key | Action |
|-----|--------|
| `Enter` | View service account details |
| `.` | Open action menu |
| `S` | Open sort menu |
| `c` | Create service account |
| `t` | Enable/disable toggle |
| `D` | Delete service account (type-to-confirm on email) |
| `/` | Filter service accounts |
| `r` | Refresh list |
| `Esc` | Go back |

## Service Account Details View

| Key | Action |
|-----|--------|
| `.` | Open action menu |
| `t` | Enable/disable service account |
| `D` | Delete service account (type-to-confirm) |
| `c` | Create key (Keys tab) |
| `w` | Download pending key JSON (Keys tab, after create) |
| `d` | Delete selected key (Keys tab, user-managed only) |
| `r` | Refresh details and keys |
| `Tab` | Switch focus (tabs/content) |
| `h/l` or `1/2` | Switch tabs (Details/Keys) |
| `j/k` or `↓/↑` | Scroll content (Details tab) / Move key selection (Keys tab) |
| `Esc` | Go back |

## IAM Policy View

| Key | Action |
|-----|--------|
| `Enter` | Open member/role detail overlay |
| `a` | Add member/role (context-dependent) |
| `.` | Open action menu |
| `S` | Open sort menu |
| `/` | Filter |
| `r` | Refresh policy |
| `Tab` | Switch focus (tabs/table) |
| `h/l` or `1/2` | Switch tabs (By Member/By Role) |
| `Esc` | Close overlay / Clear filter / Go back |

### IAM Policy Overlay (detail view)

| Key | Action |
|-----|--------|
| `j/k` or `↓/↑` | Navigate items |
| `a` | Add member/role |
| `d` | Remove selected item (with confirmation) |
| `Esc` | Close overlay |

## Custom Roles View

| Key | Action |
|-----|--------|
| `Enter` | View custom role details |
| `S` | Open sort menu |
| `/` | Filter roles |
| `r` | Refresh list |
| `Esc` | Go back |

## Custom Role Details View

| Key | Action |
|-----|--------|
| `r` | Refresh details |
| `Tab` | Switch focus (tabs/content) |
| `h/l` or `1/2` | Switch tabs (Details/Permissions) |
| `j/k` or `↓/↑` | Scroll content |
| `Esc` | Go back |

## Cloud Run Services View

| Key | Action |
|-----|--------|
| `Enter` | View service details |
| `.` | Open action menu |
| `S` | Open sort menu |
| `c` | Create new service |
| `D` | Delete service (with type-to-confirm) |
| `/` | Filter services |
| `r` | Refresh list |
| `Esc` | Go back |

## Cloud Run Service Details View

| Key | Action |
|-----|--------|
| `.` | Open action menu |
| `e` | Edit service configuration |
| `D` | Delete service (with type-to-confirm) |
| `t` | Edit traffic split (Revisions tab) |
| `r` | Refresh details and revisions |
| `Tab` | Switch focus (tabs/content) |
| `h/l` or `1/2/3/4` | Switch tabs (Details/Revisions/YAML/Observability) — when tabs focused |
| `j/k` or `↓/↑` | Scroll content |
| `Esc` | Go back |

## Cloud Run Service Details - Observability Tab

These keys are active when the **content area** is focused (use `Tab` to switch focus):

| Key | Action |
|-----|--------|
| `1` | Set time range to 1 hour |
| `2` | Set time range to 6 hours |
| `3` | Set time range to 24 hours |
| `4` | Set time range to 7 days |
| `5` | Set time range to 30 days |
| `a` | Toggle auto-refresh (30s interval) |
| `I` | Toggle INFO log filter |
| `W` | Toggle WARNING log filter |
| `E` | Toggle ERROR/CRITICAL log filter |
| `L` | Open Logs Explorer (pre-filtered for this service) |
| `r` | Manual refresh |

## Cloud Run Edit/Create View

| Key | Action |
|-----|--------|
| `Ctrl+S` | Preview changes (show diff) |
| `Enter` | Confirm deploy (from diff view) |
| `Tab/↓` | Next field |
| `Shift+Tab/↑` | Previous field |
| `Esc` | Back (diff → form) / Cancel (form → back) |

## Logs Explorer

| Key | Action |
|-----|--------|
| `/` | Focus query input |
| `Enter` | Run query (input) / Expand entry / Filter by field |
| `Esc` | Blur input / Collapse entry / Close filter / Go back |
| `Tab` | Cycle focus (entries / filters / query input / time range) |
| `Shift+Tab` | Cycle focus backwards |
| `j/k` or `↓/↑` | Navigate log entries |
| `→` or `Enter` | Expand entry / Enter field navigation |
| `←` | Collapse entry / Exit field navigation |
| `PgUp/PgDn` | Page up/down through entries |
| `E` | Expand all visible entries |
| `C` | Collapse all entries |
| `w` | Toggle line wrapping |
| `c` | Toggle logfmt/protobuf colorization |
| `1-5` | Time range (1h/6h/24h/7d/30d) |
| `f` | Toggle tail mode (15s polling) |
| `p` | Open entries in $PAGER (respects color toggle) |
| `.` | Open action menu (export TXT/CSV/JSONL) |
| `r` | Refresh (re-run query) |
| `R` | Open resource type filter |
| `L` | Open log name filter |
| `V` | Open severity filter |

### Filter Dropdown (overlay)

| Key | Action |
|-----|--------|
| `j/k` or `↓/↑` | Navigate options |
| `PgUp/PgDn` | Page up/down through options |
| `Space`/`Tab`/`Enter` | Toggle selection |
| `/` | Search within filter options |
| `Esc` | Apply selection and close |
