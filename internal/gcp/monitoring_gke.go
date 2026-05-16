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

// gkeNodeFilter builds a Cloud Monitoring filter scoped to all nodes of
// one cluster. Used for CPU/memory utilization (REDUCE_MEAN), node count
// (REDUCE_COUNT), and network rx/tx (REDUCE_SUM).
//
// GCP has no `kubernetes.io/cluster/*` metric family for CPU/memory aggregates
// — those metrics live under `kubernetes.io/node/*` and are aggregated across
// nodes via cross-series reduction. GCP rejects OR between resource.type
// values (see .claude/rules/gcp-api-gotchas.md), so we pick one resource
// type per fetch.
func gkeNodeFilter(clusterName, location, metricType string) string {
	return fmt.Sprintf( //nolint:gocritic // GCP filter syntax requires double quotes
		`resource.type = "k8s_node" AND resource.labels.cluster_name = "%s" AND resource.labels.location = "%s" AND metric.type = "%s"`,
		clusterName, location, metricType,
	)
}

// gkePodFilter builds a Cloud Monitoring filter scoped to all pods of one
// cluster. Used for pod count via REDUCE_COUNT against a pod-level metric.
func gkePodFilter(clusterName, location, metricType string) string {
	return fmt.Sprintf( //nolint:gocritic // GCP filter syntax requires double quotes
		`resource.type = "k8s_pod" AND resource.labels.cluster_name = "%s" AND resource.labels.location = "%s" AND metric.type = "%s"`,
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
	// CPU / memory: per-node allocatable utilization (0–1 ratio per node),
	// reduced across nodes with REDUCE_MEAN to give cluster-average %.
	gkeMetricNodeCPU    = "kubernetes.io/node/cpu/allocatable_utilization"
	gkeMetricNodeMemory = "kubernetes.io/node/memory/allocatable_utilization"
	// Node count: each running node emits one series for allocatable_cores
	// (a gauge); REDUCE_COUNT cross-series counts nodes.
	gkeMetricNodeCores = "kubernetes.io/node/cpu/allocatable_cores"
	// Pod count: each running pod emits one series for received_bytes_count
	// (a cumulative counter, even at rate=0); REDUCE_COUNT cross-series
	// counts pods. Network rx/tx use the node-level equivalents below.
	gkeMetricPodNetworkRx  = "kubernetes.io/pod/network/received_bytes_count"
	gkeMetricNodeNetworkRx = "kubernetes.io/node/network/received_bytes_count"
	gkeMetricNodeNetworkTx = "kubernetes.io/node/network/sent_bytes_count"
)

// GetGKEClusterCPUUtilization returns cluster-average CPU utilization as a
// 0–100 percentage. The raw per-node metric is a 0–1 ratio; REDUCE_MEAN
// across nodes gives the cluster average, which we scale ×100 so chart
// consumers can use SetYRange(0, 100) directly.
func (c *MonitoringClient) GetGKEClusterCPUUtilization(ctx context.Context, location, clusterName string, duration time.Duration) ([]DataPoint, error) {
	filter := gkeNodeFilter(clusterName, location, gkeMetricNodeCPU)
	points, err := c.fetchGKEMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_MEAN, monitoringpb.Aggregation_REDUCE_MEAN)
	if err != nil {
		return nil, err
	}
	for i := range points {
		points[i].Value *= 100
	}
	return points, nil
}

// GetGKEClusterMemoryUtilization returns cluster-average memory utilization
// as a 0–100 percentage (per-node 0–1 ratio, REDUCE_MEAN across nodes).
func (c *MonitoringClient) GetGKEClusterMemoryUtilization(ctx context.Context, location, clusterName string, duration time.Duration) ([]DataPoint, error) {
	filter := gkeNodeFilter(clusterName, location, gkeMetricNodeMemory)
	points, err := c.fetchGKEMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_MEAN, monitoringpb.Aggregation_REDUCE_MEAN)
	if err != nil {
		return nil, err
	}
	for i := range points {
		points[i].Value *= 100
	}
	return points, nil
}

// GetGKEClusterNodeCount returns node count over time. Each running node
// has one allocatable_cores series; REDUCE_COUNT cross-series counts them.
func (c *MonitoringClient) GetGKEClusterNodeCount(ctx context.Context, location, clusterName string, duration time.Duration) ([]DataPoint, error) {
	filter := gkeNodeFilter(clusterName, location, gkeMetricNodeCores)
	return c.fetchGKEMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_MEAN, monitoringpb.Aggregation_REDUCE_COUNT)
}

// GetGKEClusterPodCount returns running pod count over time. Each running
// pod emits one received_bytes_count series (cumulative, kept even at
// rate=0); REDUCE_COUNT cross-series counts them.
func (c *MonitoringClient) GetGKEClusterPodCount(ctx context.Context, location, clusterName string, duration time.Duration) ([]DataPoint, error) {
	filter := gkePodFilter(clusterName, location, gkeMetricPodNetworkRx)
	return c.fetchGKEMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_RATE, monitoringpb.Aggregation_REDUCE_COUNT)
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
