package views

import (
	"testing"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
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
		assert.Equal(t, []string{"roles/viewer"}, v.memberEntries[0].roles)

		// serviceAccount → "sa" short type
		assert.Equal(t, "serviceAccount:sa@proj.iam.gserviceaccount.com", v.memberEntries[1].fullMember)
		assert.Equal(t, "sa", v.memberEntries[1].typeName)

		// alice has 2 roles
		assert.Equal(t, "user:alice@example.com", v.memberEntries[2].fullMember)
		assert.Equal(t, "user", v.memberEntries[2].typeName)
		assert.Len(t, v.memberEntries[2].roles, 2)
		// Roles sorted
		assert.Equal(t, "roles/owner", v.memberEntries[2].roles[0])
		assert.Equal(t, "roles/viewer", v.memberEntries[2].roles[1])
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
		roles    []string
		expected string
	}{
		{[]string{"roles/viewer"}, "viewer"},
		{[]string{"roles/viewer", "roles/editor"}, "2 roles"},
		{[]string{"roles/a", "roles/b", "roles/c"}, "3 roles"},
		{[]string{"projects/p/roles/custom"}, "projects/p/roles/custom"},
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
		{fullMember: "user:alice@example.com", typeName: "user", identity: "alice@example.com", roles: []string{"roles/viewer"}},
		{fullMember: "user:bob@example.com", typeName: "user", identity: "bob@example.com", roles: []string{"roles/editor"}},
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
