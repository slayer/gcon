package gcp

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"

	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
)

// ErrEtagConflict is returned when an IAM policy update fails due to etag mismatch after retries.
var ErrEtagConflict = errors.New("IAM policy etag conflict after retry")

// ServiceAccount is the list-view summary
type ServiceAccount struct {
	Email       string
	DisplayName string
	Description string
	UniqueID    string
	Disabled    bool
}

// ServiceAccountDetails is the full detail for the details view
type ServiceAccountDetails struct {
	Email          string
	DisplayName    string
	Description    string
	UniqueID       string
	Disabled       bool
	OAuth2ClientID string
	ProjectID      string
}

// ServiceAccountKey represents a key on a service account
type ServiceAccountKey struct {
	Name            string // Full resource name
	KeyID           string // Short ID extracted from Name
	KeyAlgorithm    string // KEY_ALG_RSA_2048, etc.
	KeyOrigin       string // GOOGLE_PROVIDED, USER_PROVIDED
	KeyType         string // USER_MANAGED, SYSTEM_MANAGED
	ValidAfterTime  string
	ValidBeforeTime string
	Disabled        bool
}

// IAMBinding represents a single role→members binding in an IAM policy.
// ConditionTitle is non-empty when the binding has an IAM condition attached.
// GCP requires unique condition titles per role, so (Role, ConditionTitle)
// forms a stable composite key for identifying bindings.
type IAMBinding struct {
	Role           string
	Members        []string
	ConditionTitle string
}

// BindingKey returns a composite key that uniquely identifies this binding.
// Unconditioned bindings use just the role; conditioned ones use "role|title".
// The "|" separator is safe because GCP role names only use "/" as separators.
func (b *IAMBinding) BindingKey() string {
	if b.ConditionTitle == "" {
		return b.Role
	}
	return b.Role + "|" + b.ConditionTitle
}

// ParseBindingKey splits a composite binding key back into role and conditionTitle.
func ParseBindingKey(key string) (role, conditionTitle string) {
	role, conditionTitle, _ = strings.Cut(key, "|")
	return role, conditionTitle
}

// IAMPolicy represents the full IAM policy for a project
type IAMPolicy struct {
	Bindings []IAMBinding
	Version  int64
	Etag     string

	// rawPolicy preserves the original CRM Policy for round-tripping
	// (conditions, audit configs, etc. that we don't model)
	rawPolicy *cloudresourcemanager.Policy
}

// CustomRole represents a project-level custom IAM role
type CustomRole struct {
	Name        string // Full resource name
	Title       string
	Description string
	Stage       string // GA, BETA, ALPHA, DISABLED, EAP
	Permissions []string
	Deleted     bool
	RoleID      string // Short ID (e.g. "myCustomRole")
}

// IAMClient handles IAM operations
type IAMClient struct {
	iamService *iam.Service
	crmService *cloudresourcemanager.Service
}

// NewIAMClient creates a new IAM client
func NewIAMClient(ctx context.Context) (*IAMClient, error) {
	iamService, err := iam.NewService(ctx, option.WithScopes(
		iam.CloudPlatformScope,
	))
	if err != nil {
		return nil, fmt.Errorf("failed to create IAM service: %w", err)
	}

	crmService, err := cloudresourcemanager.NewService(ctx, option.WithScopes(
		cloudresourcemanager.CloudPlatformScope,
	))
	if err != nil {
		return nil, fmt.Errorf("failed to create CRM service: %w", err)
	}

	return &IAMClient{
		iamService: iamService,
		crmService: crmService,
	}, nil
}

// ListServiceAccounts returns all service accounts in a project
func (c *IAMClient) ListServiceAccounts(ctx context.Context, projectID string) ([]ServiceAccount, error) {
	var accounts []ServiceAccount

	resource := "projects/" + projectID
	req := c.iamService.Projects.ServiceAccounts.List(resource)
	for {
		resp, err := req.Context(ctx).Do()
		if err != nil {
			return nil, WrapListError(err, "service accounts", projectID)
		}

		for _, sa := range resp.Accounts {
			accounts = append(accounts, serviceAccountFromAPI(sa))
		}

		if resp.NextPageToken == "" {
			break
		}
		req = req.PageToken(resp.NextPageToken)
	}

	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].Email < accounts[j].Email
	})

	return accounts, nil
}

// GetServiceAccount returns detailed info for a single service account
func (c *IAMClient) GetServiceAccount(ctx context.Context, email string) (*ServiceAccountDetails, error) {
	resource := "projects/-/serviceAccounts/" + email
	sa, err := c.iamService.Projects.ServiceAccounts.Get(resource).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "service account", email)
	}
	return serviceAccountDetailsFromAPI(sa), nil
}

// CreateServiceAccount creates a new service account in a project
func (c *IAMClient) CreateServiceAccount(ctx context.Context, projectID, accountID, displayName, description string) (*ServiceAccountDetails, error) {
	resource := "projects/" + projectID
	sa, err := c.iamService.Projects.ServiceAccounts.Create(resource, &iam.CreateServiceAccountRequest{
		AccountId: accountID,
		ServiceAccount: &iam.ServiceAccount{
			DisplayName: displayName,
			Description: description,
		},
	}).Context(ctx).Do()
	if err != nil {
		return nil, WrapActionError(err, "create service account", accountID)
	}
	return serviceAccountDetailsFromAPI(sa), nil
}

// DeleteServiceAccount deletes a service account
func (c *IAMClient) DeleteServiceAccount(ctx context.Context, email string) error {
	resource := "projects/-/serviceAccounts/" + email
	_, err := c.iamService.Projects.ServiceAccounts.Delete(resource).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "delete service account", email)
	}
	return nil
}

// EnableServiceAccount enables a disabled service account
func (c *IAMClient) EnableServiceAccount(ctx context.Context, email string) error {
	resource := "projects/-/serviceAccounts/" + email
	_, err := c.iamService.Projects.ServiceAccounts.Enable(resource, &iam.EnableServiceAccountRequest{}).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "enable service account", email)
	}
	return nil
}

// DisableServiceAccount disables a service account
func (c *IAMClient) DisableServiceAccount(ctx context.Context, email string) error {
	resource := "projects/-/serviceAccounts/" + email
	_, err := c.iamService.Projects.ServiceAccounts.Disable(resource, &iam.DisableServiceAccountRequest{}).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "disable service account", email)
	}
	return nil
}

// ListServiceAccountKeys returns all keys for a service account
func (c *IAMClient) ListServiceAccountKeys(ctx context.Context, email string) ([]ServiceAccountKey, error) {
	resource := "projects/-/serviceAccounts/" + email
	resp, err := c.iamService.Projects.ServiceAccounts.Keys.List(resource).Context(ctx).Do()
	if err != nil {
		return nil, WrapListError(err, "service account keys", email)
	}

	keys := make([]ServiceAccountKey, 0, len(resp.Keys))
	for _, k := range resp.Keys {
		keys = append(keys, serviceAccountKeyFromAPI(k))
	}

	return keys, nil
}

// CreateServiceAccountKey creates a new key for a service account.
// Returns the private key JSON bytes — this is the only time the private key is available.
func (c *IAMClient) CreateServiceAccountKey(ctx context.Context, email string) ([]byte, *ServiceAccountKey, error) {
	resource := "projects/-/serviceAccounts/" + email
	key, err := c.iamService.Projects.ServiceAccounts.Keys.Create(resource, &iam.CreateServiceAccountKeyRequest{}).Context(ctx).Do()
	if err != nil {
		return nil, nil, WrapActionError(err, "create service account key", email)
	}

	saKey := serviceAccountKeyFromAPI(key)
	// IAM API returns PrivateKeyData as base64-encoded JSON key file
	keyJSON, err := base64.StdEncoding.DecodeString(key.PrivateKeyData)
	if err != nil {
		return nil, nil, fmt.Errorf("decode private key data: %w", err)
	}
	return keyJSON, &saKey, nil
}

// DeleteServiceAccountKey deletes a key from a service account
func (c *IAMClient) DeleteServiceAccountKey(ctx context.Context, keyName string) error {
	_, err := c.iamService.Projects.ServiceAccounts.Keys.Delete(keyName).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "delete service account key", keyName)
	}
	return nil
}

// GetProjectIAMPolicy returns the IAM policy for a project
func (c *IAMClient) GetProjectIAMPolicy(ctx context.Context, projectID string) (*IAMPolicy, error) {
	resp, err := c.crmService.Projects.GetIamPolicy(projectID, &cloudresourcemanager.GetIamPolicyRequest{
		Options: &cloudresourcemanager.GetPolicyOptions{
			RequestedPolicyVersion: 3,
		},
	}).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "IAM policy", projectID)
	}

	return iamPolicyFromAPI(resp), nil
}

// SetProjectIAMPolicy sets the IAM policy for a project.
// Uses the stored raw policy to preserve conditions and audit configs.
func (c *IAMClient) SetProjectIAMPolicy(ctx context.Context, projectID string, policy *IAMPolicy) (*IAMPolicy, error) {
	crmPolicy := policy.toCRMPolicy()
	resp, err := c.crmService.Projects.SetIamPolicy(projectID, &cloudresourcemanager.SetIamPolicyRequest{
		Policy: crmPolicy,
	}).Context(ctx).Do()
	if err != nil {
		return nil, WrapActionError(err, "set IAM policy", projectID)
	}
	return iamPolicyFromAPI(resp), nil
}

// AddMemberToRole adds a member to a role binding, with a single retry on etag conflict.
// conditionTitle targets the specific binding when multiple bindings share the same role.
func (c *IAMClient) AddMemberToRole(ctx context.Context, projectID, role, conditionTitle, member string) (*IAMPolicy, error) {
	for attempt := range 2 {
		policy, err := c.GetProjectIAMPolicy(ctx, projectID)
		if err != nil {
			return nil, err
		}

		policy.addMember(role, conditionTitle, member)

		result, err := c.SetProjectIAMPolicy(ctx, projectID, policy)
		if err != nil {
			// Retry once on etag conflict (409)
			if attempt == 0 && isConflictError(err) {
				continue
			}
			return nil, err
		}
		return result, nil
	}
	return nil, fmt.Errorf("add member to role: %w", ErrEtagConflict)
}

// RemoveMemberFromRole removes a member from a role binding, with a single retry on etag conflict.
// conditionTitle targets the specific binding when multiple bindings share the same role.
func (c *IAMClient) RemoveMemberFromRole(ctx context.Context, projectID, role, conditionTitle, member string) (*IAMPolicy, error) {
	for attempt := range 2 {
		policy, err := c.GetProjectIAMPolicy(ctx, projectID)
		if err != nil {
			return nil, err
		}

		policy.removeMember(role, conditionTitle, member)

		result, err := c.SetProjectIAMPolicy(ctx, projectID, policy)
		if err != nil {
			if attempt == 0 && isConflictError(err) {
				continue
			}
			return nil, err
		}
		return result, nil
	}
	return nil, fmt.Errorf("remove member from role: %w", ErrEtagConflict)
}

// ParseMemberType splits an IAM member string into type label and identity.
// Example: "user:alice@example.com" → ("user", "alice@example.com")
func ParseMemberType(member string) (typeName, identity string) {
	// Handle "deleted:" prefix — e.g. "deleted:user:alice@example.com?uid=123"
	isDeleted := strings.HasPrefix(member, "deleted:")
	rest := strings.TrimPrefix(member, "deleted:")

	var found bool
	typeName, identity, found = strings.Cut(rest, ":")
	if !found {
		return "", member
	}

	// Shorten "serviceAccount" → "sa" for display
	if typeName == "serviceAccount" {
		typeName = "sa"
	}

	if isDeleted {
		typeName = "deleted:" + typeName
	}

	return typeName, identity
}

// addMember adds a member to the specified role binding, creating the binding if needed.
// Matches on (role, conditionTitle) to handle duplicate-role bindings with different conditions.
func (p *IAMPolicy) addMember(role, conditionTitle, member string) {
	for i, b := range p.Bindings {
		if b.Role == role && b.ConditionTitle == conditionTitle {
			// Check if member already exists
			for _, m := range b.Members {
				if m == member {
					return
				}
			}
			p.Bindings[i].Members = append(p.Bindings[i].Members, member)
			sort.Strings(p.Bindings[i].Members)
			return
		}
	}
	// Only create new unconditioned bindings — conditioned bindings must already exist
	// because their condition expression lives in rawPolicy and can't be reconstructed.
	if conditionTitle != "" {
		return
	}
	p.Bindings = append(p.Bindings, IAMBinding{
		Role:    role,
		Members: []string{member},
	})
	sort.Slice(p.Bindings, func(i, j int) bool {
		if p.Bindings[i].Role != p.Bindings[j].Role {
			return p.Bindings[i].Role < p.Bindings[j].Role
		}
		return p.Bindings[i].ConditionTitle < p.Bindings[j].ConditionTitle
	})
}

// removeMember removes a member from the specified role binding.
// Matches on (role, conditionTitle) to target the correct binding.
// Removes the entire binding if no members remain.
func (p *IAMPolicy) removeMember(role, conditionTitle, member string) {
	for i, b := range p.Bindings {
		if b.Role == role && b.ConditionTitle == conditionTitle {
			filtered := make([]string, 0, len(b.Members))
			for _, m := range b.Members {
				if m != member {
					filtered = append(filtered, m)
				}
			}
			if len(filtered) == 0 {
				p.Bindings = append(p.Bindings[:i], p.Bindings[i+1:]...)
			} else {
				p.Bindings[i].Members = filtered
			}
			return
		}
	}
}

// toCRMPolicy converts back to a CRM Policy, reusing the stored raw policy
// to preserve conditions and audit configs.
func (p *IAMPolicy) toCRMPolicy() *cloudresourcemanager.Policy {
	bindings := make([]*cloudresourcemanager.Binding, 0, len(p.Bindings))
	for _, b := range p.Bindings {
		members := make([]string, len(b.Members))
		copy(members, b.Members)
		binding := &cloudresourcemanager.Binding{
			Role:    b.Role,
			Members: members,
		}
		// Restore condition from the raw policy by matching (role, conditionTitle).
		// Shallow-copy the Expr to avoid sharing pointers with the cached rawPolicy.
		if p.rawPolicy != nil {
			for _, raw := range p.rawPolicy.Bindings {
				rawTitle := ""
				if raw.Condition != nil {
					rawTitle = raw.Condition.Title
				}
				if raw.Role == b.Role && rawTitle == b.ConditionTitle {
					if raw.Condition != nil {
						cond := *raw.Condition
						binding.Condition = &cond
					}
					break
				}
			}
		}
		bindings = append(bindings, binding)
	}

	policy := &cloudresourcemanager.Policy{
		Bindings: bindings,
		Version:  p.Version,
		Etag:     p.Etag,
	}

	// Preserve audit configs from original
	if p.rawPolicy != nil {
		policy.AuditConfigs = p.rawPolicy.AuditConfigs
	}

	return policy
}

// isConflictError checks if an error is an HTTP 409 conflict (etag mismatch).
func isConflictError(err error) bool {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return apiErr.Code == 409
	}
	return false
}

// ListCustomRoles returns all custom roles in a project
func (c *IAMClient) ListCustomRoles(ctx context.Context, projectID string) ([]CustomRole, error) {
	var roles []CustomRole

	parent := "projects/" + projectID
	req := c.iamService.Projects.Roles.List(parent)
	for {
		resp, err := req.Context(ctx).Do()
		if err != nil {
			return nil, WrapListError(err, "custom roles", projectID)
		}

		for _, r := range resp.Roles {
			roles = append(roles, customRoleFromAPI(r))
		}

		if resp.NextPageToken == "" {
			break
		}
		req = req.PageToken(resp.NextPageToken)
	}

	sort.Slice(roles, func(i, j int) bool {
		return roles[i].Title < roles[j].Title
	})

	return roles, nil
}

// GetCustomRole returns a single custom role by ID
func (c *IAMClient) GetCustomRole(ctx context.Context, projectID, roleID string) (*CustomRole, error) {
	name := fmt.Sprintf("projects/%s/roles/%s", projectID, roleID)
	r, err := c.iamService.Projects.Roles.Get(name).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "custom role", roleID)
	}
	role := customRoleFromAPI(r)
	return &role, nil
}

// --- Conversion helpers ---

func serviceAccountFromAPI(sa *iam.ServiceAccount) ServiceAccount {
	return ServiceAccount{
		Email:       sa.Email,
		DisplayName: sa.DisplayName,
		Description: sa.Description,
		UniqueID:    sa.UniqueId,
		Disabled:    sa.Disabled,
	}
}

func serviceAccountDetailsFromAPI(sa *iam.ServiceAccount) *ServiceAccountDetails {
	return &ServiceAccountDetails{
		Email:          sa.Email,
		DisplayName:    sa.DisplayName,
		Description:    sa.Description,
		UniqueID:       sa.UniqueId,
		Disabled:       sa.Disabled,
		OAuth2ClientID: sa.Oauth2ClientId,
		ProjectID:      sa.ProjectId,
	}
}

func serviceAccountKeyFromAPI(k *iam.ServiceAccountKey) ServiceAccountKey {
	// Extract short key ID from full resource name
	// Format: projects/{project}/serviceAccounts/{email}/keys/{keyId}
	keyID := k.Name
	if idx := strings.LastIndex(k.Name, "/"); idx >= 0 {
		keyID = k.Name[idx+1:]
	}

	return ServiceAccountKey{
		Name:            k.Name,
		KeyID:           keyID,
		KeyAlgorithm:    k.KeyAlgorithm,
		KeyOrigin:       k.KeyOrigin,
		KeyType:         k.KeyType,
		ValidAfterTime:  k.ValidAfterTime,
		ValidBeforeTime: k.ValidBeforeTime,
		Disabled:        k.Disabled,
	}
}

func iamPolicyFromAPI(p *cloudresourcemanager.Policy) *IAMPolicy {
	bindings := make([]IAMBinding, 0, len(p.Bindings))
	for _, b := range p.Bindings {
		members := make([]string, len(b.Members))
		copy(members, b.Members)
		sort.Strings(members)

		condTitle := ""
		if b.Condition != nil {
			condTitle = b.Condition.Title
		}

		bindings = append(bindings, IAMBinding{
			Role:           b.Role,
			Members:        members,
			ConditionTitle: condTitle,
		})
	}

	// Sort by (role, conditionTitle) for deterministic display
	sort.Slice(bindings, func(i, j int) bool {
		if bindings[i].Role != bindings[j].Role {
			return bindings[i].Role < bindings[j].Role
		}
		return bindings[i].ConditionTitle < bindings[j].ConditionTitle
	})

	return &IAMPolicy{
		Bindings:  bindings,
		Version:   p.Version,
		Etag:      p.Etag,
		rawPolicy: p,
	}
}

func customRoleFromAPI(r *iam.Role) CustomRole {
	// Extract short role ID from name (projects/{project}/roles/{roleId})
	roleID := r.Name
	if idx := strings.LastIndex(r.Name, "/"); idx >= 0 {
		roleID = r.Name[idx+1:]
	}

	permissions := make([]string, len(r.IncludedPermissions))
	copy(permissions, r.IncludedPermissions)
	sort.Strings(permissions)

	return CustomRole{
		Name:        r.Name,
		Title:       r.Title,
		Description: r.Description,
		Stage:       r.Stage,
		Permissions: permissions,
		Deleted:     r.Deleted,
		RoleID:      roleID,
	}
}
