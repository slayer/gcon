package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/inputdialog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIAMPolicyView_RebuildTables(t *testing.T) {
	v := NewIAMPolicyView("test-project")

	policy := &gcp.IAMPolicy{
		Bindings: []gcp.IAMBinding{
			{Role: "roles/editor", Members: []string{"serviceAccount:sa@proj.iam.gserviceaccount.com"}},
			{Role: "roles/viewer", Members: []string{"user:alice@example.com", "group:admins@example.com"}},
			{Role: "roles/owner", Members: []string{"user:alice@example.com"}},
		},
		Version: 3,
		Etag:    "test-etag",
	}

	v.policy = policy
	v.rebuildTables()

	t.Run("member entries are built correctly", func(t *testing.T) {
		// 3 unique members across all bindings
		assert.Len(t, v.memberEntries, 3)

		// Sorted alphabetically by fullMember
		assert.Equal(t, "group:admins@example.com", v.memberEntries[0].fullMember)
		assert.Equal(t, "group", v.memberEntries[0].typeName)
		assert.Equal(t, "admins@example.com", v.memberEntries[0].identity)
		assert.Equal(t, []memberRole{{role: "roles/viewer"}}, v.memberEntries[0].roles)

		// serviceAccount → "sa" short type
		assert.Equal(t, "serviceAccount:sa@proj.iam.gserviceaccount.com", v.memberEntries[1].fullMember)
		assert.Equal(t, "sa", v.memberEntries[1].typeName)

		// alice has 2 roles
		assert.Equal(t, "user:alice@example.com", v.memberEntries[2].fullMember)
		assert.Equal(t, "user", v.memberEntries[2].typeName)
		assert.Len(t, v.memberEntries[2].roles, 2)
		// Roles sorted
		assert.Equal(t, "roles/owner", v.memberEntries[2].roles[0].role)
		assert.Equal(t, "roles/viewer", v.memberEntries[2].roles[1].role)
	})
}

func TestIAMPolicyView_HasTextInputFocused(t *testing.T) {
	v := NewIAMPolicyView("test-project")

	t.Run("false by default", func(t *testing.T) {
		assert.False(t, v.HasTextInputFocused())
	})

	t.Run("true when input dialog is shown", func(t *testing.T) {
		v.showInput = true
		v.inputDialog = nil
		// Still false — inputDialog must not be nil
		assert.False(t, v.HasTextInputFocused())
	})
}

func TestIAMPolicyView_SetError(t *testing.T) {
	v := NewIAMPolicyView("test-project")
	v.loading = true
	v.showOverlay = true
	v.showInput = true
	v.showConfirm = true

	testErr := assert.AnError
	v.SetError(testErr)

	assert.Equal(t, testErr, v.err)
	assert.False(t, v.loading)
	assert.False(t, v.showOverlay)
	assert.False(t, v.showInput)
	assert.False(t, v.showConfirm)
}

func TestIAMPolicyView_FormatRolesColumn(t *testing.T) {
	tests := []struct {
		roles    []memberRole
		expected string
	}{
		{[]memberRole{{role: "roles/viewer"}}, "viewer"},
		{[]memberRole{{role: "roles/viewer"}, {role: "roles/editor"}}, "viewer, editor"},
		{[]memberRole{{role: "roles/a"}, {role: "roles/b"}, {role: "roles/c"}}, "a, b, c"},
		{[]memberRole{{role: "projects/p/roles/custom"}}, "projects/p/roles/custom"},
		// With condition titles
		{[]memberRole{{role: "roles/viewer", conditionTitle: "Expires 2026"}}, "viewer (Expires 2026)"},
		{[]memberRole{{role: "roles/viewer"}, {role: "roles/viewer", conditionTitle: "Temp"}}, "viewer, viewer (Temp)"},
	}

	for _, tt := range tests {
		result := formatRolesColumn(tt.roles)
		assert.Equal(t, tt.expected, result)
	}
}

func TestIAMPolicyView_ShortRoleName(t *testing.T) {
	assert.Equal(t, "viewer", shortRoleName("roles/viewer"))
	assert.Equal(t, "editor", shortRoleName("roles/editor"))
	assert.Equal(t, "projects/p/roles/custom", shortRoleName("projects/p/roles/custom"))
}

func TestIAMPolicyView_TruncatePreview(t *testing.T) {
	members := []string{"user:a@x.com", "user:b@x.com", "user:c@x.com"}
	preview := truncatePreview(members, 30)
	assert.LessOrEqual(t, len(preview), 30)
	assert.Contains(t, preview, "...")

	short := []string{"user:a@x.com"}
	shortPreview := truncatePreview(short, 30)
	assert.Equal(t, "user:a@x.com", shortPreview)
}

func TestIAMPolicyView_ValidateIAMMember(t *testing.T) {
	tests := []struct {
		value   string
		wantErr bool
	}{
		{"user:alice@example.com", false},
		{"serviceAccount:sa@proj.iam.gserviceaccount.com", false},
		{"group:admins@example.com", false},
		{"domain:example.com", false},
		{"deleted:user:old@example.com", false},
		{"invalid-member", true},
		{"user:", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			err := validateIAMMember(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIAMPolicyView_ValidateIAMRole(t *testing.T) {
	tests := []struct {
		value   string
		wantErr bool
	}{
		{"roles/viewer", false},
		{"roles/editor", false},
		{"projects/my-project/roles/custom", false},
		{"organizations/123/roles/orgRole", false},
		{"invalid-role", true},
		{"", true},
		{"roles/", true},         // empty ID segment after prefix
		{"projects/", true},      // empty ID segment after prefix
		{"organizations/", true}, // empty ID segment after prefix
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			err := validateIAMRole(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIAMPolicyView_FindMemberEntry(t *testing.T) {
	v := NewIAMPolicyView("test-project")
	v.memberEntries = []memberEntry{
		{fullMember: "user:alice@example.com", typeName: "user", identity: "alice@example.com", roles: []memberRole{{role: "roles/viewer"}}},
		{fullMember: "user:bob@example.com", typeName: "user", identity: "bob@example.com", roles: []memberRole{{role: "roles/editor"}}},
	}

	t.Run("finds existing member", func(t *testing.T) {
		entry := v.findMemberEntry("user:alice@example.com")
		assert.NotNil(t, entry)
		assert.Equal(t, "alice@example.com", entry.identity)
	})

	t.Run("returns nil for non-existent member", func(t *testing.T) {
		entry := v.findMemberEntry("user:charlie@example.com")
		assert.Nil(t, entry)
	})
}

func TestIAMPolicyView_FindBinding(t *testing.T) {
	v := NewIAMPolicyView("test-project")
	v.policy = &gcp.IAMPolicy{
		Bindings: []gcp.IAMBinding{
			{Role: "roles/viewer", Members: []string{"user:a@x.com"}},
			{Role: "roles/editor", Members: []string{"user:b@x.com"}},
		},
	}

	t.Run("finds existing binding", func(t *testing.T) {
		binding := v.findBinding("roles/viewer")
		assert.NotNil(t, binding)
		assert.Equal(t, "roles/viewer", binding.Role)
	})

	t.Run("returns nil for non-existent role", func(t *testing.T) {
		binding := v.findBinding("roles/owner")
		assert.Nil(t, binding)
	})

	t.Run("finds conditioned binding by composite key", func(t *testing.T) {
		v.policy = &gcp.IAMPolicy{
			Bindings: []gcp.IAMBinding{
				{Role: "roles/viewer", Members: []string{"user:a@x.com"}},
				{Role: "roles/viewer", ConditionTitle: "Expires 2026", Members: []string{"user:b@x.com"}},
			},
		}
		uncond := v.findBinding("roles/viewer")
		assert.NotNil(t, uncond)
		assert.Equal(t, "", uncond.ConditionTitle)

		cond := v.findBinding("roles/viewer|Expires 2026")
		assert.NotNil(t, cond)
		assert.Equal(t, "Expires 2026", cond.ConditionTitle)
	})
}

func TestIAMPolicyView_AddMemberFlow(t *testing.T) {
	v := NewIAMPolicyView("test-project")
	v.loading = false
	v.policy = &gcp.IAMPolicy{
		Bindings: []gcp.IAMBinding{
			{Role: "roles/viewer", Members: []string{"user:alice@example.com"}},
			{Role: "roles/editor", Members: []string{"user:bob@example.com"}},
		},
	}
	v.rebuildTables()

	// Switch to "By Role" tab
	v.tabsComp.SetActiveByID(iamPolicyTabByRole)
	v.Table = v.activeTable()

	// Set focus to table (RegionViewport)
	v.focusMgr.SetActive(iamPolicyRegionTable)

	t.Run("pressing 'a' opens input dialog", func(t *testing.T) {
		// Press 'a' — should open input dialog to add member to selected role
		cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

		assert.True(t, v.showInput, "showInput should be true")
		assert.NotNil(t, v.inputDialog, "inputDialog should be created")
		assert.Equal(t, "roles/viewer", v.pendingRole)
		assert.Equal(t, "", v.pendingMember)

		// The cmd should be textinput.Blink for cursor animation
		assert.NotNil(t, cmd, "cmd from handleAdd should not be nil")
	})

	t.Run("confirming input emits AddIAMBindingMsg", func(t *testing.T) {
		// Simulate InputConfirmMsg as if user typed and pressed Enter
		cmd := v.Update(inputdialog.InputConfirmMsg{Value: "user:newuser@example.com"})

		assert.False(t, v.showInput, "showInput should be false after confirm")

		// The cmd should produce AddIAMBindingMsg
		require.NotNil(t, cmd, "cmd from handleInputConfirm must not be nil")

		// Execute the cmd to get the message
		msg := cmd()
		addMsg, ok := msg.(AddIAMBindingMsg)
		require.True(t, ok, "cmd should produce AddIAMBindingMsg, got %T", msg)

		assert.Equal(t, "test-project", addMsg.ProjectID)
		assert.Equal(t, "roles/viewer", addMsg.Role)
		assert.Equal(t, "", addMsg.ConditionTitle)
		assert.Equal(t, "user:newuser@example.com", addMsg.Member)
	})
}

func TestIAMPolicyView_RemoveMemberFlow(t *testing.T) {
	v := NewIAMPolicyView("test-project")
	v.loading = false
	v.policy = &gcp.IAMPolicy{
		Bindings: []gcp.IAMBinding{
			{Role: "roles/viewer", Members: []string{"user:alice@example.com", "user:bob@example.com"}},
		},
	}
	v.rebuildTables()

	// Switch to "By Role" tab
	v.tabsComp.SetActiveByID(iamPolicyTabByRole)
	v.Table = v.activeTable()

	// Set focus to table
	v.focusMgr.SetActive(iamPolicyRegionTable)

	t.Run("opening overlay then pressing 'd' shows confirm dialog", func(t *testing.T) {
		// Press Enter to open overlay showing role's members
		v.Update(tea.KeyMsg{Type: tea.KeyEnter})
		assert.True(t, v.showOverlay, "overlay should be open")
		assert.Equal(t, "roles/viewer", v.overlayCtx.role)
		assert.Len(t, v.overlayItems, 2)

		// Press 'd' in overlay to remove selected member
		v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
		assert.True(t, v.showConfirm, "confirm dialog should be shown")
		assert.NotNil(t, v.confirmDialog)
		// First member is at cursor 0
		assert.Equal(t, v.overlayItems[0], v.pendingMember)
	})

	t.Run("confirming removal emits RemoveIAMBindingMsg", func(t *testing.T) {
		// Simulate pressing 'y' in confirm dialog
		cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

		// The confirm dialog should produce ConfirmMsg
		// which the view catches and returns RemoveIAMBindingMsg cmd
		// But wait — the 'y' key goes to the confirm dialog, which returns a cmd producing ConfirmMsg
		// So we need to execute that cmd first
		if cmd != nil {
			msg := cmd()
			// This should be confirm.ConfirmMsg
			cmd = v.Update(msg)
		}

		require.NotNil(t, cmd, "cmd from handleConfirm must not be nil")
		msg := cmd()
		removeMsg, ok := msg.(RemoveIAMBindingMsg)
		require.True(t, ok, "cmd should produce RemoveIAMBindingMsg, got %T", msg)

		assert.Equal(t, "test-project", removeMsg.ProjectID)
		assert.Equal(t, "roles/viewer", removeMsg.Role)
		assert.Equal(t, "", removeMsg.ConditionTitle)
		assert.NotEmpty(t, removeMsg.Member)
	})
}

func TestIAMPolicyView_IsMenuOpen(t *testing.T) {
	v := NewIAMPolicyView("test-project")

	assert.False(t, v.IsMenuOpen())

	v.menuOpen = true
	assert.True(t, v.IsMenuOpen())

	v.menuOpen = false
	v.showOverlay = true
	assert.True(t, v.IsMenuOpen())

	v.showOverlay = false
	v.showInput = true
	assert.True(t, v.IsMenuOpen())

	v.showInput = false
	v.showConfirm = true
	assert.True(t, v.IsMenuOpen())
}
