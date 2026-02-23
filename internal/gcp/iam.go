package gcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
)

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

// IAMBinding represents a single role→members binding in an IAM policy
type IAMBinding struct {
	Role    string
	Members []string
}

// IAMPolicy represents the full IAM policy for a project
type IAMPolicy struct {
	Bindings []IAMBinding
	Version  int64
	Etag     string
}

// CustomRole represents a project-level custom IAM role
type CustomRole struct {
	Name        string   // Full resource name
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
		bindings = append(bindings, IAMBinding{
			Role:    b.Role,
			Members: members,
		})
	}

	// Sort bindings by role for consistent display
	sort.Slice(bindings, func(i, j int) bool {
		return bindings[i].Role < bindings[j].Role
	})

	return &IAMPolicy{
		Bindings: bindings,
		Version:  p.Version,
		Etag:     p.Etag,
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
