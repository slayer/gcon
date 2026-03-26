package views

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/slayer/gcon/internal/gcp"
)

func TestGolden_InstancesView_Loaded(t *testing.T) {
	view := NewInstancesView("test-project")
	ctx := goldenContext()
	view.SetContext(ctx)
	view.Update(instancesLoadedMsg{instances: testInstances()})

	golden.RequireEqual(t, []byte(view.View()))
}

func TestGolden_InstancesView_Loading(t *testing.T) {
	view := NewInstancesView("test-project")
	ctx := goldenContext()
	view.SetContext(ctx)
	// View starts in loading state with nil client — shows "Initializing..." message

	golden.RequireEqual(t, []byte(view.View()))
}

func TestGolden_InstancesView_Error(t *testing.T) {
	view := NewInstancesView("test-project")
	ctx := goldenContext()
	view.SetContext(ctx)
	view.Update(instancesErrorMsg{err: fmt.Errorf("compute.instances.list: permission denied")})

	golden.RequireEqual(t, []byte(view.View()))
}

func TestGolden_InstancesView_Empty(t *testing.T) {
	view := NewInstancesView("test-project")
	ctx := goldenContext()
	view.SetContext(ctx)
	view.Update(instancesLoadedMsg{instances: nil})

	golden.RequireEqual(t, []byte(view.View()))
}

func TestGolden_InstancesView_ActionMenu(t *testing.T) {
	view := NewInstancesView("test-project")
	ctx := goldenContext()
	view.SetContext(ctx)
	view.Update(instancesLoadedMsg{instances: testInstances()})
	// '.' opens action menu only when a row is selected (table has rows after load)
	sendKey(view, '.')

	golden.RequireEqual(t, []byte(view.View()))
}

func TestGolden_InstancesView_DeleteConfirm(t *testing.T) {
	view := NewInstancesView("test-project")
	ctx := goldenContext()
	view.SetContext(ctx)
	instances := testInstances()
	view.Update(instancesLoadedMsg{instances: instances})
	// Simulate receiving delete details — triggers type-to-confirm dialog
	view.Update(instanceDeleteDetailsMsg{
		instance: &instances[0],
		details: &gcp.InstanceDetails{
			Name:   instances[0].Name,
			Zone:   instances[0].Zone,
			Status: instances[0].Status,
		},
	})

	golden.RequireEqual(t, []byte(view.View()))
}
