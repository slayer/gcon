package views

import (
	"testing"

	"github.com/slayer/gcon/internal/ui/context"
	"github.com/stretchr/testify/assert"
)

func TestInstancesView_NewInstancesView(t *testing.T) {
	v := NewInstancesView("test-project")

	assert.NotNil(t, v)
	assert.Equal(t, "test-project", v.projectID)
	assert.True(t, v.loading, "View should start in loading state")
}

func TestInstancesView_RenderLoading(t *testing.T) {
	v := NewInstancesView("test-project")
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30}
	v.SetContext(ctx)

	// renderLoading should return a simple loading message
	// Height enforcement is now handled at the app level
	output := v.renderLoading("Loading...")

	assert.Contains(t, output, "Loading...")
	assert.Contains(t, output, v.spinner.View())
}
