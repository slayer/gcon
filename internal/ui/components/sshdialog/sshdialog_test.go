package sshdialog

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gconssh "github.com/slayer/gcon/internal/ssh"
)

// Helper: build a dialog with everything resolvable so we can test field
// behavior without depending on what's on $PATH.
func newTestDialog(t *testing.T, params Params) *Dialog {
	t.Helper()
	d := New(params)
	d.gcloudFound = true
	d.sshFound = true
	d.binaryErr = nil
	d.recomputeMethodLock()
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

func TestView_RendersInstanceName(t *testing.T) {
	d := newTestDialog(t, Params{Project: "p", Zone: "z", Instance: "my-vm",
		InternalIP: "10.0.0.5", ExternalIP: ""})
	d.SetSize(80, 24)
	out := d.View()
	assert.Contains(t, out, "my-vm")
}

func TestUpdate_AcceptsBlinkMsgs(t *testing.T) {
	d := newTestDialog(t, Params{Project: "p", Zone: "z", Instance: "vm"})
	// An unknown message type must be accepted and produce no command;
	// this guards the rule that Update takes tea.Msg, not just tea.KeyMsg.
	_, cmd := d.Update(struct{}{})
	assert.Nil(t, cmd)
}

func TestEnterFromTextField_AdvancesFocusNotSubmits(t *testing.T) {
	d := newTestDialog(t, Params{
		Project: "p", Zone: "z", Instance: "vm",
		InternalIP: "10.0.0.5", ExternalIP: "34.1.2.3",
	})
	// Focus starts on fieldUser. Press Enter — should advance, not submit.
	startFocus := d.focus
	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, cmd, "Enter on a text field must not return a submit cmd")
	assert.NotEqual(t, startFocus, d.focus, "Enter must advance focus")
}
