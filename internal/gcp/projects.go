package gcp

import (
	"context"

	"google.golang.org/api/cloudresourcemanager/v1"
)

// Project represents a simplified GCP project
type Project struct {
	ID     string
	Name   string
	Number int64
	State  string
}

// ListProjects returns all accessible GCP projects
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	var projects []Project

	req := c.crmService.Projects.List()
	err := req.Pages(ctx, func(page *cloudresourcemanager.ListProjectsResponse) error {
		for _, p := range page.Projects {
			projects = append(projects, Project{
				ID:     p.ProjectId,
				Name:   p.Name,
				Number: p.ProjectNumber,
				State:  p.LifecycleState,
			})
		}
		return nil
	})
	if err != nil {
		return nil, WrapListError(err, "projects", "")
	}

	return projects, nil
}

// GetProject returns details for a specific project
func (c *Client) GetProject(ctx context.Context, projectID string) (*Project, error) {
	p, err := c.crmService.Projects.Get(projectID).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "project", projectID)
	}

	return &Project{
		ID:     p.ProjectId,
		Name:   p.Name,
		Number: p.ProjectNumber,
		State:  p.LifecycleState,
	}, nil
}
