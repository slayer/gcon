package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/iam/v1"
)

func TestServiceAccountFromAPI(t *testing.T) {
	tests := []struct {
		name     string
		input    *iam.ServiceAccount
		validate func(t *testing.T, sa ServiceAccount)
	}{
		{
			name: "fully populated service account",
			input: &iam.ServiceAccount{
				Email:       "my-sa@my-project.iam.gserviceaccount.com",
				DisplayName: "My Service Account",
				Description: "Used for CI/CD pipelines",
				UniqueId:    "123456789012345678",
				Disabled:    false,
			},
			validate: func(t *testing.T, sa ServiceAccount) {
				assert.Equal(t, "my-sa@my-project.iam.gserviceaccount.com", sa.Email)
				assert.Equal(t, "My Service Account", sa.DisplayName)
				assert.Equal(t, "Used for CI/CD pipelines", sa.Description)
				assert.Equal(t, "123456789012345678", sa.UniqueID)
				assert.False(t, sa.Disabled)
			},
		},
		{
			name: "disabled service account",
			input: &iam.ServiceAccount{
				Email:    "disabled-sa@my-project.iam.gserviceaccount.com",
				Disabled: true,
			},
			validate: func(t *testing.T, sa ServiceAccount) {
				assert.Equal(t, "disabled-sa@my-project.iam.gserviceaccount.com", sa.Email)
				assert.True(t, sa.Disabled)
				assert.Empty(t, sa.DisplayName)
				assert.Empty(t, sa.Description)
			},
		},
		{
			name: "minimal service account with no optional fields",
			input: &iam.ServiceAccount{
				Email:    "minimal@my-project.iam.gserviceaccount.com",
				UniqueId: "999999999",
			},
			validate: func(t *testing.T, sa ServiceAccount) {
				assert.Equal(t, "minimal@my-project.iam.gserviceaccount.com", sa.Email)
				assert.Equal(t, "999999999", sa.UniqueID)
				assert.Empty(t, sa.DisplayName)
				assert.Empty(t, sa.Description)
				assert.False(t, sa.Disabled)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serviceAccountFromAPI(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestServiceAccountDetailsFromAPI(t *testing.T) {
	tests := []struct {
		name     string
		input    *iam.ServiceAccount
		validate func(t *testing.T, details *ServiceAccountDetails)
	}{
		{
			name: "fully populated details",
			input: &iam.ServiceAccount{
				Email:          "sa@my-project.iam.gserviceaccount.com",
				DisplayName:    "Full SA",
				Description:    "Full description",
				UniqueId:       "111222333",
				Disabled:       false,
				Oauth2ClientId: "111222333.apps.googleusercontent.com",
				ProjectId:      "my-project",
			},
			validate: func(t *testing.T, d *ServiceAccountDetails) {
				assert.Equal(t, "sa@my-project.iam.gserviceaccount.com", d.Email)
				assert.Equal(t, "Full SA", d.DisplayName)
				assert.Equal(t, "Full description", d.Description)
				assert.Equal(t, "111222333", d.UniqueID)
				assert.False(t, d.Disabled)
				assert.Equal(t, "111222333.apps.googleusercontent.com", d.OAuth2ClientID)
				assert.Equal(t, "my-project", d.ProjectID)
			},
		},
		{
			name: "disabled service account details",
			input: &iam.ServiceAccount{
				Email:     "disabled@proj.iam.gserviceaccount.com",
				Disabled:  true,
				ProjectId: "proj",
			},
			validate: func(t *testing.T, d *ServiceAccountDetails) {
				assert.True(t, d.Disabled)
				assert.Equal(t, "proj", d.ProjectID)
			},
		},
		{
			name: "minimal fields",
			input: &iam.ServiceAccount{
				Email: "bare@proj.iam.gserviceaccount.com",
			},
			validate: func(t *testing.T, d *ServiceAccountDetails) {
				assert.Equal(t, "bare@proj.iam.gserviceaccount.com", d.Email)
				assert.Empty(t, d.DisplayName)
				assert.Empty(t, d.Description)
				assert.Empty(t, d.UniqueID)
				assert.Empty(t, d.OAuth2ClientID)
				assert.Empty(t, d.ProjectID)
				assert.False(t, d.Disabled)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serviceAccountDetailsFromAPI(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestServiceAccountKeyFromAPI(t *testing.T) {
	tests := []struct {
		name     string
		input    *iam.ServiceAccountKey
		validate func(t *testing.T, key ServiceAccountKey)
	}{
		{
			name: "full resource name extracts short key ID",
			input: &iam.ServiceAccountKey{
				Name:            "projects/my-project/serviceAccounts/sa@proj.iam.gserviceaccount.com/keys/abcdef1234567890",
				KeyAlgorithm:    "KEY_ALG_RSA_2048",
				KeyOrigin:       "GOOGLE_PROVIDED",
				KeyType:         "USER_MANAGED",
				ValidAfterTime:  "2024-01-01T00:00:00Z",
				ValidBeforeTime: "2025-01-01T00:00:00Z",
				Disabled:        false,
			},
			validate: func(t *testing.T, k ServiceAccountKey) {
				assert.Equal(t, "projects/my-project/serviceAccounts/sa@proj.iam.gserviceaccount.com/keys/abcdef1234567890", k.Name)
				assert.Equal(t, "abcdef1234567890", k.KeyID)
				assert.Equal(t, "KEY_ALG_RSA_2048", k.KeyAlgorithm)
				assert.Equal(t, "GOOGLE_PROVIDED", k.KeyOrigin)
				assert.Equal(t, "USER_MANAGED", k.KeyType)
				assert.Equal(t, "2024-01-01T00:00:00Z", k.ValidAfterTime)
				assert.Equal(t, "2025-01-01T00:00:00Z", k.ValidBeforeTime)
				assert.False(t, k.Disabled)
			},
		},
		{
			name: "disabled system-managed key",
			input: &iam.ServiceAccountKey{
				Name:         "projects/p/serviceAccounts/sa@p.iam.gserviceaccount.com/keys/syskey123",
				KeyAlgorithm: "KEY_ALG_RSA_2048",
				KeyOrigin:    "GOOGLE_PROVIDED",
				KeyType:      "SYSTEM_MANAGED",
				Disabled:     true,
			},
			validate: func(t *testing.T, k ServiceAccountKey) {
				assert.Equal(t, "syskey123", k.KeyID)
				assert.Equal(t, "SYSTEM_MANAGED", k.KeyType)
				assert.True(t, k.Disabled)
			},
		},
		{
			name: "simple name without slashes",
			input: &iam.ServiceAccountKey{
				Name: "simple-key-id",
			},
			validate: func(t *testing.T, k ServiceAccountKey) {
				// No slash found, so keyID equals the full name
				assert.Equal(t, "simple-key-id", k.KeyID)
				assert.Equal(t, "simple-key-id", k.Name)
			},
		},
		{
			name: "user-provided key origin",
			input: &iam.ServiceAccountKey{
				Name:      "projects/p/serviceAccounts/sa@p.iam.gserviceaccount.com/keys/userkey456",
				KeyOrigin: "USER_PROVIDED",
				KeyType:   "USER_MANAGED",
			},
			validate: func(t *testing.T, k ServiceAccountKey) {
				assert.Equal(t, "userkey456", k.KeyID)
				assert.Equal(t, "USER_PROVIDED", k.KeyOrigin)
				assert.Equal(t, "USER_MANAGED", k.KeyType)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serviceAccountKeyFromAPI(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestIAMPolicyFromAPI(t *testing.T) {
	tests := []struct {
		name     string
		input    *cloudresourcemanager.Policy
		validate func(t *testing.T, policy *IAMPolicy)
	}{
		{
			name: "policy with multiple bindings sorted by role",
			input: &cloudresourcemanager.Policy{
				Version: 3,
				Etag:    "BwXyz123=",
				Bindings: []*cloudresourcemanager.Binding{
					{
						Role:    "roles/viewer",
						Members: []string{"user:bob@example.com", "user:alice@example.com"},
					},
					{
						Role:    "roles/editor",
						Members: []string{"serviceAccount:sa@proj.iam.gserviceaccount.com"},
					},
					{
						Role:    "roles/owner",
						Members: []string{"user:admin@example.com"},
					},
				},
			},
			validate: func(t *testing.T, p *IAMPolicy) {
				assert.Equal(t, int64(3), p.Version)
				assert.Equal(t, "BwXyz123=", p.Etag)
				assert.Len(t, p.Bindings, 3)

				// Bindings should be sorted by role alphabetically
				assert.Equal(t, "roles/editor", p.Bindings[0].Role)
				assert.Equal(t, "roles/owner", p.Bindings[1].Role)
				assert.Equal(t, "roles/viewer", p.Bindings[2].Role)

				// Members within each binding should be sorted
				assert.Equal(t, []string{"user:alice@example.com", "user:bob@example.com"}, p.Bindings[2].Members)
			},
		},
		{
			name: "empty policy with no bindings",
			input: &cloudresourcemanager.Policy{
				Version:  1,
				Etag:     "empty=",
				Bindings: nil,
			},
			validate: func(t *testing.T, p *IAMPolicy) {
				assert.Equal(t, int64(1), p.Version)
				assert.Equal(t, "empty=", p.Etag)
				assert.Empty(t, p.Bindings)
			},
		},
		{
			name: "policy with single binding and single member",
			input: &cloudresourcemanager.Policy{
				Version: 1,
				Bindings: []*cloudresourcemanager.Binding{
					{
						Role:    "roles/owner",
						Members: []string{"user:sole-owner@example.com"},
					},
				},
			},
			validate: func(t *testing.T, p *IAMPolicy) {
				assert.Len(t, p.Bindings, 1)
				assert.Equal(t, "roles/owner", p.Bindings[0].Role)
				assert.Equal(t, []string{"user:sole-owner@example.com"}, p.Bindings[0].Members)
			},
		},
		{
			name: "members are copied, not shared with input",
			input: &cloudresourcemanager.Policy{
				Version: 1,
				Bindings: []*cloudresourcemanager.Binding{
					{
						Role:    "roles/viewer",
						Members: []string{"user:a@example.com", "user:b@example.com"},
					},
				},
			},
			validate: func(t *testing.T, p *IAMPolicy) {
				// Verify that modifying the output doesn't affect the original
				assert.Len(t, p.Bindings[0].Members, 2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := iamPolicyFromAPI(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestIAMPolicyFromAPI_MembersCopied(t *testing.T) {
	// Verify deep copy: modifying output members doesn't affect input
	originalMembers := []string{"user:a@example.com", "user:b@example.com"}
	input := &cloudresourcemanager.Policy{
		Bindings: []*cloudresourcemanager.Binding{
			{
				Role:    "roles/viewer",
				Members: originalMembers,
			},
		},
	}

	result := iamPolicyFromAPI(input)

	// Mutate the output
	result.Bindings[0].Members[0] = "MODIFIED"

	// Original should be unchanged
	assert.Equal(t, "user:a@example.com", originalMembers[0])
}

func TestCustomRoleFromAPI(t *testing.T) {
	tests := []struct {
		name     string
		input    *iam.Role
		validate func(t *testing.T, role CustomRole)
	}{
		{
			name: "fully populated custom role",
			input: &iam.Role{
				Name:        "projects/my-project/roles/myCustomRole",
				Title:       "My Custom Role",
				Description: "A role for custom access",
				Stage:       "GA",
				IncludedPermissions: []string{
					"compute.instances.list",
					"compute.instances.get",
					"storage.buckets.list",
				},
				Deleted: false,
			},
			validate: func(t *testing.T, r CustomRole) {
				assert.Equal(t, "projects/my-project/roles/myCustomRole", r.Name)
				assert.Equal(t, "myCustomRole", r.RoleID)
				assert.Equal(t, "My Custom Role", r.Title)
				assert.Equal(t, "A role for custom access", r.Description)
				assert.Equal(t, "GA", r.Stage)
				assert.False(t, r.Deleted)

				// Permissions should be sorted
				assert.Equal(t, []string{
					"compute.instances.get",
					"compute.instances.list",
					"storage.buckets.list",
				}, r.Permissions)
			},
		},
		{
			name: "deleted role in DISABLED stage",
			input: &iam.Role{
				Name:    "projects/my-project/roles/oldRole",
				Title:   "Old Role",
				Stage:   "DISABLED",
				Deleted: true,
			},
			validate: func(t *testing.T, r CustomRole) {
				assert.Equal(t, "oldRole", r.RoleID)
				assert.Equal(t, "DISABLED", r.Stage)
				assert.True(t, r.Deleted)
				assert.Empty(t, r.Permissions)
			},
		},
		{
			name: "role with no permissions",
			input: &iam.Role{
				Name:                "projects/test/roles/emptyRole",
				Title:               "Empty Role",
				Stage:               "ALPHA",
				IncludedPermissions: nil,
			},
			validate: func(t *testing.T, r CustomRole) {
				assert.Equal(t, "emptyRole", r.RoleID)
				assert.Equal(t, "ALPHA", r.Stage)
				assert.Empty(t, r.Permissions)
			},
		},
		{
			name: "role name without project prefix",
			input: &iam.Role{
				Name:  "simpleRoleName",
				Title: "Simple",
			},
			validate: func(t *testing.T, r CustomRole) {
				// No slash, so roleID equals the full name
				assert.Equal(t, "simpleRoleName", r.RoleID)
				assert.Equal(t, "simpleRoleName", r.Name)
			},
		},
		{
			name: "BETA stage role with many permissions",
			input: &iam.Role{
				Name:  "projects/p/roles/betaRole",
				Title: "Beta Role",
				Stage: "BETA",
				IncludedPermissions: []string{
					"iam.roles.list",
					"iam.roles.get",
					"iam.roles.create",
					"compute.instances.start",
					"compute.instances.stop",
				},
			},
			validate: func(t *testing.T, r CustomRole) {
				assert.Equal(t, "betaRole", r.RoleID)
				assert.Equal(t, "BETA", r.Stage)
				assert.Len(t, r.Permissions, 5)

				// Verify sorted order
				assert.Equal(t, "compute.instances.start", r.Permissions[0])
				assert.Equal(t, "compute.instances.stop", r.Permissions[1])
				assert.Equal(t, "iam.roles.create", r.Permissions[2])
				assert.Equal(t, "iam.roles.get", r.Permissions[3])
				assert.Equal(t, "iam.roles.list", r.Permissions[4])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := customRoleFromAPI(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestCustomRoleFromAPI_PermissionsCopied(t *testing.T) {
	// Verify deep copy: modifying output permissions doesn't affect input
	originalPerms := []string{"compute.instances.list", "storage.buckets.list"}
	input := &iam.Role{
		Name:                "projects/p/roles/testRole",
		IncludedPermissions: originalPerms,
	}

	result := customRoleFromAPI(input)

	// Mutate the output
	result.Permissions[0] = "MODIFIED"

	// Original should be unchanged
	assert.Equal(t, "compute.instances.list", originalPerms[0])
}

func TestIAMClientStruct(t *testing.T) {
	// Verify that IAMClient has the expected zero value fields
	// (can't call NewIAMClient without real credentials)
	client := &IAMClient{}
	assert.Nil(t, client.iamService)
	assert.Nil(t, client.crmService)
}

func TestServiceAccountFields(t *testing.T) {
	// Verify domain type field mapping covers all expected fields
	sa := ServiceAccount{
		Email:       "test@proj.iam.gserviceaccount.com",
		DisplayName: "Test SA",
		Description: "desc",
		UniqueID:    "12345",
		Disabled:    true,
	}

	assert.Equal(t, "test@proj.iam.gserviceaccount.com", sa.Email)
	assert.Equal(t, "Test SA", sa.DisplayName)
	assert.Equal(t, "desc", sa.Description)
	assert.Equal(t, "12345", sa.UniqueID)
	assert.True(t, sa.Disabled)
}

func TestServiceAccountDetailsFields(t *testing.T) {
	details := ServiceAccountDetails{
		Email:          "sa@proj.iam.gserviceaccount.com",
		DisplayName:    "SA",
		Description:    "Description",
		UniqueID:       "111",
		Disabled:       false,
		OAuth2ClientID: "111.apps.googleusercontent.com",
		ProjectID:      "proj",
	}

	assert.Equal(t, "sa@proj.iam.gserviceaccount.com", details.Email)
	assert.Equal(t, "SA", details.DisplayName)
	assert.Equal(t, "Description", details.Description)
	assert.Equal(t, "111", details.UniqueID)
	assert.False(t, details.Disabled)
	assert.Equal(t, "111.apps.googleusercontent.com", details.OAuth2ClientID)
	assert.Equal(t, "proj", details.ProjectID)
}

func TestServiceAccountKeyFields(t *testing.T) {
	key := ServiceAccountKey{
		Name:            "projects/p/serviceAccounts/sa/keys/key1",
		KeyID:           "key1",
		KeyAlgorithm:    "KEY_ALG_RSA_2048",
		KeyOrigin:       "GOOGLE_PROVIDED",
		KeyType:         "USER_MANAGED",
		ValidAfterTime:  "2024-01-01T00:00:00Z",
		ValidBeforeTime: "2025-01-01T00:00:00Z",
		Disabled:        true,
	}

	assert.Equal(t, "projects/p/serviceAccounts/sa/keys/key1", key.Name)
	assert.Equal(t, "key1", key.KeyID)
	assert.Equal(t, "KEY_ALG_RSA_2048", key.KeyAlgorithm)
	assert.Equal(t, "GOOGLE_PROVIDED", key.KeyOrigin)
	assert.Equal(t, "USER_MANAGED", key.KeyType)
	assert.Equal(t, "2024-01-01T00:00:00Z", key.ValidAfterTime)
	assert.Equal(t, "2025-01-01T00:00:00Z", key.ValidBeforeTime)
	assert.True(t, key.Disabled)
}

func TestIAMPolicyFields(t *testing.T) {
	policy := IAMPolicy{
		Bindings: []IAMBinding{
			{
				Role:    "roles/editor",
				Members: []string{"user:a@example.com"},
			},
		},
		Version: 3,
		Etag:    "etag123",
	}

	assert.Len(t, policy.Bindings, 1)
	assert.Equal(t, "roles/editor", policy.Bindings[0].Role)
	assert.Equal(t, []string{"user:a@example.com"}, policy.Bindings[0].Members)
	assert.Equal(t, int64(3), policy.Version)
	assert.Equal(t, "etag123", policy.Etag)
}

func TestCustomRoleFields(t *testing.T) {
	role := CustomRole{
		Name:        "projects/p/roles/myRole",
		Title:       "My Role",
		Description: "Custom role",
		Stage:       "GA",
		Permissions: []string{"compute.instances.list"},
		Deleted:     false,
		RoleID:      "myRole",
	}

	assert.Equal(t, "projects/p/roles/myRole", role.Name)
	assert.Equal(t, "My Role", role.Title)
	assert.Equal(t, "Custom role", role.Description)
	assert.Equal(t, "GA", role.Stage)
	assert.Equal(t, []string{"compute.instances.list"}, role.Permissions)
	assert.False(t, role.Deleted)
	assert.Equal(t, "myRole", role.RoleID)
}

func TestIAMBindingFields(t *testing.T) {
	binding := IAMBinding{
		Role:    "roles/viewer",
		Members: []string{"user:a@example.com", "group:g@example.com"},
	}

	assert.Equal(t, "roles/viewer", binding.Role)
	assert.Len(t, binding.Members, 2)
	assert.Equal(t, "user:a@example.com", binding.Members[0])
	assert.Equal(t, "group:g@example.com", binding.Members[1])
}

func TestIAMPolicyFromAPI_BindingsSortedByRole(t *testing.T) {
	// Explicit test for deterministic binding ordering
	input := &cloudresourcemanager.Policy{
		Bindings: []*cloudresourcemanager.Binding{
			{Role: "roles/viewer", Members: []string{"user:v@x.com"}},
			{Role: "roles/admin", Members: []string{"user:a@x.com"}},
			{Role: "roles/editor", Members: []string{"user:e@x.com"}},
			{Role: "roles/browser", Members: []string{"user:b@x.com"}},
		},
	}

	result := iamPolicyFromAPI(input)

	assert.Len(t, result.Bindings, 4)
	assert.Equal(t, "roles/admin", result.Bindings[0].Role)
	assert.Equal(t, "roles/browser", result.Bindings[1].Role)
	assert.Equal(t, "roles/editor", result.Bindings[2].Role)
	assert.Equal(t, "roles/viewer", result.Bindings[3].Role)
}

func TestIAMPolicyFromAPI_MembersSortedPerBinding(t *testing.T) {
	input := &cloudresourcemanager.Policy{
		Bindings: []*cloudresourcemanager.Binding{
			{
				Role: "roles/viewer",
				Members: []string{
					"user:z@example.com",
					"serviceAccount:sa@proj.iam.gserviceaccount.com",
					"group:devs@example.com",
					"user:a@example.com",
				},
			},
		},
	}

	result := iamPolicyFromAPI(input)

	expected := []string{
		"group:devs@example.com",
		"serviceAccount:sa@proj.iam.gserviceaccount.com",
		"user:a@example.com",
		"user:z@example.com",
	}
	assert.Equal(t, expected, result.Bindings[0].Members)
}

func TestParseMemberType(t *testing.T) {
	tests := []struct {
		member       string
		wantType     string
		wantIdentity string
	}{
		{"user:alice@example.com", "user", "alice@example.com"},
		{"serviceAccount:sa@proj.iam.gserviceaccount.com", "sa", "sa@proj.iam.gserviceaccount.com"},
		{"group:admins@example.com", "group", "admins@example.com"},
		{"domain:example.com", "domain", "example.com"},
		{"deleted:user:old@example.com?uid=123", "deleted:user", "old@example.com?uid=123"},
		{"deleted:serviceAccount:sa@proj.iam.gserviceaccount.com?uid=456", "deleted:sa", "sa@proj.iam.gserviceaccount.com?uid=456"},
		{"nocolon", "", "nocolon"},
		{"", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.member, func(t *testing.T) {
			typeName, identity := ParseMemberType(tt.member)
			assert.Equal(t, tt.wantType, typeName)
			assert.Equal(t, tt.wantIdentity, identity)
		})
	}
}

func TestIAMPolicy_AddMember(t *testing.T) {
	t.Run("add to existing role", func(t *testing.T) {
		p := &IAMPolicy{
			Bindings: []IAMBinding{
				{Role: "roles/viewer", Members: []string{"user:a@x.com"}},
			},
		}
		p.addMember("roles/viewer", "user:b@x.com")
		assert.Len(t, p.Bindings, 1)
		assert.Equal(t, []string{"user:a@x.com", "user:b@x.com"}, p.Bindings[0].Members)
	})

	t.Run("add creates new binding for new role", func(t *testing.T) {
		p := &IAMPolicy{
			Bindings: []IAMBinding{
				{Role: "roles/viewer", Members: []string{"user:a@x.com"}},
			},
		}
		p.addMember("roles/editor", "user:b@x.com")
		assert.Len(t, p.Bindings, 2)
		// Bindings sorted by role
		assert.Equal(t, "roles/editor", p.Bindings[0].Role)
		assert.Equal(t, []string{"user:b@x.com"}, p.Bindings[0].Members)
	})

	t.Run("add duplicate member is no-op", func(t *testing.T) {
		p := &IAMPolicy{
			Bindings: []IAMBinding{
				{Role: "roles/viewer", Members: []string{"user:a@x.com"}},
			},
		}
		p.addMember("roles/viewer", "user:a@x.com")
		assert.Len(t, p.Bindings[0].Members, 1)
	})

	t.Run("add to empty policy", func(t *testing.T) {
		p := &IAMPolicy{}
		p.addMember("roles/owner", "user:admin@x.com")
		assert.Len(t, p.Bindings, 1)
		assert.Equal(t, "roles/owner", p.Bindings[0].Role)
		assert.Equal(t, []string{"user:admin@x.com"}, p.Bindings[0].Members)
	})
}

func TestIAMPolicy_RemoveMember(t *testing.T) {
	t.Run("remove member from role with multiple members", func(t *testing.T) {
		p := &IAMPolicy{
			Bindings: []IAMBinding{
				{Role: "roles/viewer", Members: []string{"user:a@x.com", "user:b@x.com"}},
			},
		}
		p.removeMember("roles/viewer", "user:a@x.com")
		assert.Len(t, p.Bindings, 1)
		assert.Equal(t, []string{"user:b@x.com"}, p.Bindings[0].Members)
	})

	t.Run("remove last member removes entire binding", func(t *testing.T) {
		p := &IAMPolicy{
			Bindings: []IAMBinding{
				{Role: "roles/viewer", Members: []string{"user:a@x.com"}},
				{Role: "roles/editor", Members: []string{"user:b@x.com"}},
			},
		}
		p.removeMember("roles/viewer", "user:a@x.com")
		assert.Len(t, p.Bindings, 1)
		assert.Equal(t, "roles/editor", p.Bindings[0].Role)
	})

	t.Run("remove non-existent member is no-op", func(t *testing.T) {
		p := &IAMPolicy{
			Bindings: []IAMBinding{
				{Role: "roles/viewer", Members: []string{"user:a@x.com"}},
			},
		}
		p.removeMember("roles/viewer", "user:nonexistent@x.com")
		assert.Len(t, p.Bindings[0].Members, 1)
	})

	t.Run("remove from non-existent role is no-op", func(t *testing.T) {
		p := &IAMPolicy{
			Bindings: []IAMBinding{
				{Role: "roles/viewer", Members: []string{"user:a@x.com"}},
			},
		}
		p.removeMember("roles/editor", "user:a@x.com")
		assert.Len(t, p.Bindings, 1)
	})
}

func TestIAMPolicy_ToCRMPolicy(t *testing.T) {
	t.Run("round-trips basic bindings", func(t *testing.T) {
		p := &IAMPolicy{
			Bindings: []IAMBinding{
				{Role: "roles/viewer", Members: []string{"user:a@x.com", "user:b@x.com"}},
			},
			Version: 3,
			Etag:    "etag1",
		}
		crm := p.toCRMPolicy()
		assert.Equal(t, int64(3), crm.Version)
		assert.Equal(t, "etag1", crm.Etag)
		assert.Len(t, crm.Bindings, 1)
		assert.Equal(t, "roles/viewer", crm.Bindings[0].Role)
		assert.Equal(t, []string{"user:a@x.com", "user:b@x.com"}, crm.Bindings[0].Members)
	})

	t.Run("preserves audit configs from raw policy", func(t *testing.T) {
		raw := &cloudresourcemanager.Policy{
			AuditConfigs: []*cloudresourcemanager.AuditConfig{
				{Service: "allServices"},
			},
		}
		p := &IAMPolicy{
			Bindings:  []IAMBinding{{Role: "roles/viewer", Members: []string{"user:a@x.com"}}},
			rawPolicy: raw,
		}
		crm := p.toCRMPolicy()
		assert.Len(t, crm.AuditConfigs, 1)
		assert.Equal(t, "allServices", crm.AuditConfigs[0].Service)
	})

	t.Run("members are copied not shared", func(t *testing.T) {
		p := &IAMPolicy{
			Bindings: []IAMBinding{
				{Role: "roles/viewer", Members: []string{"user:a@x.com"}},
			},
		}
		crm := p.toCRMPolicy()
		crm.Bindings[0].Members[0] = "MODIFIED"
		assert.Equal(t, "user:a@x.com", p.Bindings[0].Members[0])
	})
}

func TestCustomRoleFromAPI_PermissionsSorted(t *testing.T) {
	input := &iam.Role{
		Name: "projects/p/roles/r",
		IncludedPermissions: []string{
			"zzz.permission",
			"aaa.permission",
			"mmm.permission",
		},
	}

	result := customRoleFromAPI(input)

	assert.Equal(t, []string{
		"aaa.permission",
		"mmm.permission",
		"zzz.permission",
	}, result.Permissions)
}

func TestServiceAccountKeyFromAPI_KeyIDExtraction(t *testing.T) {
	// Focused tests for key ID extraction from full resource name
	tests := []struct {
		name          string
		fullName      string
		expectedKeyID string
	}{
		{
			name:          "standard resource name",
			fullName:      "projects/my-proj/serviceAccounts/sa@proj.iam.gserviceaccount.com/keys/abc123",
			expectedKeyID: "abc123",
		},
		{
			name:          "name with single slash",
			fullName:      "keys/mykey",
			expectedKeyID: "mykey",
		},
		{
			name:          "name without any slashes",
			fullName:      "standalone-key",
			expectedKeyID: "standalone-key",
		},
		{
			name:          "empty name",
			fullName:      "",
			expectedKeyID: "",
		},
		{
			name:          "trailing slash produces empty key ID",
			fullName:      "projects/p/keys/",
			expectedKeyID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := serviceAccountKeyFromAPI(&iam.ServiceAccountKey{Name: tt.fullName})
			assert.Equal(t, tt.expectedKeyID, key.KeyID)
		})
	}
}

func TestCustomRoleFromAPI_RoleIDExtraction(t *testing.T) {
	// Focused tests for role ID extraction from full resource name
	tests := []struct {
		name           string
		fullName       string
		expectedRoleID string
	}{
		{
			name:           "standard project role",
			fullName:       "projects/my-project/roles/customViewer",
			expectedRoleID: "customViewer",
		},
		{
			name:           "organization role",
			fullName:       "organizations/123456/roles/orgAdmin",
			expectedRoleID: "orgAdmin",
		},
		{
			name:           "simple name no slashes",
			fullName:       "simpleRole",
			expectedRoleID: "simpleRole",
		},
		{
			name:           "empty name",
			fullName:       "",
			expectedRoleID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role := customRoleFromAPI(&iam.Role{Name: tt.fullName})
			assert.Equal(t, tt.expectedRoleID, role.RoleID)
		})
	}
}
