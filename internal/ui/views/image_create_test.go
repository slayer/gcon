package views

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

var errImageCreateFailed = errors.New("image create failed")

func TestImageCreateView_SetErrorResetsState(t *testing.T) {
	view := NewImageCreateView("project-id", "disk-name", "zone-a", "", nil)
	view.State = createViewStateSaving

	view.SetError(errImageCreateFailed)

	assert.Equal(t, createViewStateForm, view.State)
	assert.Equal(t, errImageCreateFailed, view.Err)
	assert.Contains(t, view.View(), "Error: "+errImageCreateFailed.Error())
}
