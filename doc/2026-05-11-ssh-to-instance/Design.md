# SSH to Compute Instance — Design

## Goal

Let the user open an interactive SSH session to a Compute Engine instance from
inside `gcon`, without leaving the TUI. On exit, the user lands back where they
came from.

## UX

### Entry points

| View | Trigger |
|------|---------|
| Instance details | `t` key, or `.` action menu → "SSH" |
| Instances list   | `t` key on a running row, or `.` action menu → "SSH" |

`t` is unbound across both views today (`s` already means *start*; uppercase
is reserved for destructive actions). `t` opens the SSH dialog with smart
defaults pre-filled; `Enter` connects.

### Dialog

A focused, modal overlay (~12 lines tall, centered via `overlay.Center`):

```
╭─ SSH to my-instance ─────────────────────────────╮
│                                                  │
│  Method:  [● gcloud]  ( ) ssh                    │
│                                                  │
│  User:           [_________________]             │
│  Host (ssh):     [10.0.0.5_________]  (greyed)   │
│  IAP tunnel:     [x] on                          │
│  Internal IP:    [ ] off                         │
│  Port forward:   [5432:localhost:5432]           │
│                                                  │
│  [ Connect ]   [ Cancel ]                        │
╰──────────────────────────────────────────────────╯
```

### Smart defaults at open time

| Field | Default |
|---|---|
| Method | `gcloud` if on `$PATH`, else `ssh` |
| User | empty (gcloud → ADC/OS Login picks it; ssh → local `$USER`) |
| Host | external IP if present, else internal IP (ssh mode only) |
| IAP tunnel | `true` when instance has **no** external IP, else `false` |
| Internal IP | `false` |
| Port forward | empty |

### Keyboard

| Key | Action |
|---|---|
| `Tab` / `↓` | next field |
| `Shift+Tab` / `↑` | previous field |
| `←` / `→` | toggle method radio (when focused) |
| `Space` | toggle boolean field |
| `Enter` | activate Connect button (or Cancel) |
| `Esc` | cancel and close dialog |

When `Method = ssh`, the IAP toggle and Internal IP toggle render greyed out
and their values are ignored — the user supplies the destination directly via
the Host field. When `Method = gcloud`, the Host field is greyed (gcloud
resolves the address itself from `--internal-ip` and `--tunnel-through-iap`).
Greying instead of hiding keeps the dialog height stable as method changes.

### Validation

- `Port forward`, if non-empty, must match `\d+:[^:]+:\d+` — inline red error
  under the field, dialog stays open, Connect disabled.
- `ssh` mode requires `Host` non-empty — inline red error.
- Connect button is disabled while any validation error is present.

### Missing-binary behavior

`internal/ssh.LookupBinary` runs on dialog open.

- **Both `gcloud` and `ssh` missing**: dialog renders an inline error with the
  [gcloud install URL](https://cloud.google.com/sdk/docs/install). Only Cancel
  is enabled.
- **Only `gcloud` missing, `ssh` present**: Method toggle is locked to `ssh`
  and a small hint reads `gcloud not found — using plain ssh`.
- **Only `ssh` missing, `gcloud` present**: Method toggle is locked to `gcloud`.
  This is the common WSL/macOS case where users have gcloud but no native ssh
  client; gcloud bundles its own.

### Post-exit

The TUI re-renders the originating view exactly as it was. If the SSH process
exited non-zero, the originating view shows an inline error
(`components.RenderInlineError`) with the exit code and any captured stderr.
On clean exit (Ctrl-D / `exit`), silent return — no banner, no refresh.

## Architecture

```
                ┌─────────────────────┐
                │ instance_details.go │       't' key / action menu
                │ instances.go        │  ──────────────────────────► open dialog
                └──────────┬──────────┘
                           │ instance + zone + project + ips
                           ▼
              ┌─────────────────────────────┐
              │ components/sshdialog/        │
              │   - method radio (gcloud|ssh)│
              │   - IAP toggle               │
              │   - internal-ip toggle       │
              │   - user textinput           │
              │   - host textinput (ssh only)│
              │   - port forward textinput   │
              │  emits SSHConnectMsg{Options}│
              └──────────────┬───────────────┘
                             │
                             ▼
              ┌──────────────────────────────┐
              │ internal/ssh/                │
              │   Options struct             │
              │   BuildGcloudArgs() []string │  ← pure, unit-tested
              │   BuildSSHArgs()    []string │
              │   LookupBinary() (path, ok)  │
              └──────────────┬───────────────┘
                             │
                             ▼
                  app.go: tea.ExecProcess(cmd, onExit)
                             │
                             ▼
                  SSHExitedMsg{err}  → silent return / inline error
```

### New files

- `internal/ssh/ssh.go` — `Options`, `Method`, `BuildGcloudArgs`,
  `BuildSSHArgs`, `LookupBinary`.
- `internal/ssh/ssh_test.go` — table-driven argv tests.
- `internal/ui/components/sshdialog/sshdialog.go` — the focused dialog.
- `internal/ui/components/sshdialog/sshdialog_test.go` — dialog tests.

### Modified files

- `internal/ui/views/instance_details.go` — `t` opens dialog, route
  `SSHConnectMsg` / `SSHExitedMsg`, render dialog overlay.
- `internal/ui/views/instances.go` — same wiring on the list view; `t` only
  fires when the cursor row's status is `RUNNING`.
- `internal/ui/views/instances_actionmenu.go` (and the details equivalent) —
  add "SSH" entry.
- `internal/ui/keys.go` — bind `t` to `SSH` action.
- `internal/ui/app.go` — handle `SSHConnectMsg`: `LookupBinary` → build
  `*exec.Cmd` → `tea.ExecProcess(cmd, onExit)`. Handle `SSHExitedMsg`: stash
  on originating view's `sshErr` field.
- `README.md`, `.claude/rules/key-bindings.md`, `CLAUDE.md` — docs.

## Argv construction

```go
package ssh

type Method int
const (
    MethodGcloud Method = iota
    MethodSSH
)

type Options struct {
    Method      Method
    Project     string  // gcloud only
    Zone        string  // gcloud only
    Instance    string  // gcloud only (used as the SSH target)
    Host        string  // ssh only
    User        string  // optional, both modes
    IAPTunnel   bool    // gcloud only
    InternalIP  bool    // gcloud only — for ssh, the dialog encodes the IP into Host
    PortForward string  // optional, both modes — "L:H:R"
}

func BuildGcloudArgs(opts Options) []string {
    args := []string{"compute", "ssh", opts.Instance,
        "--project=" + opts.Project,
        "--zone=" + opts.Zone}
    if opts.IAPTunnel  { args = append(args, "--tunnel-through-iap") }
    if opts.InternalIP { args = append(args, "--internal-ip") }
    if opts.User != "" {
        args = append(args, "--ssh-flag=-l", "--ssh-flag="+opts.User)
    }
    if opts.PortForward != "" {
        args = append(args, "--ssh-flag=-L", "--ssh-flag="+opts.PortForward)
    }
    return args
}

func BuildSSHArgs(opts Options) []string {
    target := opts.Host
    if opts.User != "" { target = opts.User + "@" + opts.Host }
    args := []string{}
    if opts.PortForward != "" { args = append(args, "-L", opts.PortForward) }
    args = append(args, target)
    return args
}
```

`--ssh-flag=` is split as two args (`--ssh-flag=-l --ssh-flag=USER`) because
gcloud passes each value to ssh as a single token. The joined form
`--ssh-flag="-l USER"` would land in ssh as one argv element and be rejected.
Splitting also avoids shell-injection concerns — each piece is its own argv
element passed straight to `exec.Command`.

`InternalIP` only applies in gcloud mode (`--internal-ip` flag). In ssh mode
the user picks the destination directly via the Host field, so `BuildSSHArgs`
ignores `opts.InternalIP`. A code comment documents this.

## Message flow

```
view.Update(KeyMsg "t")
  └► v.sshDialog = sshdialog.New(...)
     v.showSSHDialog = true
     return v.sshDialog.Init()                    // textinput blink cmd

sshdialog.Update(Enter on Connect)
  └► emits SSHConnectMsg{Options, OriginView}

app.Update(SSHConnectMsg)
  └► binary, ok := ssh.LookupBinary(method)
     if !ok                       → SSHExitedMsg{err: ErrBinaryNotFound}
     cmd := exec.Command(binary, args...)
     return tea.ExecProcess(cmd, func(err error) tea.Msg {
         return SSHExitedMsg{err: err, OriginView: msg.OriginView}
     })

app.Update(SSHExitedMsg)
  └► v.showSSHDialog = false on the origin view
     if msg.err != nil → v.sshErr = msg.err
     return nil
```

The origin view is carried on both messages so the app can route the error
back to whichever view (details or list) launched the session.

## Error handling

| Failure | Surfaced as | Visible where |
|---|---|---|
| Both `gcloud` and `ssh` missing | inline error + Cancel-only | dialog overlay |
| Validation (bad port forward, empty host in ssh mode) | red message under field | dialog overlay |
| `tea.ExecProcess` can't start the binary | `SSHExitedMsg{err}` | originating view inline error |
| Non-zero exit (auth refused, host unreachable, etc.) | `SSHExitedMsg{err: *exec.ExitError}` | originating view inline error, exit code + stderr |
| Clean exit (Ctrl-D / `exit`) | `SSHExitedMsg{err: nil}` | silent return |

## Concurrency

The dialog is modal — only one open at a time per app. `tea.ExecProcess` is
synchronous; bubble-tea suspends and resumes around it. No goroutines, no
tickers, no leaks.

## Testing

1. **`internal/ssh/ssh_test.go`** — table-driven argv tests covering:
   - Minimal gcloud (instance + project + zone only).
   - All gcloud flags combined.
   - Plain ssh with and without `User`, with and without port forward.
   - Port forward arg is split into two `--ssh-flag=` elements, not
     joined-and-quoted.
   - `User` containing `@` or spaces is passed as one argv element (no shell
     interpolation).
2. **`internal/ui/components/sshdialog/sshdialog_test.go`** — focus traversal,
   method radio swap greys/ungreys fields, validation errors render,
   `Enter` on Connect emits `SSHConnectMsg` with expected `Options`, Esc emits
   cancel.
3. **`internal/ui/views/instance_details_test.go`** — `t` opens dialog when
   status is `RUNNING`; no-op when stopped. `SSHExitedMsg{err}` lands as
   `v.sshErr` and renders.
4. **`internal/ui/views/instances_test.go`** — same on the list view.
5. **No end-to-end test for `tea.ExecProcess`** — that's bubble-tea internals
   and is already covered indirectly by the `$PAGER` integration in
   `logs.go`.

## Out of scope (intentionally deferred)

- Save SSH options per instance (would need a small JSON cache).
- One-shot command mode (`--command=...`).
- Multiple port forwards (single forward only in v1).
- SSH key file override (`--ssh-key-file`).
- Connection history / "recent SSH sessions" list.
- Suspended/Stopping/Provisioning states — `t` is only bound when the
  instance is `RUNNING`.
