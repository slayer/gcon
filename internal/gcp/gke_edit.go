package gcp

import (
	container "google.golang.org/api/container/v1"
)

// ClusterEdit captures fields editable via Clusters.Update / Clusters.SetResourceLabels.
// nil pointer = no change; non-nil = apply.
type ClusterEdit struct {
	ResourceLabels            *map[string]string
	ResourceLabelsFingerprint string // required when ResourceLabels is set (avoids 409)
	LoggingService            *string
	MonitoringService         *string
}

// MaintenanceWindow is set via Clusters.SetMaintenancePolicy. MVP supports
// "none" (clear) and "daily" (single time-of-day) only.
type MaintenanceWindow struct {
	Kind  MaintenanceKind
	Daily string // "HH:MM" UTC, required when Kind == MaintenanceKindDaily
}

// MaintenanceKind identifies the style of maintenance window.
type MaintenanceKind string

const (
	// MaintenanceKindNone clears any existing maintenance window.
	MaintenanceKindNone MaintenanceKind = "none"
	// MaintenanceKindDaily sets a recurring daily maintenance window.
	MaintenanceKindDaily MaintenanceKind = "daily"
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
// SetResourceLabels endpoint. ResourceLabels and ResourceLabelsFingerprint
// from the edit struct must be set before calling this.
func buildSetResourceLabelsRequest(edit ClusterEdit) *container.SetLabelsRequest {
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
