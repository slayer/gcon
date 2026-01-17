package gcp

import (
	"context"
	"fmt"

	"github.com/slayer/gcon/internal/config"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/option"
)

// Client wraps GCP API clients and provides unified access
type Client struct {
	ctx                   context.Context
	crmService            *cloudresourcemanager.Service // Cloud Resource Manager for projects
	authenticatedIdentity string                        // Email of authenticated user or service account
	identityType          config.IdentityType           // Type of authenticated identity (user or service account)
}

// NewClient creates a new GCP client using Application Default Credentials
func NewClient() (*Client, error) {
	ctx := context.Background()

	// Cloud Resource Manager for listing projects
	crmService, err := cloudresourcemanager.NewService(ctx, option.WithScopes(
		cloudresourcemanager.CloudPlatformReadOnlyScope,
	))
	if err != nil {
		return nil, fmt.Errorf("failed to create resource manager client: %w", err)
	}

	// Retrieve authenticated identity and type (non-critical, ignore errors)
	identity, identityType, _ := config.GetAuthenticatedIdentity()

	return &Client{
		ctx:                   ctx,
		crmService:            crmService,
		authenticatedIdentity: identity,
		identityType:          identityType,
	}, nil
}

// Close cleans up any resources
func (c *Client) Close() error {
	// Currently no persistent connections to close
	return nil
}

// Context returns the client's context
func (c *Client) Context() context.Context {
	return c.ctx
}

// GetAuthenticatedIdentity returns the email of the authenticated user or service account.
// Returns empty string if unable to determine identity.
func (c *Client) GetAuthenticatedIdentity() string {
	return c.authenticatedIdentity
}

// GetIdentityType returns the type of the authenticated identity (user or service account).
func (c *Client) GetIdentityType() config.IdentityType {
	return c.identityType
}
