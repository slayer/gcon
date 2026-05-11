package views

import "github.com/slayer/gcon/internal/ssh"

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
