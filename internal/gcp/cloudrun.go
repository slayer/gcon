package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"google.golang.org/api/option"
	run "google.golang.org/api/run/v2"
	"gopkg.in/yaml.v3"
)

// CloudRunService is the list-view summary
type CloudRunService struct {
	Name           string
	FullName       string // projects/p/locations/r/services/s
	Region         string
	URL            string
	LatestRevision string
	Status         string // Ready, Deploying, Failed, etc.
	UpdatedAt      string
}

// CloudRunServiceDetails has full info for the detail view
type CloudRunServiceDetails struct {
	Name           string
	FullName       string
	Region         string
	URL            string
	LatestRevision string
	Status         string
	StatusMessage  string

	Description    string
	ServiceAccount string
	Ingress        string

	// Container configuration from the template
	ContainerImage string
	CPU            string
	Memory         string
	ContainerPort  int64

	// Scaling
	MinInstances   int64
	MaxInstances   int64
	Concurrency    int64
	TimeoutSeconds int64

	EnvVars map[string]string
	Labels  map[string]string

	// Traffic routing
	Traffic []CloudRunTrafficTarget

	CreatedAt    string
	UpdatedAt    string
	Creator      string
	LastModifier string

	// YAML representation for the YAML tab
	RawYAML string
}

// CloudRunTrafficTarget represents a traffic split entry
type CloudRunTrafficTarget struct {
	RevisionName string
	Percent      int64
	Tag          string
	Type         string // "LATEST" or "REVISION"
}

// CloudRunRevision represents a revision of a Cloud Run service
type CloudRunRevision struct {
	Name           string
	ShortName      string
	Status         string
	ContainerImage string
	TrafficPercent int64
	CreatedAt      string
}

// CloudRunClient handles Cloud Run operations
type CloudRunClient struct {
	service *run.Service
}

// NewCloudRunClient creates a new Cloud Run client
func NewCloudRunClient(ctx context.Context) (*CloudRunClient, error) {
	service, err := run.NewService(ctx, option.WithScopes(
		"https://www.googleapis.com/auth/cloud-platform",
	))
	if err != nil {
		return nil, fmt.Errorf("failed to create cloud run client: %w", err)
	}
	return &CloudRunClient{service: service}, nil
}

// ListServices returns all Cloud Run services in a project across all regions
func (c *CloudRunClient) ListServices(ctx context.Context, projectID string) ([]CloudRunService, error) {
	var services []CloudRunService

	// Use "-" wildcard location to list across all regions
	parent := fmt.Sprintf("projects/%s/locations/-", projectID)
	req := c.service.Projects.Locations.Services.List(parent)

	for {
		resp, err := req.Context(ctx).Do()
		if err != nil {
			return nil, WrapListError(err, "cloud run services", projectID)
		}

		for _, svc := range resp.Services {
			services = append(services, cloudRunServiceFromAPI(svc))
		}

		if resp.NextPageToken == "" {
			break
		}
		req = req.PageToken(resp.NextPageToken)
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	return services, nil
}

// GetService returns detailed info for a single Cloud Run service
func (c *CloudRunClient) GetService(ctx context.Context, fullName string) (*CloudRunServiceDetails, error) {
	svc, err := c.service.Projects.Locations.Services.Get(fullName).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "cloud run service", extractShortName(fullName))
	}
	return cloudRunServiceDetailsFromAPI(svc), nil
}

// ListRevisions returns all revisions for a Cloud Run service.
// Traffic percentages are NOT populated here — the caller should use
// ApplyTrafficToRevisions with data from GetService to avoid a redundant API call.
func (c *CloudRunClient) ListRevisions(ctx context.Context, serviceFullName string) ([]CloudRunRevision, error) {
	var revisions []CloudRunRevision

	req := c.service.Projects.Locations.Services.Revisions.List(serviceFullName)
	for {
		resp, err := req.Context(ctx).Do()
		if err != nil {
			return nil, WrapListError(err, "cloud run revisions", extractShortName(serviceFullName))
		}

		for _, rev := range resp.Revisions {
			revisions = append(revisions, cloudRunRevisionFromAPI(rev))
		}

		if resp.NextPageToken == "" {
			break
		}
		req = req.PageToken(resp.NextPageToken)
	}

	return revisions, nil
}

// ApplyTrafficToRevisions enriches revisions with traffic percentages from the service details.
// Both the traffic map keys (CloudRunTrafficTarget.RevisionName) and the lookup here use short revision names.
func ApplyTrafficToRevisions(revisions []CloudRunRevision, traffic []CloudRunTrafficTarget) {
	trafficMap := make(map[string]int64, len(traffic))
	for _, t := range traffic {
		if t.RevisionName != "" {
			trafficMap[t.RevisionName] = t.Percent
		}
	}
	for i := range revisions {
		if pct, ok := trafficMap[revisions[i].ShortName]; ok {
			revisions[i].TrafficPercent = pct
		}
	}
}

// DeleteService deletes a Cloud Run service
func (c *CloudRunClient) DeleteService(ctx context.Context, fullName string) error {
	_, err := c.service.Projects.Locations.Services.Delete(fullName).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "delete cloud run service", extractShortName(fullName))
	}
	return nil
}

// UpdateTraffic updates the traffic split for a Cloud Run service
func (c *CloudRunClient) UpdateTraffic(ctx context.Context, fullName string, targets []CloudRunTrafficTarget) error {
	apiTargets := make([]*run.GoogleCloudRunV2TrafficTarget, 0, len(targets))
	for _, t := range targets {
		apiTarget := &run.GoogleCloudRunV2TrafficTarget{
			Percent:         t.Percent,
			ForceSendFields: []string{"Percent"},
		}
		if t.Type == "LATEST" {
			apiTarget.Type = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
		} else {
			apiTarget.Type = "TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION"
			apiTarget.Revision = t.RevisionName
		}
		if t.Tag != "" {
			apiTarget.Tag = t.Tag
		}
		apiTargets = append(apiTargets, apiTarget)
	}

	patchSvc := &run.GoogleCloudRunV2Service{
		Traffic:         apiTargets,
		ForceSendFields: []string{"Traffic"},
	}

	_, err := c.service.Projects.Locations.Services.Patch(fullName, patchSvc).
		UpdateMask("traffic").
		Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "update traffic", extractShortName(fullName))
	}
	return nil
}

// cloudRunServiceFromAPI converts an API service to our list-view summary
func cloudRunServiceFromAPI(svc *run.GoogleCloudRunV2Service) CloudRunService {
	return CloudRunService{
		Name:           extractShortName(svc.Name),
		FullName:       svc.Name,
		Region:         extractRegion(svc.Name),
		URL:            svc.Uri,
		LatestRevision: extractShortName(svc.LatestReadyRevision),
		Status:         deriveStatus(svc),
		UpdatedAt:      formatCloudRunTime(svc.UpdateTime),
	}
}

// cloudRunServiceDetailsFromAPI converts an API service to full details
func cloudRunServiceDetailsFromAPI(svc *run.GoogleCloudRunV2Service) *CloudRunServiceDetails {
	details := &CloudRunServiceDetails{
		Name:           extractShortName(svc.Name),
		FullName:       svc.Name,
		Region:         extractRegion(svc.Name),
		URL:            svc.Uri,
		LatestRevision: extractShortName(svc.LatestReadyRevision),
		Status:         deriveStatus(svc),
		Description:    svc.Description,
		Ingress:        formatIngress(svc.Ingress),
		Labels:         svc.Labels,
		CreatedAt:      svc.CreateTime,
		UpdatedAt:      svc.UpdateTime,
		Creator:        svc.Creator,
		LastModifier:   svc.LastModifier,
	}

	// Status message from terminal condition
	if svc.TerminalCondition != nil {
		details.StatusMessage = svc.TerminalCondition.Message
	}

	// Extract container configuration from the template
	if svc.Template != nil {
		details.ServiceAccount = svc.Template.ServiceAccount

		if len(svc.Template.Containers) > 0 {
			container := svc.Template.Containers[0]
			details.ContainerImage = container.Image

			if len(container.Ports) > 0 {
				details.ContainerPort = container.Ports[0].ContainerPort
			}

			if container.Resources != nil {
				details.CPU = container.Resources.Limits["cpu"]
				details.Memory = container.Resources.Limits["memory"]
			}

			// Collect environment variables
			if len(container.Env) > 0 {
				details.EnvVars = make(map[string]string, len(container.Env))
				for _, env := range container.Env {
					if env.ValueSource != nil {
						details.EnvVars[env.Name] = "(secret ref)"
					} else {
						details.EnvVars[env.Name] = env.Value
					}
				}
			}
		}

		details.Concurrency = svc.Template.MaxInstanceRequestConcurrency

		// Parse timeout from duration string (e.g. "300s")
		if svc.Template.Timeout != "" {
			if d, err := time.ParseDuration(svc.Template.Timeout); err == nil {
				details.TimeoutSeconds = int64(d.Seconds())
			}
		}

		// Revision-level scaling
		if svc.Template.Scaling != nil {
			details.MinInstances = svc.Template.Scaling.MinInstanceCount
			details.MaxInstances = svc.Template.Scaling.MaxInstanceCount
		}
	}

	// Service-level scaling overrides template values when present.
	// No >0 guard — MinInstanceCount=0 is valid (scale to zero).
	if svc.Scaling != nil {
		details.MinInstances = svc.Scaling.MinInstanceCount
		details.MaxInstances = svc.Scaling.MaxInstanceCount
	}

	// Traffic targets
	for _, t := range svc.Traffic {
		target := CloudRunTrafficTarget{
			Percent: t.Percent,
			Tag:     t.Tag,
		}
		if t.Type == "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST" {
			target.Type = "LATEST"
			target.RevisionName = "(latest)"
		} else {
			target.Type = "REVISION"
			target.RevisionName = extractShortName(t.Revision)
		}
		details.Traffic = append(details.Traffic, target)
	}

	// Convert via JSON to preserve json struct tags, then marshal to YAML
	rawJSON, err := json.Marshal(svc)
	if err == nil {
		var obj any
		if json.Unmarshal(rawJSON, &obj) == nil {
			if rawYAML, yErr := yaml.Marshal(obj); yErr == nil {
				details.RawYAML = string(rawYAML)
			}
		}
	}

	return details
}

// cloudRunRevisionFromAPI converts an API revision to our domain type
func cloudRunRevisionFromAPI(rev *run.GoogleCloudRunV2Revision) CloudRunRevision {
	var image string
	if len(rev.Containers) > 0 {
		image = rev.Containers[0].Image
	}

	return CloudRunRevision{
		Name:           rev.Name,
		ShortName:      extractShortName(rev.Name),
		Status:         deriveRevisionStatus(rev),
		ContainerImage: image,
		CreatedAt:      rev.CreateTime,
	}
}

// extractShortName gets the last segment from a full resource name.
// "projects/p/locations/us-central1/services/my-svc" → "my-svc"
func extractShortName(fullName string) string {
	parts := strings.Split(fullName, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return fullName
}

// extractRegion gets the location segment from a full resource name.
// "projects/p/locations/us-central1/services/my-svc" → "us-central1"
func extractRegion(fullName string) string {
	parts := strings.Split(fullName, "/")
	for i, part := range parts {
		if part == "locations" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// deriveStatus determines the display status from the API service response.
// The terminal condition of type "Ready" with state "CONDITION_SUCCEEDED" means ready.
// If Reconciling is true, the service is still deploying.
func deriveStatus(svc *run.GoogleCloudRunV2Service) string {
	if svc.TerminalCondition != nil {
		cond := svc.TerminalCondition
		if cond.Type == "Ready" && cond.State == "CONDITION_SUCCEEDED" {
			return "Ready"
		}
		if cond.State == "CONDITION_FAILED" {
			if cond.Reason != "" {
				return cond.Reason
			}
			return "Failed"
		}
	}

	if svc.Reconciling {
		return "Deploying"
	}

	return "Unknown"
}

// deriveRevisionStatus determines the status of a revision from its conditions
func deriveRevisionStatus(rev *run.GoogleCloudRunV2Revision) string {
	for _, cond := range rev.Conditions {
		if cond.Type == "Ready" {
			if cond.State == "CONDITION_SUCCEEDED" {
				return "Ready"
			}
			if cond.State == "CONDITION_FAILED" {
				return "Failed"
			}
		}
	}
	return "Pending"
}

// formatIngress converts the API ingress value to a human-readable string
func formatIngress(ingress string) string {
	switch ingress {
	case "INGRESS_TRAFFIC_ALL":
		return "All"
	case "INGRESS_TRAFFIC_INTERNAL_ONLY":
		return "Internal only"
	case "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER":
		return "Internal + Load Balancer"
	case "INGRESS_TRAFFIC_NONE":
		return "None"
	default:
		return ingress
	}
}

// formatCloudRunTime formats an RFC3339 time string for display
func formatCloudRunTime(t string) string {
	if t == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339Nano, t)
	if err != nil {
		return t
	}
	return parsed.Format("2006-01-02 15:04:05")
}
