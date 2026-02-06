package views

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

var errSnapshotCreateFailed = errors.New("snapshot create failed")

func TestSnapshotCreateView_SetErrorResetsState(t *testing.T) {
	view := NewSnapshotCreateView("project-id", "disk-name", "zone-a", "", nil)
	view.state = snapshotCreateStateSaving

	view.SetError(errSnapshotCreateFailed)

	assert.Equal(t, snapshotCreateStateForm, view.state)
	assert.Equal(t, errSnapshotCreateFailed, view.err)
	assert.Contains(t, view.View(), "Error: "+errSnapshotCreateFailed.Error())
}
