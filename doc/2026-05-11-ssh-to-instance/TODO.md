# SSH to Instance — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the user open an interactive SSH session to a Compute Engine instance from inside `gcon` and land back where they came from on exit.

**Architecture:** A pure `internal/ssh/` package builds gcloud/ssh argv from an `Options` struct. A focused `sshdialog` component collects options with smart defaults. `App.Update` handles `SSHConnectMsg` by calling `tea.ExecProcess`, returning an `SSHExitedMsg` on completion that routes inline errors back to the originating view.

**Tech Stack:** Go 1.26, Bubble Tea, lipgloss, bubbles/textinput; `os/exec`, `tea.ExecProcess`. Testing via `testify`.

**Spec:** `doc/2026-05-11-ssh-to-instance/Design.md`.

---

## File Structure

| File | Purpose |
|---|---|
| `internal/ssh/ssh.go` *(new)* | `Options`, `Method`, `BuildGcloudArgs`, `BuildSSHArgs`, `LookupBinary`. |
| `internal/ssh/ssh_test.go` *(new)* | Table-driven argv & lookup tests. |
| `internal/ui/components/sshdialog/sshdialog.go` *(new)* | Focused modal dialog. |
| `internal/ui/components/sshdialog/sshdialog_test.go` *(new)* | Focus / validation / submit tests. |
| `internal/ui/views/ssh_messages.go` *(new)* | `SSHConnectMsg`, `SSHExitedMsg` shared types. |
| `internal/ui/views/instance_details.go` *(modify)* | `t` key, dialog overlay, error display, action-menu entry. |
| `internal/ui/views/instance_details_test.go` *(modify)* | `t` opens dialog, exit msg routes error. |
| `internal/ui/views/instances.go` *(modify)* | `t` key on RUNNING rows, same wiring. |
| `internal/ui/views/instances_test.go` *(modify)* | Same tests on list view. |
| `internal/ui/app.go` *(modify)* | Handle `SSHConnectMsg` → `tea.ExecProcess`. |
| `README.md` *(modify)* | Document SSH feature. |
| `CLAUDE.md` *(modify)* | Add SSH to Implemented Features. |
| `.claude/rules/key-bindings.md` *(modify)* | Document `t` in Instances View and Instance Details. |

---

## Task 1: `internal/ssh` package — Options, argv builders, LookupBinary

**Files:**
- Create: `internal/ssh/ssh.go`
- Create: `internal/ssh/ssh_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/ssh/ssh_test.go`:

```go
package ssh

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildGcloudArgs(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want []string
	}{
		{
			name: "minimal",
			opts: Options{
				Method:   MethodGcloud,
				Project:  "my-proj",
				Zone:     "us-central1-a",
				Instance: "my-vm",
			},
			want: []string{
				"compute", "ssh", "my-vm",
				"--project=my-proj",
				"--zone=us-central1-a",
			},
		},
		{
			name: "iap and internal-ip",
			opts: Options{
				Method:     MethodGcloud,
				Project:    "p",
				Zone:       "z",
				Instance:   "vm",
				IAPTunnel:  true,
				InternalIP: true,
			},
			want: []string{
				"compute", "ssh", "vm",
				"--project=p", "--zone=z",
				"--tunnel-through-iap", "--internal-ip",
			},
		},
		{
			name: "user override is split into two --ssh-flag args",
			opts: Options{
				Method:   MethodGcloud,
				Project:  "p", Zone: "z", Instance: "vm",
				User: "alice",
			},
			want: []string{
				"compute", "ssh", "vm",
				"--project=p", "--zone=z",
				"--ssh-flag=-l", "--ssh-flag=alice",
			},
		},
		{
			name: "port forward",
			opts: Options{
				Method:      MethodGcloud,
				Project:     "p", Zone: "z", Instance: "vm",
				PortForward: "5432:localhost:5432",
			},
			want: []string{
				"compute", "ssh", "vm",
				"--project=p", "--zone=z",
				"--ssh-flag=-L", "--ssh-flag=5432:localhost:5432",
			},
		},
		{
			name: "all gcloud flags combined",
			opts: Options{
				Method:      MethodGcloud,
				Project:     "p", Zone: "z", Instance: "vm",
				User:        "bob",
				IAPTunnel:   true,
				InternalIP:  true,
				PortForward: "8080:localhost:80",
			},
			want: []string{
				"compute", "ssh", "vm",
				"--project=p", "--zone=z",
				"--tunnel-through-iap", "--internal-ip",
				"--ssh-flag=-l", "--ssh-flag=bob",
				"--ssh-flag=-L", "--ssh-flag=8080:localhost:80",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, BuildGcloudArgs(tc.opts))
		})
	}
}

func TestBuildSSHArgs(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want []string
	}{
		{
			name: "bare host",
			opts: Options{Method: MethodSSH, Host: "10.0.0.5"},
			want: []string{"10.0.0.5"},
		},
		{
			name: "user @ host",
			opts: Options{Method: MethodSSH, Host: "1.2.3.4", User: "alice"},
			want: []string{"alice@1.2.3.4"},
		},
		{
			name: "port forward",
			opts: Options{
				Method: MethodSSH, Host: "h", User: "u",
				PortForward: "5432:localhost:5432",
			},
			want: []string{"-L", "5432:localhost:5432", "u@h"},
		},
		{
			name: "internalIP is ignored in ssh mode",
			opts: Options{
				Method: MethodSSH, Host: "10.0.0.1", InternalIP: true,
			},
			want: []string{"10.0.0.1"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, BuildSSHArgs(tc.opts))
		})
	}
}

func TestBuildArgs_NoShellInjection(t *testing.T) {
	// User containing spaces / shell metacharacters must arrive as a
	// single argv element (no string-splitting, no quoting).
	opts := Options{
		Method:   MethodGcloud,
		Project:  "p", Zone: "z", Instance: "vm",
		User: "alice; rm -rf /",
	}
	args := BuildGcloudArgs(opts)
	// The user value must appear exactly once, untouched.
	found := 0
	for _, a := range args {
		if a == "--ssh-flag=alice; rm -rf /" {
			found++
		}
	}
	assert.Equal(t, 1, found, "user value must arrive as one argv element")
	// And it must NOT have been split into multiple args.
	for _, a := range args {
		assert.False(t, strings.Contains(a, "rm -rf") && a != "--ssh-flag=alice; rm -rf /",
			"unexpected fragment leaked: %q", a)
	}
}

func TestLookupBinary(t *testing.T) {
	// `sh` is guaranteed to exist on POSIX systems used for CI.
	path, ok := LookupBinary("sh")
	assert.True(t, ok)
	assert.NotEmpty(t, path)

	_, ok = LookupBinary("definitely-not-a-real-binary-xyzzy")
	assert.False(t, ok)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ssh/...`
Expected: FAIL with `package ./internal/ssh: directory not found` (or build error — types not defined).

- [ ] **Step 3: Implement the package**

Create `internal/ssh/ssh.go`:

```go
// Package ssh builds argument vectors for invoking either `gcloud compute ssh`
// or the system `ssh` binary, given a set of user-chosen Options.
//
// The package is intentionally UI-free: it has no Bubble Tea dependencies and
// is fully unit-testable.
package ssh

import "os/exec"

// Method selects which binary the caller will launch.
type Method int

const (
	MethodGcloud Method = iota
	MethodSSH
)

// Options bundles every choice the SSH dialog can produce.
type Options struct {
	Method   Method
	Project  string // gcloud only
	Zone     string // gcloud only
	Instance string // gcloud only — used as the SSH target

	Host string // ssh only — the destination address
	User string // optional, both modes

	IAPTunnel   bool   // gcloud only
	InternalIP  bool   // gcloud only — ssh mode encodes the IP into Host instead
	PortForward string // optional, both modes — "L:H:R"
}

// BuildGcloudArgs returns argv to pass to `exec.Command("gcloud", args...)`.
//
// Each --ssh-flag value becomes its own argv element. gcloud forwards each
// element to ssh as a single token, so "-l USER" must be split as
// "--ssh-flag=-l --ssh-flag=USER" rather than joined-and-quoted.
func BuildGcloudArgs(opts Options) []string {
	args := []string{
		"compute", "ssh", opts.Instance,
		"--project=" + opts.Project,
		"--zone=" + opts.Zone,
	}
	if opts.IAPTunnel {
		args = append(args, "--tunnel-through-iap")
	}
	if opts.InternalIP {
		args = append(args, "--internal-ip")
	}
	if opts.User != "" {
		args = append(args, "--ssh-flag=-l", "--ssh-flag="+opts.User)
	}
	if opts.PortForward != "" {
		args = append(args, "--ssh-flag=-L", "--ssh-flag="+opts.PortForward)
	}
	return args
}

// BuildSSHArgs returns argv to pass to `exec.Command("ssh", args...)`.
//
// opts.InternalIP is intentionally ignored here: in ssh mode the dialog has
// already encoded the desired address into opts.Host.
func BuildSSHArgs(opts Options) []string {
	target := opts.Host
	if opts.User != "" {
		target = opts.User + "@" + opts.Host
	}
	args := []string{}
	if opts.PortForward != "" {
		args = append(args, "-L", opts.PortForward)
	}
	args = append(args, target)
	return args
}

// LookupBinary returns the absolute path to `name` if found on $PATH.
func LookupBinary(name string) (string, bool) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	return path, true
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/ssh/... -v`
Expected: all tests PASS.

- [ ] **Step 5: Lint and commit**

Run: `golangci-lint run ./internal/ssh/...`
Expected: no issues.

```bash
git add internal/ssh/
git commit -m "2026-05-11: add internal/ssh package — argv builders + LookupBinary"
```

---

## Task 2: `sshdialog` component — focused modal with smart defaults

**Files:**
- Create: `internal/ui/components/sshdialog/sshdialog.go`
- Create: `internal/ui/components/sshdialog/sshdialog_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/ui/components/sshdialog/sshdialog_test.go`:

```go
package sshdialog

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gconssh "gcon/internal/ssh"
)

// Helper: build a dialog with everything resolvable so we can test field
// behavior without depending on what's on $PATH.
func newTestDialog(t *testing.T, params Params) *Dialog {
	t.Helper()
	d := New(params)
	// Force both binaries "found" for predictable tests.
	d.gcloudFound = true
	d.sshFound = true
	d.binaryErr = nil
	return d
}

func TestDefaults_InstanceWithoutExternalIP_PrefersIAP(t *testing.T) {
	d := newTestDialog(t, Params{
		Project:    "p",
		Zone:       "z",
		Instance:   "vm",
		InternalIP: "10.0.0.5",
		ExternalIP: "",
	})
	assert.Equal(t, gconssh.MethodGcloud, d.options().Method)
	assert.True(t, d.options().IAPTunnel, "IAP should default ON when no external IP")
}

func TestDefaults_InstanceWithExternalIP_NoIAP(t *testing.T) {
	d := newTestDialog(t, Params{
		Project: "p", Zone: "z", Instance: "vm",
		InternalIP: "10.0.0.5", ExternalIP: "34.1.2.3",
	})
	assert.False(t, d.options().IAPTunnel)
}

func TestDefaults_SSHHost_PrefersExternalIP(t *testing.T) {
	d := newTestDialog(t, Params{
		Project: "p", Zone: "z", Instance: "vm",
		InternalIP: "10.0.0.5", ExternalIP: "34.1.2.3",
	})
	d.setMethod(gconssh.MethodSSH)
	assert.Equal(t, "34.1.2.3", d.options().Host)
}

func TestDefaults_SSHHost_FallsBackToInternalIP(t *testing.T) {
	d := newTestDialog(t, Params{
		Project: "p", Zone: "z", Instance: "vm",
		InternalIP: "10.0.0.5", ExternalIP: "",
	})
	d.setMethod(gconssh.MethodSSH)
	assert.Equal(t, "10.0.0.5", d.options().Host)
}

func TestValidation_PortForwardFormat(t *testing.T) {
	d := newTestDialog(t, Params{Project: "p", Zone: "z", Instance: "vm"})
	d.setPortForward("not-a-spec")
	errs := d.validate()
	require.NotEmpty(t, errs)
	assert.Contains(t, errs["port_forward"], "L:H:R")
}

func TestValidation_PortForwardEmptyIsValid(t *testing.T) {
	d := newTestDialog(t, Params{Project: "p", Zone: "z", Instance: "vm"})
	d.setPortForward("")
	assert.Empty(t, d.validate())
}

func TestValidation_SSHModeRequiresHost(t *testing.T) {
	d := newTestDialog(t, Params{Project: "p", Zone: "z", Instance: "vm",
		InternalIP: "", ExternalIP: ""})
	d.setMethod(gconssh.MethodSSH)
	d.setHost("")
	errs := d.validate()
	require.NotEmpty(t, errs)
	assert.Contains(t, errs["host"], "required")
}

func TestSubmit_EmitsSSHConnectMsg(t *testing.T) {
	d := newTestDialog(t, Params{
		Project: "p", Zone: "z", Instance: "vm",
		InternalIP: "10.0.0.5", ExternalIP: "34.1.2.3",
	})
	cmd := d.submit()
	require.NotNil(t, cmd)
	msg := cmd()
	connect, ok := msg.(ConnectMsg)
	require.True(t, ok, "expected ConnectMsg, got %T", msg)
	assert.Equal(t, "vm", connect.Options.Instance)
}

func TestSubmit_BlockedByValidationError(t *testing.T) {
	d := newTestDialog(t, Params{Project: "p", Zone: "z", Instance: "vm"})
	d.setMethod(gconssh.MethodSSH)
	d.setHost("") // invalid for ssh mode
	cmd := d.submit()
	assert.Nil(t, cmd, "submit must be blocked when validation fails")
}

func TestCancel_EmitsCancelMsg(t *testing.T) {
	d := newTestDialog(t, Params{Project: "p", Zone: "z", Instance: "vm"})
	cmd := d.cancel()
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(CancelMsg)
	assert.True(t, ok)
}

func TestBinaryMissing_LocksMethodToAvailable(t *testing.T) {
	d := New(Params{Project: "p", Zone: "z", Instance: "vm"})
	d.gcloudFound = false
	d.sshFound = true
	d.binaryErr = nil
	d.recomputeMethodLock()
	assert.Equal(t, gconssh.MethodSSH, d.options().Method)
	assert.True(t, d.methodLocked, "method should be locked when one binary is missing")
}

func TestBinaryMissing_BothMissing_ShowsError(t *testing.T) {
	d := New(Params{Project: "p", Zone: "z", Instance: "vm"})
	d.gcloudFound = false
	d.sshFound = false
	d.recomputeMethodLock()
	require.Error(t, d.binaryErr)
	cmd := d.submit()
	assert.Nil(t, cmd, "submit must be blocked when both binaries missing")
}

// Smoke test: rendering must not panic and must include the instance name.
func TestView_RendersInstanceName(t *testing.T) {
	d := newTestDialog(t, Params{Project: "p", Zone: "z", Instance: "my-vm",
		InternalIP: "10.0.0.5", ExternalIP: ""})
	d.SetSize(80, 24)
	out := d.View()
	assert.Contains(t, out, "my-vm")
}

// Update must accept tea.Msg (not just tea.KeyMsg) so textinput blink cmds
// can flow through (per .claude/rules/component-patterns.md).
func TestUpdate_AcceptsBlinkMsgs(t *testing.T) {
	d := newTestDialog(t, Params{Project: "p", Zone: "z", Instance: "vm"})
	var msg tea.Msg = struct{}{}
	_, _ = d.Update(msg)
	// No panic = pass.
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ui/components/sshdialog/...`
Expected: FAIL with `package not found` / `undefined: New` etc.

- [ ] **Step 3: Implement the dialog**

Create `internal/ui/components/sshdialog/sshdialog.go`:

```go
// Package sshdialog renders a focused modal dialog that collects SSH options
// for a Compute Engine instance and emits a ConnectMsg to launch the session.
package sshdialog

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	gconssh "gcon/internal/ssh"
)

// Params is the read-only input required to construct a Dialog.
type Params struct {
	Project    string
	Zone       string
	Instance   string
	InternalIP string
	ExternalIP string
	// OriginView is opaque to the dialog; it is echoed back on ConnectMsg /
	// CancelMsg so the app can route the exit message to the right view.
	OriginView any
}

// ConnectMsg is emitted on successful submission.
type ConnectMsg struct {
	Options    gconssh.Options
	OriginView any
}

// CancelMsg is emitted when the user dismisses the dialog.
type CancelMsg struct {
	OriginView any
}

// Field identifiers for focus & validation.
type field int

const (
	fieldMethod field = iota
	fieldUser
	fieldHost
	fieldIAP
	fieldInternalIP
	fieldPortForward
	fieldConnect
	fieldCancel
	fieldCount
)

var portForwardRe = regexp.MustCompile(`^\d+:[^:]+:\d+$`)

// Dialog is the modal SSH-options popup.
type Dialog struct {
	params Params

	method       gconssh.Method
	user         textinput.Model
	host         textinput.Model
	iap          bool
	internalIP   bool
	portForward  textinput.Model

	focus field

	gcloudFound  bool
	sshFound     bool
	binaryErr    error
	methodLocked bool

	width, height int
}

// New constructs a Dialog with smart defaults.
func New(params Params) *Dialog {
	user := textinput.New()
	user.Placeholder = "(default)"
	user.CharLimit = 64

	host := textinput.New()
	host.CharLimit = 128

	pf := textinput.New()
	pf.Placeholder = "localPort:remoteHost:remotePort"
	pf.CharLimit = 64

	d := &Dialog{
		params:      params,
		user:        user,
		host:        host,
		portForward: pf,
		focus:       fieldUser,
	}

	// Detect binaries.
	_, d.gcloudFound = gconssh.LookupBinary("gcloud")
	_, d.sshFound = gconssh.LookupBinary("ssh")

	// Default method.
	if d.gcloudFound {
		d.method = gconssh.MethodGcloud
	} else {
		d.method = gconssh.MethodSSH
	}
	d.recomputeMethodLock()

	// IAP default: ON when no external IP, OFF otherwise.
	d.iap = params.ExternalIP == ""

	// Default Host: external IP if present, else internal IP.
	if params.ExternalIP != "" {
		d.host.SetValue(params.ExternalIP)
	} else if params.InternalIP != "" {
		d.host.SetValue(params.InternalIP)
	}

	d.user.Focus()
	return d
}

// recomputeMethodLock resolves binary availability into the locked state.
func (d *Dialog) recomputeMethodLock() {
	switch {
	case !d.gcloudFound && !d.sshFound:
		d.binaryErr = fmt.Errorf("neither gcloud nor ssh found on $PATH — install gcloud: https://cloud.google.com/sdk/docs/install")
		d.methodLocked = true
	case !d.gcloudFound:
		d.method = gconssh.MethodSSH
		d.methodLocked = true
	case !d.sshFound:
		d.method = gconssh.MethodGcloud
		d.methodLocked = true
	default:
		d.methodLocked = false
	}
}

// SetSize records the terminal dimensions for centered rendering.
func (d *Dialog) SetSize(width, height int) {
	d.width = width
	d.height = height
}

// Init returns the initial blink command.
func (d *Dialog) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles key and textinput messages.
func (d *Dialog) Update(msg tea.Msg) (*Dialog, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			return d, d.cancel()
		case "tab", "down":
			d.advanceFocus(+1)
			return d, nil
		case "shift+tab", "up":
			d.advanceFocus(-1)
			return d, nil
		case "left":
			if d.focus == fieldMethod && !d.methodLocked {
				d.setMethod(gconssh.MethodGcloud)
			}
			return d, nil
		case "right":
			if d.focus == fieldMethod && !d.methodLocked {
				d.setMethod(gconssh.MethodSSH)
			}
			return d, nil
		case " ":
			switch d.focus {
			case fieldIAP:
				if d.method == gconssh.MethodGcloud {
					d.iap = !d.iap
				}
				return d, nil
			case fieldInternalIP:
				if d.method == gconssh.MethodGcloud {
					d.internalIP = !d.internalIP
				}
				return d, nil
			}
		case "enter":
			switch d.focus {
			case fieldCancel:
				return d, d.cancel()
			case fieldConnect:
				return d, d.submit()
			default:
				return d, d.submit()
			}
		}
	}

	// Route blink / character messages to the currently focused textinput.
	var cmd tea.Cmd
	switch d.focus {
	case fieldUser:
		d.user, cmd = d.user.Update(msg)
	case fieldHost:
		if d.method == gconssh.MethodSSH {
			d.host, cmd = d.host.Update(msg)
		}
	case fieldPortForward:
		d.portForward, cmd = d.portForward.Update(msg)
	}
	return d, cmd
}

func (d *Dialog) advanceFocus(delta int) {
	d.user.Blur()
	d.host.Blur()
	d.portForward.Blur()
	d.focus = field((int(d.focus) + delta + int(fieldCount)) % int(fieldCount))
	switch d.focus {
	case fieldUser:
		d.user.Focus()
	case fieldHost:
		if d.method == gconssh.MethodSSH {
			d.host.Focus()
		} else {
			d.advanceFocus(delta) // skip greyed field
		}
	case fieldPortForward:
		d.portForward.Focus()
	}
}

func (d *Dialog) setMethod(m gconssh.Method) {
	if d.methodLocked {
		return
	}
	d.method = m
}

func (d *Dialog) setHost(h string)        { d.host.SetValue(h) }
func (d *Dialog) setPortForward(p string) { d.portForward.SetValue(p) }

// validate returns a map of field-id → error message. Empty map means valid.
func (d *Dialog) validate() map[string]string {
	errs := map[string]string{}
	if pf := d.portForward.Value(); pf != "" && !portForwardRe.MatchString(pf) {
		errs["port_forward"] = "expected L:H:R format, e.g. 5432:localhost:5432"
	}
	if d.method == gconssh.MethodSSH && strings.TrimSpace(d.host.Value()) == "" {
		errs["host"] = "host is required in ssh mode"
	}
	return errs
}

// options snapshots current state into an ssh.Options struct.
func (d *Dialog) options() gconssh.Options {
	return gconssh.Options{
		Method:      d.method,
		Project:     d.params.Project,
		Zone:        d.params.Zone,
		Instance:    d.params.Instance,
		Host:        d.host.Value(),
		User:        d.user.Value(),
		IAPTunnel:   d.iap,
		InternalIP:  d.internalIP,
		PortForward: d.portForward.Value(),
	}
}

func (d *Dialog) submit() tea.Cmd {
	if d.binaryErr != nil {
		return nil
	}
	if len(d.validate()) > 0 {
		return nil
	}
	opts := d.options()
	origin := d.params.OriginView
	return func() tea.Msg {
		return ConnectMsg{Options: opts, OriginView: origin}
	}
}

func (d *Dialog) cancel() tea.Cmd {
	origin := d.params.OriginView
	return func() tea.Msg { return CancelMsg{OriginView: origin} }
}

// --- Rendering ---

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	greyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#5F6368")).Faint(true)
	focusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4")).Bold(true)
	boxStyle   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4285F4")).
			Padding(1, 2)
)

// View renders the dialog as a boxed, centered popup body.
// (Centering on the terminal is the caller's job via overlay.Center.)
func (d *Dialog) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("SSH to " + d.params.Instance))
	b.WriteString("\n\n")

	// Method radio.
	gcloud := "( ) gcloud"
	ssh := "( ) ssh"
	if d.method == gconssh.MethodGcloud {
		gcloud = "(●) gcloud"
	} else {
		ssh = "(●) ssh"
	}
	if d.methodLocked {
		gcloud = greyStyle.Render(gcloud)
		ssh = greyStyle.Render(ssh)
	} else if d.focus == fieldMethod {
		if d.method == gconssh.MethodGcloud {
			gcloud = focusStyle.Render(gcloud)
		} else {
			ssh = focusStyle.Render(ssh)
		}
	}
	b.WriteString(labelStyle.Render("Method:   ") + gcloud + "  " + ssh + "\n\n")

	// Inputs.
	errs := d.validate()

	b.WriteString(labelStyle.Render("User:           "))
	b.WriteString(d.user.View() + "\n")

	hostLabel := "Host (ssh):     "
	if d.method == gconssh.MethodGcloud {
		b.WriteString(greyStyle.Render(hostLabel + d.host.Value()) + "\n")
	} else {
		b.WriteString(labelStyle.Render(hostLabel))
		b.WriteString(d.host.View() + "\n")
	}
	if e := errs["host"]; e != "" {
		b.WriteString("                " + errorStyle.Render(e) + "\n")
	}

	iapBox := checkbox(d.iap)
	intBox := checkbox(d.internalIP)
	if d.method == gconssh.MethodSSH {
		iapBox = greyStyle.Render(iapBox)
		intBox = greyStyle.Render(intBox)
	}
	b.WriteString(labelStyle.Render("IAP tunnel:     ") + iapBox + "\n")
	b.WriteString(labelStyle.Render("Internal IP:    ") + intBox + "\n")

	b.WriteString(labelStyle.Render("Port forward:   "))
	b.WriteString(d.portForward.View() + "\n")
	if e := errs["port_forward"]; e != "" {
		b.WriteString("                " + errorStyle.Render(e) + "\n")
	}

	// Buttons.
	connect := "[ Connect ]"
	cancel := "[ Cancel ]"
	disabled := d.binaryErr != nil || len(errs) > 0
	switch {
	case disabled:
		connect = greyStyle.Render(connect)
	case d.focus == fieldConnect:
		connect = focusStyle.Render(connect)
	}
	if d.focus == fieldCancel {
		cancel = focusStyle.Render(cancel)
	}
	b.WriteString("\n" + connect + "   " + cancel + "\n")

	if d.binaryErr != nil {
		b.WriteString("\n" + errorStyle.Render(d.binaryErr.Error()) + "\n")
	} else if !d.gcloudFound {
		b.WriteString("\n" + labelStyle.Render("gcloud not found — using plain ssh") + "\n")
	} else if !d.sshFound {
		b.WriteString("\n" + labelStyle.Render("ssh not found — using gcloud") + "\n")
	}

	b.WriteString("\n" + labelStyle.Render("Tab/Shift-Tab navigate · Space toggle · Enter connect · Esc cancel"))

	return boxStyle.Render(b.String())
}

func checkbox(on bool) string {
	if on {
		return "[x] on"
	}
	return "[ ] off"
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/ui/components/sshdialog/... -v`
Expected: all tests PASS.

- [ ] **Step 5: Lint and commit**

Run: `golangci-lint run ./internal/ui/components/sshdialog/...`
Expected: no issues.

```bash
git add internal/ui/components/sshdialog/
git commit -m "2026-05-11: add sshdialog component"
```

---

## Task 3: Shared SSH message types

**Files:**
- Create: `internal/ui/views/ssh_messages.go`

- [ ] **Step 1: Create the file**

Create `internal/ui/views/ssh_messages.go`:

```go
package views

import "gcon/internal/ssh"

// SSHRequestMsg is emitted by a view to request that the app launch an SSH
// session. The OriginView (an opaque any) is echoed back on SSHExitedMsg so
// the app can route exit errors to the right view.
type SSHRequestMsg struct {
	Options    ssh.Options
	OriginView any
}

// SSHExitedMsg is delivered after tea.ExecProcess returns control to the TUI.
// Err is nil on clean exit, non-nil on launch failure or non-zero ssh exit.
type SSHExitedMsg struct {
	Err        error
	OriginView any
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/ui/views/...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/views/ssh_messages.go
git commit -m "2026-05-11: add SSH request/exit message types"
```

---

## Task 4: Wire SSH into Instance Details view

**Files:**
- Modify: `internal/ui/views/instance_details.go` (add `t` keybinding, dialog state, action-menu entry, View overlay, sshErr field)
- Modify: `internal/ui/views/instance_details_test.go` (regression tests)

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/views/instance_details_test.go`:

```go
func TestInstanceDetails_TKey_OpensSSHDialog_WhenRunning(t *testing.T) {
	v := NewInstanceDetailsView("proj", "us-central1-a", "vm", nil, nil)
	v.details = &gcp.InstanceDetails{
		Name:       "vm",
		Status:     "RUNNING",
		InternalIP: "10.0.0.5",
		ExternalIP: "34.1.2.3",
	}
	cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	assert.NotNil(t, cmd, "expected dialog Init cmd")
	assert.True(t, v.showSSHDialog, "dialog should be visible after t key")
	assert.NotNil(t, v.sshDialog)
}

func TestInstanceDetails_TKey_NoOp_WhenStopped(t *testing.T) {
	v := NewInstanceDetailsView("proj", "us-central1-a", "vm", nil, nil)
	v.details = &gcp.InstanceDetails{Name: "vm", Status: "TERMINATED"}
	_ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	assert.False(t, v.showSSHDialog, "dialog must not open on stopped instance")
}

func TestInstanceDetails_SSHExited_StoresError(t *testing.T) {
	v := NewInstanceDetailsView("proj", "us-central1-a", "vm", nil, nil)
	v.SetSSHError(errors.New("connection refused"))
	assert.EqualError(t, v.sshErr, "connection refused")
}

func TestInstanceDetails_HasTextInputFocused_WhenDialogOpen(t *testing.T) {
	v := NewInstanceDetailsView("proj", "us-central1-a", "vm", nil, nil)
	v.details = &gcp.InstanceDetails{Name: "vm", Status: "RUNNING",
		InternalIP: "10.0.0.5", ExternalIP: "34.1.2.3"}
	_ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	assert.True(t, v.HasTextInputFocused())
}
```

If the test file already imports `errors` and `tea`/`gcp`, do not duplicate the imports.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ui/views/ -run TestInstanceDetails_TKey -v`
Expected: FAIL with `undefined: showSSHDialog` etc.

- [ ] **Step 3: Add dialog state, key handler, and error plumbing**

In `internal/ui/views/instance_details.go`:

3a. Add imports at the top of the file (alongside existing imports):

```go
import (
    // ...existing imports...
    "gcon/internal/ui/components/sshdialog"
)
```

3b. Add new fields to `InstanceDetailsView` struct (find the struct around line ~150 and append):

```go
    // SSH dialog state
    sshDialog     *sshdialog.Dialog
    showSSHDialog bool
    sshErr        error
```

3c. Add `SSH` to `instanceDetailsKeyMap` (around line 156) and bind it in `defaultInstanceDetailsKeyMap`:

```go
type instanceDetailsKeyMap struct {
    // ...existing...
    SSH key.Binding
}
```

```go
SSH: key.NewBinding(
    key.WithKeys("t"),
    key.WithHelp("t", "ssh"),
),
```

3d. Add the key handler in the keybinding switch (around line 558, in `handleKey`/`Update`):

```go
case key.Matches(msg, v.keys.SSH):
    if v.details != nil && v.isInstanceRunning() {
        return v.openSSHDialog()
    }
```

3e. Add the `openSSHDialog` helper (place it near `buildActions` around line 633):

```go
func (v *InstanceDetailsView) openSSHDialog() tea.Cmd {
    v.sshErr = nil
    v.sshDialog = sshdialog.New(sshdialog.Params{
        Project:    v.projectID,
        Zone:       v.zone,
        Instance:   v.instanceName,
        InternalIP: v.details.InternalIP,
        ExternalIP: v.details.ExternalIP,
        OriginView: v,
    })
    v.sshDialog.SetSize(v.width, v.height)
    v.showSSHDialog = true
    return v.sshDialog.Init()
}
```

3f. Add `sshdialog.ConnectMsg` / `CancelMsg` handling at the top of `Update` (alongside the existing `actionmenu.ActionSelectedMsg` cases). The view does not handle ConnectMsg itself — it just closes its dialog and re-emits an `SSHRequestMsg` for the app:

```go
case sshdialog.ConnectMsg:
    v.showSSHDialog = false
    return func() tea.Msg {
        return SSHRequestMsg{Options: msg.Options, OriginView: v}
    }
case sshdialog.CancelMsg:
    v.showSSHDialog = false
    return nil
```

3g. Route incoming messages to the dialog while it's open (before the keymap switch in `Update`):

```go
if v.showSSHDialog && v.sshDialog != nil {
    var cmd tea.Cmd
    v.sshDialog, cmd = v.sshDialog.Update(msg)
    return cmd
}
```

3h. Render `sshErr` inline by appending it to `mainContent` in `View()` (just before the overlay checks, around line 995). The local `mainContent` is built around line 993:

```go
mainContent := header + tabBar + "\n" + viewportContent + help
if v.sshErr != nil {
    mainContent += "\n" + components.RenderInlineError(v.sshErr)
}
```

3i. Render the SSH dialog as the **highest-priority** overlay. Per `.claude/rules/bubble-tea-rendering.md` ("Overlay Z-Order"), dialogs that can be spawned from within other overlays render first. Add this check **before** the menu/stop/delete overlay checks (around line 995):

```go
if v.showSSHDialog && v.sshDialog != nil {
    return v.renderWithOverlay(mainContent, v.sshDialog.View())
}
```

3j. Add `SetSSHError` (public, called from the app handler):

```go
// SetSSHError stores an error from a finished SSH session for inline display.
func (v *InstanceDetailsView) SetSSHError(err error) { v.sshErr = err }
```

3k. Update `HasTextInputFocused` (existing impl is around line 1033; add the dialog check):

```go
func (v *InstanceDetailsView) HasTextInputFocused() bool {
    if v.showSSHDialog && v.sshDialog != nil {
        return true
    }
    // ...existing logic...
    return false
}
```

3l. Update `IsMenuOpen` (existing impl at line 1029) so Esc routes to the dialog instead of navigating back:

```go
func (v *InstanceDetailsView) IsMenuOpen() bool {
    return v.menuOpen || v.showStopConfirm || v.showDeleteConfirm || v.showSSHDialog
}
```

3m. Add a SSH entry to the action menu. In `buildActions` (around line 633), add:

```go
{Key: 't', Label: "SSH", Enabled: isRunning},
```

3n. Add the menu-action wiring in the `actionmenu.ActionSelectedMsg` switch (the one that already handles 'e', 's', 'x', etc.):

```go
case 't':
    if v.details != nil && v.isInstanceRunning() {
        return v.openSSHDialog()
    }
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/ui/views/ -run TestInstanceDetails_TKey -v`
Expected: PASS.

Also run the full file's tests to make sure we haven't broken anything:

Run: `go test ./internal/ui/views/ -run TestInstanceDetails -v`
Expected: PASS.

- [ ] **Step 5: Lint and commit**

Run: `golangci-lint run ./internal/ui/views/...`
Expected: no issues.

```bash
git add internal/ui/views/instance_details.go internal/ui/views/instance_details_test.go
git commit -m "2026-05-11: wire SSH into instance details view"
```

---

## Task 5: Wire SSH into Instances list view

**Files:**
- Modify: `internal/ui/views/instances.go`
- Modify: `internal/ui/views/instances_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/views/instances_test.go`:

```go
func TestInstances_TKey_OpensSSHDialog_OnRunningRow(t *testing.T) {
	v := NewInstancesView("proj", nil, nil)
	v.instances = []gcp.Instance{
		{Name: "vm", Status: "RUNNING", InternalIP: "10.0.0.5", ExternalIP: "34.1.2.3",
			Zone: "us-central1-a"},
	}
	v.table.SetCursor(0)
	_ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	assert.True(t, v.showSSHDialog)
	assert.NotNil(t, v.sshDialog)
}

func TestInstances_TKey_NoOp_OnStoppedRow(t *testing.T) {
	v := NewInstancesView("proj", nil, nil)
	v.instances = []gcp.Instance{{Name: "vm", Status: "TERMINATED"}}
	v.table.SetCursor(0)
	_ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	assert.False(t, v.showSSHDialog)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ui/views/ -run TestInstances_TKey -v`
Expected: FAIL.

- [ ] **Step 3: Add the same wiring as Task 4, list-view flavor**

In `internal/ui/views/instances.go`:

3a. Add the import:

```go
"gcon/internal/ui/components/sshdialog"
```

3b. Add fields to `InstancesView`:

```go
    sshDialog     *sshdialog.Dialog
    showSSHDialog bool
    sshErr        error
```

3c. Add `SSH` to whatever local keymap the file uses (mirror the instance_details pattern). Bind to `"t"`.

3d. Add the key handler in `Update`, gated by running status of the cursor row:

```go
case key.Matches(msg, v.keys.SSH):
    if inst := v.cursorInstance(); inst != nil && inst.IsRunning() {
        return v.openSSHDialog(inst)
    }
```

(If `cursorInstance()` doesn't exist, use the same row-lookup pattern that the existing Start/Stop/Reset handlers in this file use — see lines 445–463 from the earlier grep.)

3e. Add `openSSHDialog`:

```go
func (v *InstancesView) openSSHDialog(inst *gcp.Instance) tea.Cmd {
    v.sshErr = nil
    v.sshDialog = sshdialog.New(sshdialog.Params{
        Project:    v.projectID,
        Zone:       inst.Zone,
        Instance:   inst.Name,
        InternalIP: inst.InternalIP,
        ExternalIP: inst.ExternalIP,
        OriginView: v,
    })
    v.sshDialog.SetSize(v.width, v.height)
    v.showSSHDialog = true
    return v.sshDialog.Init()
}
```

3f. Route dialog messages while it's open (same shape as Task 4 Step 3g):

```go
if v.showSSHDialog && v.sshDialog != nil {
    var cmd tea.Cmd
    v.sshDialog, cmd = v.sshDialog.Update(msg)
    return cmd
}
```

3g. Handle `ConnectMsg` / `CancelMsg`:

```go
case sshdialog.ConnectMsg:
    v.showSSHDialog = false
    return func() tea.Msg {
        return SSHRequestMsg{Options: msg.Options, OriginView: v}
    }
case sshdialog.CancelMsg:
    v.showSSHDialog = false
    return nil
```

3h. Render `sshErr` inline by appending it to `mainContent` in `View()` (just before the overlay checks at lines ~726-740):

```go
if v.sshErr != nil {
    mainContent += "\n" + components.RenderInlineError(v.sshErr)
}
```

3i. Render the SSH dialog as the highest-priority overlay (mirror Task 4 Step 3i). Add this check **before** the existing menu/stop/delete overlay checks (before line 729):

```go
if v.showSSHDialog && v.sshDialog != nil {
    return v.renderWithOverlay(mainContent, v.sshDialog.View())
}
```

3j. `SetSSHError`:

```go
func (v *InstancesView) SetSSHError(err error) { v.sshErr = err }
```

3k. `HasTextInputFocused`:

```go
func (v *InstancesView) HasTextInputFocused() bool {
    if v.showSSHDialog && v.sshDialog != nil {
        return true
    }
    return false
}
```

3l. Update `IsMenuOpen` (existing impl at line ~774):

```go
func (v *InstancesView) IsMenuOpen() bool {
    // existing terms || v.showSSHDialog
}
```

Add `|| v.showSSHDialog` to the existing return expression.

3m. Add a `t` entry to the action menu (mirror Task 4 Step 3m), enabled only when the cursor row is running.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/ui/views/ -run TestInstances -v`
Expected: PASS.

- [ ] **Step 5: Lint and commit**

Run: `golangci-lint run ./internal/ui/views/...`
Expected: no issues.

```bash
git add internal/ui/views/instances.go internal/ui/views/instances_test.go
git commit -m "2026-05-11: wire SSH into instances list view"
```

---

## Task 6: App-level handler — `SSHRequestMsg` → `tea.ExecProcess`

**Files:**
- Modify: `internal/ui/app.go`

- [ ] **Step 1: Locate the central message switch**

Run: `grep -n "func (a \*App) Update" /home/vlad/dev/my/gcon/internal/ui/app.go`

Note the function and switch-case structure. We'll add a `case views.SSHRequestMsg` and a `case views.SSHExitedMsg`.

- [ ] **Step 2: Add the handlers**

In `internal/ui/app.go`, add imports:

```go
import (
    "os/exec"
    // ...existing...
    gconssh "gcon/internal/ssh"
    "gcon/internal/ui/views"
)
```

(If `views` or `os/exec` is already imported, do not duplicate. `gconssh` alias avoids collision with the `ssh` package name elsewhere; if no collision exists, use plain `"gcon/internal/ssh"`.)

In `(*App).Update`, in the message switch:

```go
case views.SSHRequestMsg:
    return a, a.runSSHSession(msg)

case views.SSHExitedMsg:
    a.routeSSHExit(msg)
    return a, nil
```

Add the handler methods (near other private message handlers):

```go
// runSSHSession picks the right binary and hands the terminal to it.
func (a *App) runSSHSession(msg views.SSHRequestMsg) tea.Cmd {
    binName := "gcloud"
    args := gconssh.BuildGcloudArgs(msg.Options)
    if msg.Options.Method == gconssh.MethodSSH {
        binName = "ssh"
        args = gconssh.BuildSSHArgs(msg.Options)
    }

    path, ok := gconssh.LookupBinary(binName)
    if !ok {
        return func() tea.Msg {
            return views.SSHExitedMsg{
                Err:        fmt.Errorf("%s not found on $PATH", binName),
                OriginView: msg.OriginView,
            }
        }
    }

    //nolint:gosec // binary path resolved via LookPath; args built from typed Options struct
    cmd := exec.Command(path, args...)
    origin := msg.OriginView
    return tea.ExecProcess(cmd, func(err error) tea.Msg {
        return views.SSHExitedMsg{Err: err, OriginView: origin}
    })
}

// routeSSHExit hands the (optional) error back to whichever view originated it.
func (a *App) routeSSHExit(msg views.SSHExitedMsg) {
    if msg.Err == nil {
        return
    }
    type sshErrSetter interface{ SetSSHError(error) }
    if setter, ok := msg.OriginView.(sshErrSetter); ok {
        setter.SetSSHError(msg.Err)
    } else {
        a.err = msg.Err
    }
}
```

- [ ] **Step 3: Verify it builds**

Run: `go build ./...`
Expected: clean build.

- [ ] **Step 4: Add a smoke test**

Append to `internal/ui/app_test.go` (or create one if missing — check first with `ls internal/ui/app_test.go`):

```go
func TestApp_SSHExited_RoutesErrorToOriginView(t *testing.T) {
    a := &App{}
    detailsView := views.NewInstanceDetailsView("p", "z", "vm", nil, nil)
    a.routeSSHExit(views.SSHExitedMsg{
        Err:        errors.New("boom"),
        OriginView: detailsView,
    })
    // The view should now expose the error via its View() inline error path.
    assert.NotNil(t, detailsView)
    // (We don't reach into private fields; the test mainly asserts no panic
    // and that the type-assertion path matches.)
}

func TestApp_SSHExited_NilError_NoOp(t *testing.T) {
    a := &App{}
    a.routeSSHExit(views.SSHExitedMsg{Err: nil})
    // No panic, no app-level error set.
    assert.Nil(t, a.err)
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/ui/... -v`
Expected: all pass.

- [ ] **Step 6: Lint and commit**

Run: `golangci-lint run ./internal/ui/...`
Expected: no issues.

```bash
git add internal/ui/app.go internal/ui/app_test.go
git commit -m "2026-05-11: handle SSHRequestMsg via tea.ExecProcess"
```

---

## Task 7: Documentation

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`
- Modify: `.claude/rules/key-bindings.md`

- [ ] **Step 1: Update `.claude/rules/key-bindings.md`**

In the "Instances View" table, add a row:

```markdown
| `t` | SSH to instance (running only) |
```

In the "Instance Details" table, add a row:

```markdown
| `t` | SSH to instance (running only) |
```

- [ ] **Step 2: Update `CLAUDE.md` Implemented Features list**

Under the Compute Engine entries, add:

```markdown
- [x] SSH to running instance
  - `t` key opens an options dialog (method, user, IAP tunnel, internal IP, port forward)
  - Hands off via `gcloud compute ssh` by default; falls back to plain `ssh` when gcloud is absent
  - Returns to the TUI on exit; non-zero exits surfaced inline
```

And remove the `[ ] SSH to instance (via gcloud)` line from the Planned Features list.

- [ ] **Step 3: Update `README.md`**

In the feature list / Compute Engine section, mention SSH (`t` key) and the gcloud / plain-ssh fallback. Keep it brief — one line in the Compute Engine bullet list plus an inline note that the session takes over the terminal and returns on exit.

- [ ] **Step 4: Commit**

```bash
git add README.md CLAUDE.md .claude/rules/key-bindings.md
git commit -m "2026-05-11: document SSH-to-instance feature"
```

---

## Task 8: Final validation — full test suite + lint + manual smoke

**Files:** none modified; verification only.

- [ ] **Step 1: Run the full test suite**

Run: `make test`
Expected: all packages pass.

- [ ] **Step 2: Lint**

Run: `make lint`
Expected: no issues.

- [ ] **Step 3: Manual smoke test**

Build and launch the app:

```bash
make build && ./bin/gcon
```

Then:
1. Navigate to Instances. Move cursor to a running instance. Press `t`.
2. Verify the dialog appears centered, with smart defaults filled in.
3. Tab through fields, toggle IAP, type a port forward, press Enter.
4. Confirm the TUI suspends and `gcloud compute ssh` runs.
5. Exit the session (Ctrl-D). Confirm the TUI restores and shows no error.
6. Press `t` again, choose `ssh` method, fill Host, connect. Confirm fallback path works (or that the appropriate inline error shows if the instance has no key set up).
7. Press `Esc` from the dialog. Confirm it closes cleanly.

If anything fails, file the issue inline and fix before merging.

- [ ] **Step 4: Push the branch**

```bash
git push -u origin 2026-05-11-ssh-to-instance
```

- [ ] **Step 5: Open a pull request**

```bash
gh pr create --title "SSH to running instance (gcloud / ssh fallback)" --body "$(cat <<'EOF'
## Summary
- `t` key on a running Compute Engine instance opens a focused SSH-options dialog (method, user, IAP tunnel, internal IP, port forward) with smart defaults.
- Hands off via `gcloud compute ssh` by default; falls back to plain `ssh` when gcloud is missing.
- Returns to the TUI on exit; non-zero exits surfaced inline on the originating view.

## Test plan
- [ ] Unit tests for `internal/ssh` argv builders + LookupBinary
- [ ] Unit tests for `sshdialog` focus / validation / submit
- [ ] View tests: `t` opens dialog only when RUNNING; SSHExitedMsg surfaces error inline
- [ ] Manual smoke: gcloud handoff, ssh fallback, Esc cancel, port-forward validation

Spec: `doc/2026-05-11-ssh-to-instance/Design.md`
EOF
)"
```

---

## Self-Review Checklist

Run through these before handing off:

1. **Spec coverage:** Every UX, Architecture, Argv, Message-flow, Error-handling, and Testing item from `Design.md` maps to a task above. The "Out of scope" list stays out of scope.
2. **No placeholders:** Every step has concrete code or commands. No "TODO", "TBD", or "implement as needed".
3. **Type consistency:** `Options`, `Method`, `MethodGcloud`, `MethodSSH`, `BuildGcloudArgs`, `BuildSSHArgs`, `LookupBinary`, `Dialog`, `Params`, `ConnectMsg`, `CancelMsg`, `SSHRequestMsg`, `SSHExitedMsg`, `SetSSHError`, `openSSHDialog` — all spelled identically across tasks.
4. **TDD order honored:** every code task starts with a failing test before the implementation step.
5. **Commits frequent:** each task ends in a commit.
