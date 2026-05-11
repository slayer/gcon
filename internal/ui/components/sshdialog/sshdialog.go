// Package sshdialog renders a focused modal dialog that collects SSH options
// for a Compute Engine instance and emits a ConnectMsg to launch the session.
package sshdialog

import (
	"errors"
	"os/user"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	gconssh "github.com/slayer/gcon/internal/ssh"
)

// errNoBinaries is returned when neither gcloud nor ssh is found on $PATH.
var errNoBinaries = errors.New("neither gcloud nor ssh found on $PATH — install gcloud: https://cloud.google.com/sdk/docs/install")

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

// field is an internal identifier for focus & validation.
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

const errorIndent = "                " // aligns error text under input fields (16 cols)

// Dialog is the modal SSH-options popup.
type Dialog struct {
	params Params

	method      gconssh.Method
	user        textinput.Model
	host        textinput.Model
	iap         bool
	internalIP  bool
	portForward textinput.Model

	focus field

	gcloudFound  bool
	sshFound     bool
	binaryErr    error
	methodLocked bool

	width, height int
}

// defaultUserPlaceholder returns the placeholder text shown in the User
// field when empty: the local OS username if it can be resolved, otherwise
// a generic "(default)" hint. gcloud and ssh both fall back to this name
// when no explicit user is passed (gcloud additionally honors OS Login).
func defaultUserPlaceholder() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username + " (default)"
	}
	return "(default)"
}

// New constructs a Dialog with smart defaults.
func New(params Params) *Dialog {
	const inputWidth = 32

	userInput := textinput.New()
	userInput.Placeholder = defaultUserPlaceholder()
	userInput.CharLimit = 64
	userInput.Width = inputWidth

	host := textinput.New()
	host.CharLimit = 128
	host.Width = inputWidth

	pf := textinput.New()
	pf.Placeholder = "localPort:remoteHost:remotePort"
	pf.CharLimit = 64
	pf.Width = inputWidth

	d := &Dialog{
		params:      params,
		user:        userInput,
		host:        host,
		portForward: pf,
		focus:       fieldUser,
	}

	_, d.gcloudFound = gconssh.LookupBinary("gcloud")
	_, d.sshFound = gconssh.LookupBinary("ssh")

	if d.gcloudFound {
		d.method = gconssh.MethodGcloud
	} else {
		d.method = gconssh.MethodSSH
	}
	d.recomputeMethodLock()

	d.iap = params.ExternalIP == ""

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
		d.binaryErr = errNoBinaries
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

// Update handles key and textinput messages. Accepts tea.Msg (not just
// tea.KeyMsg) so textinput blink cmds can flow through.
func (d *Dialog) Update(msg tea.Msg) (*Dialog, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			cmd := d.cancel()
			return d, cmd
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
				cmd := d.cancel()
				return d, cmd
			case fieldConnect:
				cmd := d.submit()
				return d, cmd
			default:
				d.advanceFocus(+1)
				return d, nil
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
	for range int(fieldCount) {
		d.focus = field((int(d.focus) + delta + int(fieldCount)) % int(fieldCount))
		if !d.isFieldSkipped() {
			break
		}
	}
	switch d.focus {
	case fieldUser:
		d.user.Focus()
	case fieldHost:
		d.host.Focus()
	case fieldPortForward:
		d.portForward.Focus()
	}
}

// isFieldSkipped reports whether the currently-focused field is greyed out
// and should be passed over by navigation.
func (d *Dialog) isFieldSkipped() bool {
	switch d.focus {
	case fieldHost:
		return d.method == gconssh.MethodGcloud
	case fieldIAP, fieldInternalIP:
		return d.method == gconssh.MethodSSH
	}
	return false
}

func (d *Dialog) setMethod(m gconssh.Method) {
	if d.methodLocked {
		return
	}
	d.method = m
}

// setHost / setPortForward are test-only mutators that bypass the textinput
// keystroke pipeline. Production code should not call them directly — the
// dialog's textinputs receive characters through Update.
func (d *Dialog) setHost(h string)        { d.host.SetValue(h) }
func (d *Dialog) setPortForward(p string) { d.portForward.SetValue(p) }

// validate returns a map of field-id → error message. Empty map means valid.
// Input values are trimmed before validation so callers cannot bypass checks
// by submitting whitespace-only values.
func (d *Dialog) validate() map[string]string {
	errs := map[string]string{}
	if pf := strings.TrimSpace(d.portForward.Value()); pf != "" && !portForwardRe.MatchString(pf) {
		errs["port_forward"] = "expected L:H:R format, e.g. 5432:localhost:5432"
	}
	if d.method == gconssh.MethodSSH && strings.TrimSpace(d.host.Value()) == "" {
		errs["host"] = "host is required in ssh mode"
	}
	return errs
}

// options snapshots current state into an ssh.Options struct. User-typed
// strings are trimmed so stray whitespace doesn't reach gcloud/ssh and
// produce confusing connection errors (e.g., "alice " resolving to a
// non-existent OS account).
func (d *Dialog) options() gconssh.Options {
	return gconssh.Options{
		Method:      d.method,
		Project:     d.params.Project,
		Zone:        d.params.Zone,
		Instance:    d.params.Instance,
		Host:        strings.TrimSpace(d.host.Value()),
		User:        strings.TrimSpace(d.user.Value()),
		IAPTunnel:   d.iap,
		InternalIP:  d.internalIP,
		PortForward: strings.TrimSpace(d.portForward.Value()),
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

// View renders the dialog as a boxed body. Centering on the terminal is the
// caller's job via overlay.Center.
func (d *Dialog) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("SSH to " + d.params.Instance))
	b.WriteString("\n\n")

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
		b.WriteString(errorIndent + errorStyle.Render(e) + "\n")
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
		b.WriteString(errorIndent + errorStyle.Render(e) + "\n")
	}

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

	switch {
	case d.binaryErr != nil:
		b.WriteString("\n" + errorStyle.Render(d.binaryErr.Error()) + "\n")
	case !d.gcloudFound:
		b.WriteString("\n" + labelStyle.Render("gcloud not found — using plain ssh") + "\n")
	case !d.sshFound:
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
