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
	Command        []string // Container entrypoint override
	Args           []string // Container args override

	// Networking
	VPCConnector string // Full VPC connector resource name
	VPCEgress    string // Human-readable egress setting

	// Raw API ingress value for round-tripping through edit forms
	IngressRaw string

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

// CloudRunServiceUpdate contains the editable fields for a Cloud Run service.
// Only non-nil/non-zero fields are included in the Patch request.
type CloudRunServiceUpdate struct {
	// Service-level
	Description *string
	Ingress     *string           // Raw API value: "INGRESS_TRAFFIC_ALL", etc.
	Labels      map[string]string // nil = don't change

	// Container config
	Image   *string
	Port    *int64
	Command []string          // nil = don't change, empty slice = clear
	Args    []string          // nil = don't change, empty slice = clear
	EnvVars map[string]string // Plain-text env vars only; nil = don't change
	CPU     *string
	Memory  *string

	// Scaling & performance
	MinInstances *int64
	MaxInstances *int64
	Concurrency  *int64
	Timeout      *int64 // Seconds, converted to duration string for API

	// Security
	ServiceAccount *string

	// Networking
	VPCConnector *string // Full connector path or empty to clear
	VPCEgress    *string // "ALL_TRAFFIC" or "PRIVATE_RANGES_ONLY"
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

// UpdateService patches an existing Cloud Run service with the given changes.
// Only fields set in the update struct are included in the Patch request via UpdateMask.
func (c *CloudRunClient) UpdateService(ctx context.Context, fullName string, update *CloudRunServiceUpdate) error {
	svc, maskPaths := buildServicePatch(update)

	if len(maskPaths) == 0 {
		return nil // Nothing to update
	}

	mask := strings.Join(maskPaths, ",")
	_, err := c.service.Projects.Locations.Services.Patch(fullName, svc).
		UpdateMask(mask).
		Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "update cloud run service", extractShortName(fullName))
	}
	return nil
}

// CreateService creates a new Cloud Run service from the given configuration.
func (c *CloudRunClient) CreateService(ctx context.Context, projectID, region, name string, config *CloudRunServiceUpdate) error {
	parent := fmt.Sprintf("projects/%s/locations/%s", projectID, region)

	svc := buildServiceFromConfig(config)

	_, err := c.service.Projects.Locations.Services.Create(parent, svc).
		ServiceId(name).
		Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "create cloud run service", name)
	}
	return nil
}

// buildServicePatch constructs the API service struct and UpdateMask paths from an update.
// Container fields (image, port, command, args, env, cpu, memory) share one mask entry
// because the API replaces the entire containers array atomically.
func buildServicePatch(update *CloudRunServiceUpdate) (svc *run.GoogleCloudRunV2Service, maskPaths []string) {
	svc = &run.GoogleCloudRunV2Service{}
	var forceSend []string

	// Service-level fields
	if update.Description != nil {
		svc.Description = *update.Description
		maskPaths = append(maskPaths, "description")
		forceSend = append(forceSend, "Description")
	}
	if update.Ingress != nil {
		svc.Ingress = *update.Ingress
		maskPaths = append(maskPaths, "ingress")
		forceSend = append(forceSend, "Ingress")
	}
	if update.Labels != nil {
		svc.Labels = update.Labels
		maskPaths = append(maskPaths, "labels")
		forceSend = append(forceSend, "Labels")
	}

	// Template-level fields
	template := &run.GoogleCloudRunV2RevisionTemplate{}
	templateForce := []string{}
	needTemplate := false

	// Container fields — rebuild container if any container field changes
	needContainer := update.Image != nil || update.Port != nil ||
		update.Command != nil || update.Args != nil ||
		update.EnvVars != nil || update.CPU != nil || update.Memory != nil

	if needContainer {
		container := buildContainerFromUpdate(update)
		template.Containers = []*run.GoogleCloudRunV2Container{container}
		maskPaths = append(maskPaths, "template.containers")
		needTemplate = true
	}

	// Scaling
	if update.MinInstances != nil || update.MaxInstances != nil {
		scaling := &run.GoogleCloudRunV2RevisionScaling{}
		scalingForce := []string{}
		if update.MinInstances != nil {
			scaling.MinInstanceCount = *update.MinInstances
			scalingForce = append(scalingForce, "MinInstanceCount")
			maskPaths = append(maskPaths, "template.scaling.minInstanceCount")
		}
		if update.MaxInstances != nil {
			scaling.MaxInstanceCount = *update.MaxInstances
			scalingForce = append(scalingForce, "MaxInstanceCount")
			maskPaths = append(maskPaths, "template.scaling.maxInstanceCount")
		}
		scaling.ForceSendFields = scalingForce
		template.Scaling = scaling
		needTemplate = true
	}

	if update.Concurrency != nil {
		template.MaxInstanceRequestConcurrency = *update.Concurrency
		templateForce = append(templateForce, "MaxInstanceRequestConcurrency")
		maskPaths = append(maskPaths, "template.maxInstanceRequestConcurrency")
		needTemplate = true
	}

	if update.Timeout != nil {
		template.Timeout = fmt.Sprintf("%ds", *update.Timeout)
		templateForce = append(templateForce, "Timeout")
		maskPaths = append(maskPaths, "template.timeout")
		needTemplate = true
	}

	if update.ServiceAccount != nil {
		template.ServiceAccount = *update.ServiceAccount
		templateForce = append(templateForce, "ServiceAccount")
		maskPaths = append(maskPaths, "template.serviceAccount")
		needTemplate = true
	}

	// VPC access
	if update.VPCConnector != nil || update.VPCEgress != nil {
		vpcAccess := &run.GoogleCloudRunV2VpcAccess{}
		vpcForce := []string{}
		if update.VPCConnector != nil {
			vpcAccess.Connector = *update.VPCConnector
			vpcForce = append(vpcForce, "Connector")
		}
		if update.VPCEgress != nil {
			vpcAccess.Egress = *update.VPCEgress
			vpcForce = append(vpcForce, "Egress")
		}
		vpcAccess.ForceSendFields = vpcForce
		template.VpcAccess = vpcAccess
		maskPaths = append(maskPaths, "template.vpcAccess")
		needTemplate = true
	}

	if needTemplate {
		template.ForceSendFields = templateForce
		svc.Template = template
	}
	svc.ForceSendFields = forceSend

	return svc, maskPaths
}

// buildContainerFromUpdate assembles a container spec from update fields.
func buildContainerFromUpdate(update *CloudRunServiceUpdate) *run.GoogleCloudRunV2Container {
	container := &run.GoogleCloudRunV2Container{}
	containerForce := []string{}

	if update.Image != nil {
		container.Image = *update.Image
		containerForce = append(containerForce, "Image")
	}
	if update.Port != nil {
		container.Ports = []*run.GoogleCloudRunV2ContainerPort{{ContainerPort: *update.Port}}
	}
	if update.Command != nil {
		container.Command = update.Command
		containerForce = append(containerForce, "Command")
	}
	if update.Args != nil {
		container.Args = update.Args
		containerForce = append(containerForce, "Args")
	}
	if update.EnvVars != nil {
		envs := make([]*run.GoogleCloudRunV2EnvVar, 0, len(update.EnvVars))
		for k, v := range update.EnvVars {
			envs = append(envs, &run.GoogleCloudRunV2EnvVar{Name: k, Value: v})
		}
		container.Env = envs
	}
	if update.CPU != nil || update.Memory != nil {
		limits := make(map[string]string)
		if update.CPU != nil {
			limits["cpu"] = *update.CPU
		}
		if update.Memory != nil {
			limits["memory"] = *update.Memory
		}
		container.Resources = &run.GoogleCloudRunV2ResourceRequirements{Limits: limits}
	}

	container.ForceSendFields = containerForce
	return container
}

// buildServiceFromConfig builds a full service spec for Create operations.
func buildServiceFromConfig(config *CloudRunServiceUpdate) *run.GoogleCloudRunV2Service {
	svc := &run.GoogleCloudRunV2Service{}

	if config.Description != nil {
		svc.Description = *config.Description
	}
	if config.Ingress != nil {
		svc.Ingress = *config.Ingress
	}
	if config.Labels != nil {
		svc.Labels = config.Labels
	}

	template := &run.GoogleCloudRunV2RevisionTemplate{}
	container := buildContainerFromUpdate(config)
	template.Containers = []*run.GoogleCloudRunV2Container{container}

	if config.MinInstances != nil || config.MaxInstances != nil {
		scaling := &run.GoogleCloudRunV2RevisionScaling{}
		if config.MinInstances != nil {
			scaling.MinInstanceCount = *config.MinInstances
		}
		if config.MaxInstances != nil {
			scaling.MaxInstanceCount = *config.MaxInstances
		}
		template.Scaling = scaling
	}
	if config.Concurrency != nil {
		template.MaxInstanceRequestConcurrency = *config.Concurrency
	}
	if config.Timeout != nil {
		template.Timeout = fmt.Sprintf("%ds", *config.Timeout)
	}
	if config.ServiceAccount != nil {
		template.ServiceAccount = *config.ServiceAccount
	}
	if config.VPCConnector != nil || config.VPCEgress != nil {
		vpcAccess := &run.GoogleCloudRunV2VpcAccess{}
		if config.VPCConnector != nil {
			vpcAccess.Connector = *config.VPCConnector
		}
		if config.VPCEgress != nil {
			vpcAccess.Egress = *config.VPCEgress
		}
		template.VpcAccess = vpcAccess
	}

	svc.Template = template
	return svc
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
		IngressRaw:     svc.Ingress,
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
			details.Command = container.Command
			details.Args = container.Args

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

		// VPC access configuration
		if svc.Template.VpcAccess != nil {
			details.VPCConnector = svc.Template.VpcAccess.Connector
			details.VPCEgress = formatVPCEgress(svc.Template.VpcAccess.Egress)
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

// formatVPCEgress converts the API VPC egress value to a human-readable string
func formatVPCEgress(egress string) string {
	switch egress {
	case "ALL_TRAFFIC":
		return "All traffic"
	case "PRIVATE_RANGES_ONLY":
		return "Private ranges only"
	default:
		return egress
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
