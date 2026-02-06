package views

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

var errSnapshotCreateFailed = errors.New("snapshot create failed")

func TestSnapshotCreateView_SetErrorResetsState(t *testing.T) {
	view := NewSnapshotCreateView("project-id", "disk-name", "zone-a", "", nil)
	view.State = createViewStateSaving

	view.SetError(errSnapshotCreateFailed)

	assert.Equal(t, createViewStateForm, view.State)
	assert.Equal(t, errSnapshotCreateFailed, view.Err)
	assert.Contains(t, view.View(), "Error: "+errSnapshotCreateFailed.Error())
}
