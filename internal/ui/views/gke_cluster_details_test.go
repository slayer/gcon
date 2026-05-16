package views

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/confirm"
	"github.com/slayer/gcon/internal/ui/components/tabs"
	"github.com/slayer/gcon/internal/ui/context"
)

func gkeDetailsFixture(mode string) *GKEClusterDetailsView {
	v := NewGKEClusterDetailsView("proj", "us-central1", "prod", nil, nil, nil)
	v.SetSize(160, 40)
	v.loading = false
	v.details = &gcp.ClusterDetails{
		Cluster: gcp.Cluster{
			Name:                "prod",
			Location:            "us-central1",
			LocationType:        "region",
			Mode:                mode,
			Status:              "RUNNING",
			MasterVersion:       "1.30.5-gke.1014001",
			NodeVersion:         "1.30.5-gke.1014001",
			NodeVersionsUniform: true,
			Network:             "default",
			Subnetwork:          "default-uscent1",
			ReleaseChannel:      "REGULAR",
			Endpoint:            "34.123.45.67",
			PrivateCluster:      false,
			CreatedAt:           "2025-08-12T14:03:00Z",
		},
		ClusterIPv4CIDR:      "10.4.0.0/14",
		ServicesIPv4CIDR:     "10.8.0.0/20",
		Addons:               gcp.AddonsSummary{HTTPLoadBalancing: true, PersistentDiskCSI: true},
		WorkloadIdentityPool: "prod.svc.id.goog",
		DatabaseEncrypted:    true,
		DatabaseKMSKey:       "projects/p/locations/global/keyRings/r/cryptoKeys/my-key",
		NodePools: []gcp.NodePool{
			{Name: "default", MachineType: "e2-medium", NodeCount: 3, AutoscalingOn: true, AutoscalingMin: 1, AutoscalingMax: 10, NodeVersion: "1.30.5-gke.1014001", Status: "RUNNING", AutoUpgrade: true, AutoRepair: true},
		},
	}
	return v
}

func TestGKEClusterDetails_OverviewRendersStandard(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	v.tabs.SetActiveByID("overview")
	out := v.View()
	assert.Contains(t, out, "Standard")
	assert.Contains(t, out, "us-central1 (regional)")
	assert.Contains(t, out, "default")
	assert.Contains(t, out, "default-uscent1")
	assert.Contains(t, out, "REGULAR")
	assert.Contains(t, out, "prod.svc.id.goog")
	assert.Contains(t, out, "ENCRYPTED (key: my-key)")
	assert.Contains(t, out, "HTTP load balancing: Enabled")
	assert.Contains(t, out, "Network policy: Disabled")
}

func TestGKEClusterDetails_OverviewRendersAutopilot(t *testing.T) {
	v := gkeDetailsFixture("AUTOPILOT")
	v.tabs.SetActiveByID("overview")
	out := v.View()
	assert.Contains(t, out, "Autopilot")
}

func TestGKEClusterDetails_NodePoolsRendersStandard(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	v.tabs.SetActiveByID("nodepools")
	v.refreshNodePoolsTable()
	out := v.View()
	assert.Contains(t, out, "default")
	assert.Contains(t, out, "e2-medium")
	assert.Contains(t, out, "on (1–10)")
	assert.Contains(t, out, "1.30.5-gke.1014001")
	// Status cell must render — regression: bubbles/table byte-truncates
	// ANSI-styled cells mid-escape, stripping the visible text.
	assert.Contains(t, out, "RUNNING")
}

func TestGKEClusterDetails_NodePoolsRendersAutopilotSuffix(t *testing.T) {
	v := gkeDetailsFixture("AUTOPILOT")
	// Inject a system-managed pool
	v.details.NodePools = []gcp.NodePool{
		{Name: "default-pool", MachineType: "e2-medium", NodeCount: 1, Status: "RUNNING"},
	}
	v.tabs.SetActiveByID("nodepools")
	v.refreshNodePoolsTable()
	out := v.View()
	assert.Contains(t, out, "default-pool [managed by Autopilot]")
	assert.Contains(t, out, "—") // autoscale / version cells
}

// typeStringIntoDialog feeds each rune in s as an individual tea.KeyMsg.
// Mirrors what bubbletea does for character input.
func typeStringIntoDialog(d *confirm.TypeConfirmDialog, s string) {
	for _, r := range s {
		d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func TestGKEClusterDetails_DeleteDialogGatesOnName(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")

	// User presses D to open the dialog
	cmd := v.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	_ = cmd
	assert.True(t, v.showConfirm, "expected delete dialog to open on D")
	assert.NotNil(t, v.confirmDialog)
	assert.True(t, v.HasTextInputFocused(), "dialog's text input should report focus")

	// Pressing Enter with no input should be a no-op (returns nil cmd)
	noOpCmd := v.confirmDialog.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, noOpCmd, "empty input must not confirm")

	// Type the WRONG name: still no submit
	typeStringIntoDialog(v.confirmDialog, "not-prod")
	wrongCmd := v.confirmDialog.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, wrongCmd, "wrong name must not confirm")

	// Reset the dialog by re-opening it for a clean text input
	v.showConfirm = false
	v.confirmDialog = nil
	v.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	assert.NotNil(t, v.confirmDialog)

	// Type the CORRECT cluster name and submit
	typeStringIntoDialog(v.confirmDialog, "prod")
	submitCmd := v.confirmDialog.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, submitCmd, "correct name + Enter should emit a cmd")

	// The cmd returns confirm.TypeConfirmMsg{}; the view's Update routes it
	// to GKEClusterDeleteRequestMsg with project / location / name.
	msg := submitCmd()
	_, isConfirm := msg.(confirm.TypeConfirmMsg)
	assert.True(t, isConfirm, "expected confirm.TypeConfirmMsg, got %T", msg)

	// Feed the confirm msg back through the view's Update; the view should
	// close the dialog and emit GKEClusterDeleteRequestMsg.
	cmd2 := v.Update(msg)
	assert.False(t, v.showConfirm, "dialog should close after confirm")
	assert.NotNil(t, cmd2, "view should return a request cmd")
	reqMsg := cmd2()
	req, ok := reqMsg.(GKEClusterDeleteRequestMsg)
	assert.True(t, ok, "expected GKEClusterDeleteRequestMsg, got %T", reqMsg)
	assert.Equal(t, "proj", req.ProjectID)
	assert.Equal(t, "us-central1", req.Location)
	assert.Equal(t, "prod", req.Name)
	assert.True(t, v.deleting, "view should be in deleting state after request")
}

func TestGKEClusterDetails_DeleteDialogCancel(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	v.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	assert.True(t, v.showConfirm)

	cancelCmd := v.confirmDialog.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.NotNil(t, cancelCmd, "esc should emit cancel cmd")
	msg := cancelCmd()
	_, isCancel := msg.(confirm.TypeCancelMsg)
	assert.True(t, isCancel)

	// Feed cancel into the view; dialog should close.
	v.Update(msg)
	assert.False(t, v.showConfirm)
	assert.False(t, v.deleting)
}

func TestGKEClusterDetails_SetErrorClearsDeleting(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	v.deleting = true
	v.SetError(errors.New("api: insufficient permission")) //nolint:err113 // test fixture
	assert.False(t, v.deleting)
	assert.NotNil(t, v.err)
}

func TestGKEClusterDetails_HasTextInputFocusedWhenDialogClosed(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	assert.False(t, v.HasTextInputFocused(), "no text input when dialog is closed")
}

func TestGKEClusterDetails_AllFiveTabs(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	ids := []string{"overview", "nodepools", "nodes", "observability", "logs"}
	for _, id := range ids {
		v.tabs.SetActiveByID(id)
		out := v.View()
		assert.NotEmpty(t, out, "tab %s should render non-empty content", id)
	}
}

func TestGKEClusterDetails_LazyObservability(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	assert.Nil(t, v.observability, "observability should be nil before first visit")
	v.tabs.SetActiveByID("observability")
	v.Update(tabs.TabChangedMsg{})
	assert.NotNil(t, v.observability, "observability should be lazy-created on first visit")
}

func TestGKEClusterDetails_TabActiveToggling(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	v.tabs.SetActiveByID("observability")
	v.Update(tabs.TabChangedMsg{})
	assert.True(t, v.observability.tabActive)

	v.tabs.SetActiveByID("logs")
	v.Update(tabs.TabChangedMsg{})
	assert.False(t, v.observability.tabActive, "leaving observability must clear its tabActive flag")
	assert.True(t, v.logs.tabActive)
}

func TestGKEClusterDetails_RefreshOnObservability(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	v.tabs.SetActiveByID("observability")
	v.Update(tabs.TabChangedMsg{})
	require.NotNil(t, v.observability)
	v.observability.loading = false
	v.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	assert.True(t, v.observability.loading, "r on observability tab should kick a refresh (loading=true)")
}

// TestGKEClusterDetails_SetContextPropagatesSize guards against a regression
// where SetContext was a no-op. The app routes sizing through SetContext —
// without forwarding to SetSize, v.width / v.height stay at zero and the
// Observability sub-view's charts clamp to the 10-column minimum.
func TestGKEClusterDetails_SetContextPropagatesSize(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	v.width = 0
	v.height = 0
	v.SetContext(&context.ProgramContext{ContentWidth: 180, ContentHeight: 60})
	assert.Equal(t, 180, v.width, "SetContext must forward ContentWidth into SetSize")
	assert.Equal(t, 60, v.height, "SetContext must forward ContentHeight into SetSize")

	v.tabs.SetActiveByID("observability")
	v.Update(tabs.TabChangedMsg{})
	require.NotNil(t, v.observability)
	// observability.SetSize is called with v.width-4. width=180-4=176 → cw=174.
	assert.GreaterOrEqual(t, v.observability.width, 100,
		"observability width must reflect the propagated context, not the 10-col clamp")
}

// TestGKEClusterDetails_ObsRangeKeysGoToSubView guards against a regression
// where the parent's `1`–`5` tab-switch case stole the range-selector keys
// from the Observability sub-view.
func TestGKEClusterDetails_ObsRangeKeysGoToSubView(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	v.SetContext(&context.ProgramContext{ContentWidth: 180, ContentHeight: 60})
	v.tabs.SetActiveByID("observability")
	v.Update(tabs.TabChangedMsg{})
	require.NotNil(t, v.observability)

	// Press '3' → should set range to 24h (index 2), NOT switch parent tab.
	startIdx := v.observability.rangeIdx
	v.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	assert.NotEqual(t, startIdx, v.observability.rangeIdx,
		"'3' on observability must update rangeIdx in the sub-view")
	assert.Equal(t, "observability", v.tabs.ActiveTab().ID,
		"'3' on observability must NOT switch parent tabs")
}

// TestGKEClusterDetails_LogsViewportScrollKeysFallToViewport guards against a
// regression where j/k on the Logs tab got routed into the (no-op) sub-view
// instead of scrolling the parent viewport that contains the log body.
func TestGKEClusterDetails_LogsViewportScrollKeysFallToViewport(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	v.SetContext(&context.ProgramContext{ContentWidth: 180, ContentHeight: 60})
	v.tabs.SetActiveByID("logs")
	v.Update(tabs.TabChangedMsg{})
	require.NotNil(t, v.logs)

	// 'j' should not be claimed by isGKESubViewActionKey for logs.
	assert.False(t, isGKESubViewActionKey("logs", "j"),
		"j is a viewport scroll key, not a logs action — must fall through to viewport")
	// 'I' is a logs-owned severity toggle, must NOT fall through.
	assert.True(t, isGKESubViewActionKey("logs", "I"),
		"I (info toggle) must be routed to logs sub-view, not viewport")
}
