package views

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errSubnetCreateFailed = errors.New("subnet create failed")

func TestNewSubnetCreateView(t *testing.T) {
	view := NewSubnetCreateView("my-project", nil)

	assert.NotNil(t, view)
	assert.NotNil(t, view.Form)
	assert.Equal(t, "my-project", view.projectID)
	assert.Nil(t, view.computeClient)
}

func TestSubnetCreateView_FormFields(t *testing.T) {
	view := NewSubnetCreateView("my-project", nil)

	expectedFields := []string{
		"name", "description", "network", "region",
		"cidr_range", "purpose", "stack_type",
		"private_google_access", "flow_logs",
	}

	for _, fieldID := range expectedFields {
		field := view.Form.GetField(fieldID)
		require.NotNilf(t, field, "field %q should exist", fieldID)
	}
}

func TestSubnetCreateView_HandleSubmit_ValidationFails(t *testing.T) {
	view := NewSubnetCreateView("my-project", nil)
	// Form has required fields (name, network, region, cidr_range) that are empty,
	// so validation should fail and handleSubmit returns nil.
	cmd := view.handleSubmit()
	assert.Nil(t, cmd)
}

func TestSubnetCreateView_SetErrorResetsState(t *testing.T) {
	view := NewSubnetCreateView("my-project", nil)
	view.State = createViewStateSaving

	errFailed := fmt.Errorf("test: %w", errSubnetCreateFailed)
	view.SetError(errFailed)

	assert.Equal(t, createViewStateForm, view.State)
	assert.Equal(t, errFailed, view.Err)
	assert.Contains(t, view.View(), "Error: "+errFailed.Error())
}
