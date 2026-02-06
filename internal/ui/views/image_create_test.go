package views

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

var errImageCreateFailed = errors.New("image create failed")

func TestImageCreateView_SetErrorResetsState(t *testing.T) {
	view := NewImageCreateView("project-id", "disk-name", "zone-a", "", nil)
	view.state = imageCreateStateSaving

	view.SetError(errImageCreateFailed)

	assert.Equal(t, imageCreateStateForm, view.state)
	assert.Equal(t, errImageCreateFailed, view.err)
	assert.Contains(t, view.View(), "Error: "+errImageCreateFailed.Error())
}
