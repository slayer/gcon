package gcp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	container "google.golang.org/api/container/v1"
)

// sentinel errors for edit-method dispatch validation. Returned directly
// (not wrapped) since the sentinel message is self-contained.
var (
	errNoClusterEditFields  = errors.New("UpdateClusterBasic: no fields to update")
	errMixedClusterEdit     = errors.New("UpdateClusterBasic: labels and services must be sent as separate calls")
	errNoNodePoolEditFields = errors.New("UpdateNodePoolFields: no fields to update")
)

// ClusterEdit captures fields editable via Clusters.Update / Clusters.SetResourceLabels.
// nil pointer = no change; non-nil = apply.
type ClusterEdit struct {
	ResourceLabels            *map[string]string
	ResourceLabelsFingerprint string // required when ResourceLabels is set (avoids 409)
	LoggingService            *string
	MonitoringService         *string
}

// MaintenanceWindow is set via Clusters.SetMaintenancePolicy.
type MaintenanceWindow struct {
	Kind  MaintenanceKind
	Daily string // "HH:MM" UTC, required when Kind == MaintenanceKindDaily

	// ─── Phase 2d: recurring (weekly) ───
	// Used when Kind == MaintenanceKindRecurring.
	Days     []string // subset of {"MO","TU","WE","TH","FR","SA","SU"}, sorted by caller
	Start    string   // "HH:MM" UTC
	Duration string   // "Nh", 1-23
}

// MaintenanceKind identifies the style of maintenance window.
type MaintenanceKind string

const (
	// MaintenanceKindNone clears any existing maintenance window.
	MaintenanceKindNone MaintenanceKind = "none"
	// MaintenanceKindDaily sets a recurring daily maintenance window.
	MaintenanceKindDaily MaintenanceKind = "daily"
	// MaintenanceKindRecurring sets a weekly recurring maintenance window by
	// day-of-week, start time, and duration.
	MaintenanceKindRecurring MaintenanceKind = "recurring"
)

// NodePoolEdit captures fields editable via NodePools.Update.
// nil pointer fields are left unchanged on the node pool.
type NodePoolEdit struct {
	Labels          *map[string]string
	Taints          *[]NodeTaint
	Tags            *[]string
	UpgradeSettings *UpgradeSettings
}

// NodeTaint is a single Kubernetes node taint.
type NodeTaint struct {
	Key, Value, Effect string // Effect: NO_SCHEDULE | PREFER_NO_SCHEDULE | NO_EXECUTE
}

// UpgradeSettings controls how node pool upgrades are executed.
type UpgradeSettings struct {
	MaxSurge       int64
	MaxUnavailable int64
	Strategy       string // SURGE | BLUE_GREEN
}

// NodePoolManagement is set via NodePools.SetManagement.
type NodePoolManagement struct {
	AutoUpgrade, AutoRepair bool
}

// buildClusterUpdate maps ClusterEdit into container.ClusterUpdate.
// Only LoggingService and MonitoringService go through this builder —
// ResourceLabels uses a different endpoint (SetResourceLabels) and is
// NOT included in the output.
func buildClusterUpdate(edit ClusterEdit) *container.ClusterUpdate {
	upd := &container.ClusterUpdate{}
	if edit.LoggingService != nil {
		upd.DesiredLoggingService = *edit.LoggingService
		upd.ForceSendFields = append(upd.ForceSendFields, "DesiredLoggingService")
	}
	if edit.MonitoringService != nil {
		upd.DesiredMonitoringService = *edit.MonitoringService
		upd.ForceSendFields = append(upd.ForceSendFields, "DesiredMonitoringService")
	}
	return upd
}

// buildSetResourceLabelsRequest produces the SetLabelsRequest for the
// SetResourceLabels endpoint. Returns nil if ResourceLabels is unset —
// callers should branch on nil rather than rely on a panic.
func buildSetResourceLabelsRequest(edit ClusterEdit) *container.SetLabelsRequest {
	if edit.ResourceLabels == nil {
		return nil
	}
	return &container.SetLabelsRequest{
		ResourceLabels:   *edit.ResourceLabels,
		LabelFingerprint: edit.ResourceLabelsFingerprint,
	}
}

// buildNodePoolUpdate maps NodePoolEdit into container.UpdateNodePoolRequest.
// Only fields explicitly set (non-nil) on edit appear in the output.
func buildNodePoolUpdate(edit NodePoolEdit) *container.UpdateNodePoolRequest {
	req := &container.UpdateNodePoolRequest{}
	if edit.Labels != nil {
		req.Labels = &container.NodeLabels{Labels: *edit.Labels}
	}
	if edit.Taints != nil {
		taints := make([]*container.NodeTaint, 0, len(*edit.Taints))
		for _, t := range *edit.Taints {
			taints = append(taints, &container.NodeTaint{
				Key:    t.Key,
				Value:  t.Value,
				Effect: t.Effect,
			})
		}
		req.Taints = &container.NodeTaints{Taints: taints}
	}
	if edit.Tags != nil {
		req.Tags = &container.NetworkTags{Tags: *edit.Tags}
	}
	if edit.UpgradeSettings != nil {
		req.UpgradeSettings = &container.UpgradeSettings{
			MaxSurge:        edit.UpgradeSettings.MaxSurge,
			MaxUnavailable:  edit.UpgradeSettings.MaxUnavailable,
			Strategy:        edit.UpgradeSettings.Strategy,
			ForceSendFields: []string{"MaxSurge", "MaxUnavailable"},
		}
	}
	return req
}

// buildSetNodePoolManagementRequest ensures both bool fields survive JSON
// marshaling even when both are false. Without ForceSendFields, a false
// value is omitted and the API sees the existing setting rather than an
// explicit disable.
func buildSetNodePoolManagementRequest(mgmt NodePoolManagement) *container.SetNodePoolManagementRequest {
	return &container.SetNodePoolManagementRequest{
		Management: &container.NodeManagement{
			AutoUpgrade:     mgmt.AutoUpgrade,
			AutoRepair:      mgmt.AutoRepair,
			ForceSendFields: []string{"AutoUpgrade", "AutoRepair"},
		},
	}
}

// buildSetMaintenancePolicyRequest renders a MaintenanceWindow into the
// API's nested struct. Kind=="none" clears the window by sending an empty
// MaintenanceWindow. Kind=="daily" sends DailyMaintenanceWindow with HH:MM.
// Kind=="recurring" sends a RecurringTimeWindow with a weekly RRULE.
func buildSetMaintenancePolicyRequest(mw MaintenanceWindow) *container.SetMaintenancePolicyRequest {
	policy := &container.MaintenancePolicy{}
	switch mw.Kind {
	case MaintenanceKindDaily:
		policy.Window = &container.MaintenanceWindow{
			DailyMaintenanceWindow: &container.DailyMaintenanceWindow{StartTime: mw.Daily},
		}
	case MaintenanceKindRecurring:
		start := composeRecurringStart(mw.Start)
		end := composeRecurringEnd(mw.Start, mw.Duration)
		policy.Window = &container.MaintenanceWindow{
			RecurringWindow: &container.RecurringTimeWindow{
				Window:     &container.TimeWindow{StartTime: start, EndTime: end},
				Recurrence: "FREQ=WEEKLY;BYDAY=" + strings.Join(mw.Days, ","),
			},
		}
	default: // MaintenanceKindNone or unset — empty window clears any existing policy
		policy.Window = &container.MaintenanceWindow{}
	}
	return &container.SetMaintenancePolicyRequest{MaintenancePolicy: policy}
}

// recurringBaseline is the baseline date for the recurring window's first
// occurrence. 2026-01-04 is a Sunday — a convenient anchor since the RRULE
// makes the actual date irrelevant (GCP only uses time-of-day and BYDAY).
var recurringBaseline = time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)

// composeRecurringStart returns the RFC3339 datetime for the first
// occurrence's start. "HH:MM" gets layered onto the baseline date.
// On parse failure, returns the baseline at midnight.
func composeRecurringStart(timeOfDay string) string {
	h, m, ok := parseHHMM(timeOfDay)
	if !ok {
		return recurringBaseline.Format(time.RFC3339)
	}
	return recurringBaseline.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).Format(time.RFC3339)
}

// composeRecurringEnd returns the RFC3339 datetime for the first
// occurrence's end. duration is "Nh"; clamped to 1-23, fallback 4.
func composeRecurringEnd(timeOfDay, duration string) string {
	h, m, ok := parseHHMM(timeOfDay)
	if !ok {
		h, m = 0, 0
	}
	dur := parseHoursOr(duration, 4)
	return recurringBaseline.
		Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(dur)*time.Hour).
		Format(time.RFC3339)
}

// parseHHMM parses a "HH:MM" string into hours and minutes.
// Returns (h, m int, ok bool); ok is false if the format is invalid or values are out of range.
func parseHHMM(s string) (h int, m int, ok bool) {
	if len(s) != 5 || s[2] != ':' {
		return 0, 0, false
	}
	var err1, err2 error
	h, err1 = strconv.Atoi(s[0:2])
	m, err2 = strconv.Atoi(s[3:5])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

// parseHoursOr parses a duration string like "4h" into an integer hour count.
// Values outside [1, 23] return the fallback.
func parseHoursOr(s string, fallback int) int {
	s = strings.TrimSuffix(s, "h")
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 23 {
		return fallback
	}
	return n
}

// UpdateClusterBasic dispatches a ClusterEdit to the appropriate GKE API
// endpoint. Exactly one category may be set per call:
//   - ResourceLabels != nil  →  Clusters.SetResourceLabels
//   - LoggingService or MonitoringService set  →  Clusters.Update
//
// Passing both categories in one call returns an error; the app layer
// must submit them as sequential steps.
func (c *ContainerClient) UpdateClusterBasic(
	ctx context.Context,
	projectID, location, clusterName string,
	edit ClusterEdit,
) (Operation, error) {
	hasLabels := edit.ResourceLabels != nil
	hasService := edit.LoggingService != nil || edit.MonitoringService != nil

	switch {
	case !hasLabels && !hasService:
		return Operation{}, errNoClusterEditFields
	case hasLabels && hasService:
		return Operation{}, errMixedClusterEdit
	case hasLabels:
		req := buildSetResourceLabelsRequest(edit)
		raw, err := c.service.Projects.Locations.Clusters.
			SetResourceLabels(clusterFQN(projectID, location, clusterName), req).
			Context(ctx).Do()
		if err != nil {
			return Operation{}, fmt.Errorf("set cluster resource labels: %w", err)
		}
		return projectOperation(raw), nil
	default: // hasService
		upd := buildClusterUpdate(edit)
		raw, err := c.service.Projects.Locations.Clusters.
			Update(clusterFQN(projectID, location, clusterName),
				&container.UpdateClusterRequest{Update: upd}).
			Context(ctx).Do()
		if err != nil {
			return Operation{}, fmt.Errorf("update cluster: %w", err)
		}
		return projectOperation(raw), nil
	}
}

// SetClusterMaintenancePolicy applies a maintenance window to the cluster.
// Sending Kind==MaintenanceKindNone clears any existing window.
func (c *ContainerClient) SetClusterMaintenancePolicy(
	ctx context.Context,
	projectID, location, clusterName string,
	mw MaintenanceWindow,
) (Operation, error) {
	req := buildSetMaintenancePolicyRequest(mw)
	raw, err := c.service.Projects.Locations.Clusters.
		SetMaintenancePolicy(clusterFQN(projectID, location, clusterName), req).
		Context(ctx).Do()
	if err != nil {
		return Operation{}, fmt.Errorf("set maintenance policy: %w", err)
	}
	return projectOperation(raw), nil
}

// UpdateNodePoolFields patches labels, taints, tags, or upgrade settings on a
// node pool. At least one field must be set; an empty edit returns an error.
func (c *ContainerClient) UpdateNodePoolFields(
	ctx context.Context,
	projectID, location, clusterName, poolName string,
	edit NodePoolEdit,
) (Operation, error) {
	if edit.Labels == nil && edit.Taints == nil && edit.Tags == nil && edit.UpgradeSettings == nil {
		return Operation{}, errNoNodePoolEditFields
	}
	req := buildNodePoolUpdate(edit)
	raw, err := c.service.Projects.Locations.Clusters.NodePools.
		Update(nodePoolFQN(projectID, location, clusterName, poolName), req).
		Context(ctx).Do()
	if err != nil {
		return Operation{}, fmt.Errorf("update node pool fields: %w", err)
	}
	return projectOperation(raw), nil
}

// SetNodePoolManagement sets auto-upgrade and auto-repair flags on a node pool.
// Both bool fields are always sent via ForceSendFields so explicit false values
// are not silently omitted.
func (c *ContainerClient) SetNodePoolManagement(
	ctx context.Context,
	projectID, location, clusterName, poolName string,
	mgmt NodePoolManagement,
) (Operation, error) {
	req := buildSetNodePoolManagementRequest(mgmt)
	raw, err := c.service.Projects.Locations.Clusters.NodePools.
		SetManagement(nodePoolFQN(projectID, location, clusterName, poolName), req).
		Context(ctx).Do()
	if err != nil {
		return Operation{}, fmt.Errorf("set node pool management: %w", err)
	}
	return projectOperation(raw), nil
}
