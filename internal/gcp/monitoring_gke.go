package gcp

import (
	"context"
	"fmt"
	"sort"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GKEMetrics aggregates the time-series data the GKE Observability tab renders.
type GKEMetrics struct {
	CPUUtilization    []DataPoint // 0–100 (already scaled from the API's 0–1 allocatable ratio)
	MemoryUtilization []DataPoint // 0–100
	NodeCount         []DataPoint // integer count
	PodCount          []DataPoint // integer count
	NetworkRxBytes    []DataPoint // bytes/sec (sum across nodes)
	NetworkTxBytes    []DataPoint // bytes/sec
	LastFetch         time.Time
}

// gkeClusterFilter builds a Cloud Monitoring filter scoped to one cluster
// at the k8s_cluster resource level (CPU, memory, node count, pod count).
// GCP rejects OR between resource.type values (see .claude/rules/gcp-api-gotchas.md),
// so we pick one resource type per fetch and dispatch from the caller.
func gkeClusterFilter(clusterName, location, metricType string) string {
	return fmt.Sprintf( //nolint:gocritic // GCP filter syntax requires double quotes
		`resource.type = "k8s_cluster" AND resource.labels.cluster_name = "%s" AND resource.labels.location = "%s" AND metric.type = "%s"`,
		clusterName, location, metricType,
	)
}

// gkeNodeFilter builds a Cloud Monitoring filter scoped to all nodes of
// one cluster (network rx/tx aggregated via REDUCE_SUM).
func gkeNodeFilter(clusterName, location, metricType string) string {
	return fmt.Sprintf( //nolint:gocritic // GCP filter syntax requires double quotes
		`resource.type = "k8s_node" AND resource.labels.cluster_name = "%s" AND resource.labels.location = "%s" AND metric.type = "%s"`,
		clusterName, location, metricType,
	)
}

// fetchGKEMetric is the rate/mean fetcher mirrored from fetchLBMetric.
func (c *MonitoringClient) fetchGKEMetric(ctx context.Context, filter string, duration time.Duration, aligner monitoringpb.Aggregation_Aligner, reducer monitoringpb.Aggregation_Reducer) ([]DataPoint, error) {
	endTime := time.Now()
	startTime := endTime.Add(-duration)

	req := &monitoringpb.ListTimeSeriesRequest{
		Name:   fmt.Sprintf("projects/%s", c.projectID),
		Filter: filter,
		Interval: &monitoringpb.TimeInterval{
			StartTime: timestamppb.New(startTime),
			EndTime:   timestamppb.New(endTime),
		},
		Aggregation: &monitoringpb.Aggregation{
			AlignmentPeriod:    durationpb.New(60 * time.Second),
			PerSeriesAligner:   aligner,
			CrossSeriesReducer: reducer,
		},
	}

	points, err := c.collectDataPoints(ctx, req)
	if err != nil {
		return nil, err
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Timestamp.Before(points[j].Timestamp) })
	return points, nil
}

const (
	gkeMetricClusterCPU    = "kubernetes.io/cluster/cpu/allocatable_utilization"
	gkeMetricClusterMemory = "kubernetes.io/cluster/memory/allocatable_utilization"
	gkeMetricNodeCount     = "kubernetes.io/cluster/node_count"
	gkeMetricPodCount      = "kubernetes.io/cluster/pod_count"
	gkeMetricNodeNetworkRx = "kubernetes.io/node/network/received_bytes_count"
	gkeMetricNodeNetworkTx = "kubernetes.io/node/network/sent_bytes_count"
)

// GetGKEClusterCPUUtilization fetches cluster-scope CPU as a 0–100
// percentage. The raw GCP metric is a 0–1 ratio; we scale at the data
// layer so chart consumers can use SetYRange(0, 100) directly.
func (c *MonitoringClient) GetGKEClusterCPUUtilization(ctx context.Context, location, clusterName string, duration time.Duration) ([]DataPoint, error) {
	filter := gkeClusterFilter(clusterName, location, gkeMetricClusterCPU)
	points, err := c.fetchGKEMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_MEAN, monitoringpb.Aggregation_REDUCE_MEAN)
	if err != nil {
		return nil, err
	}
	for i := range points {
		points[i].Value *= 100
	}
	return points, nil
}

// GetGKEClusterMemoryUtilization fetches cluster-scope memory as a 0–100
// percentage (scaled from the API's 0–1 ratio).
func (c *MonitoringClient) GetGKEClusterMemoryUtilization(ctx context.Context, location, clusterName string, duration time.Duration) ([]DataPoint, error) {
	filter := gkeClusterFilter(clusterName, location, gkeMetricClusterMemory)
	points, err := c.fetchGKEMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_MEAN, monitoringpb.Aggregation_REDUCE_MEAN)
	if err != nil {
		return nil, err
	}
	for i := range points {
		points[i].Value *= 100
	}
	return points, nil
}

// GetGKEClusterNodeCount fetches current node count over time.
func (c *MonitoringClient) GetGKEClusterNodeCount(ctx context.Context, location, clusterName string, duration time.Duration) ([]DataPoint, error) {
	filter := gkeClusterFilter(clusterName, location, gkeMetricNodeCount)
	return c.fetchGKEMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_MEAN, monitoringpb.Aggregation_REDUCE_SUM)
}

// GetGKEClusterPodCount fetches running pod count over time.
func (c *MonitoringClient) GetGKEClusterPodCount(ctx context.Context, location, clusterName string, duration time.Duration) ([]DataPoint, error) {
	filter := gkeClusterFilter(clusterName, location, gkeMetricPodCount)
	return c.fetchGKEMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_MEAN, monitoringpb.Aggregation_REDUCE_SUM)
}

// GetGKEClusterNetworkBytes returns aggregate rx/tx bytes-per-second across
// every node in the cluster. Uses the k8s_node resource type with REDUCE_SUM
// so a single time series surfaces total cluster network throughput.
func (c *MonitoringClient) GetGKEClusterNetworkBytes(ctx context.Context, location, clusterName string, duration time.Duration) (rx, tx []DataPoint, err error) {
	rxFilter := gkeNodeFilter(clusterName, location, gkeMetricNodeNetworkRx)
	rx, err = c.fetchGKEMetric(ctx, rxFilter, duration, monitoringpb.Aggregation_ALIGN_RATE, monitoringpb.Aggregation_REDUCE_SUM)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch network rx: %w", err)
	}
	txFilter := gkeNodeFilter(clusterName, location, gkeMetricNodeNetworkTx)
	tx, err = c.fetchGKEMetric(ctx, txFilter, duration, monitoringpb.Aggregation_ALIGN_RATE, monitoringpb.Aggregation_REDUCE_SUM)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch network tx: %w", err)
	}
	return rx, tx, nil
}
