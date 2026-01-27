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
	ctx                    context.Context
	crmService             *cloudresourcemanager.Service // Cloud Resource Manager for projects
	monitoringClient       *MonitoringClient
	monitoringClientProjID string
	loggingClient          *LoggingClient
	loggingClientProjID    string
	authenticatedIdentity  string              // Email of authenticated user or service account
	identityType           config.IdentityType // Type of authenticated identity (user or service account)
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
	identity, identityType, _ := config.GetAuthenticatedIdentity() //nolint:errcheck // Display only

	return &Client{
		ctx:                   ctx,
		crmService:            crmService,
		authenticatedIdentity: identity,
		identityType:          identityType,
	}, nil
}

// Close cleans up any resources
func (c *Client) Close() error {
	if c.monitoringClient != nil {
		if err := c.monitoringClient.Close(); err != nil {
			return fmt.Errorf("failed to close monitoring client: %w", err)
		}
	}
	if c.loggingClient != nil {
		if err := c.loggingClient.Close(); err != nil {
			return fmt.Errorf("failed to close logging client: %w", err)
		}
	}
	return nil
}

// Context returns the client's context
func (c *Client) Context() context.Context {
	return c.ctx
}

// GetMonitoringClient returns or initializes the monitoring client
// Reinitializes if projectID changes to prevent querying wrong project
func (c *Client) GetMonitoringClient(projectID string) (*MonitoringClient, error) {
	if c.monitoringClient == nil || c.monitoringClientProjID != projectID {
		// Close old client before replacing to prevent resource leak
		if c.monitoringClient != nil {
			_ = c.monitoringClient.Close() //nolint:errcheck // Best-effort cleanup
		}
		client, err := NewMonitoringClient(c.ctx, projectID)
		if err != nil {
			return nil, err
		}
		c.monitoringClient = client
		c.monitoringClientProjID = projectID
	}
	return c.monitoringClient, nil
}

// GetLoggingClient returns or initializes the logging client
// Reinitializes if projectID changes to prevent querying wrong project
func (c *Client) GetLoggingClient(projectID string) (*LoggingClient, error) {
	if c.loggingClient == nil || c.loggingClientProjID != projectID {
		// Close old client before replacing to prevent resource leak
		if c.loggingClient != nil {
			_ = c.loggingClient.Close() //nolint:errcheck // Best-effort cleanup
		}
		client, err := NewLoggingClient(c.ctx, projectID)
		if err != nil {
			return nil, err
		}
		c.loggingClient = client
		c.loggingClientProjID = projectID
	}
	return c.loggingClient, nil
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
