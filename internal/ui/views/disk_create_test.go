package views

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

var errDiskCreateFailed = errors.New("disk create failed")

func TestDiskCreateView_SetErrorResetsState(t *testing.T) {
	view := NewDiskCreateView("project-id", "snapshot-name", 10, nil)
	view.State = createViewStateSaving

	view.SetError(errDiskCreateFailed)

	assert.Equal(t, createViewStateForm, view.State)
	assert.Equal(t, errDiskCreateFailed, view.Err)
	assert.Contains(t, view.View(), "Error: "+errDiskCreateFailed.Error())
}
