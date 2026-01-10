package gcp

import (
	"context"
	"fmt"

	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/option"
)

// Client wraps GCP API clients and provides unified access
type Client struct {
	ctx        context.Context
	crmService *cloudresourcemanager.Service // Cloud Resource Manager for projects
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

	return &Client{
		ctx:        ctx,
		crmService: crmService,
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
