package gcp

import (
	"context"
	"fmt"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// MonitoringClient provides access to Cloud Monitoring metrics
type MonitoringClient struct {
	metricsClient *monitoring.MetricClient
	projectID     string
}

// DataPoint represents a single metric data point
type DataPoint struct {
	Timestamp time.Time
	Value     float64
}

// NetworkMetrics contains network traffic statistics
type NetworkMetrics struct {
	SentBytesPerSec     float64
	ReceivedBytesPerSec float64
	TotalSentBytes      float64
	TotalReceivedBytes  float64
}

// DiskMetrics contains disk I/O statistics
type DiskMetrics struct {
	DiskName         string
	ReadOpsPerSec    float64
	WriteOpsPerSec   float64
	ReadBytesPerSec  float64
	WriteBytesPerSec float64
	TotalReadBytes   float64
	TotalWriteBytes  float64
}

// ObservabilityMetrics aggregates all observability data
type ObservabilityMetrics struct {
	CPU       []DataPoint
	Memory    []DataPoint
	Network   NetworkMetrics
	Disks     []DiskMetrics
	Uptime    time.Duration
	LastFetch time.Time
}

// NewMonitoringClient creates a new Cloud Monitoring client
func NewMonitoringClient(ctx context.Context, projectID string) (*MonitoringClient, error) {
	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create monitoring client: %w", err)
	}

	return &MonitoringClient{
		metricsClient: client,
		projectID:     projectID,
	}, nil
}

// Close closes the monitoring client
func (c *MonitoringClient) Close() error {
	return c.metricsClient.Close()
}

// GetCPUUtilization fetches CPU utilization metrics
func (c *MonitoringClient) GetCPUUtilization(ctx context.Context, instanceID, zone string, duration time.Duration) ([]DataPoint, error) {
	filter := fmt.Sprintf(`resource.type = "gce_instance"
		AND resource.labels.instance_id = "%s"
		AND resource.labels.zone = "%s"
		AND metric.type = "compute.googleapis.com/instance/cpu/utilization"`, instanceID, zone)

	return c.fetchMetricData(ctx, filter, duration)
}

// GetMemoryUtilization fetches memory utilization metrics (requires Ops Agent)
func (c *MonitoringClient) GetMemoryUtilization(ctx context.Context, instanceID, zone string, duration time.Duration) ([]DataPoint, error) {
	filter := fmt.Sprintf(`resource.type = "gce_instance"
		AND resource.labels.instance_id = "%s"
		AND resource.labels.zone = "%s"
		AND metric.type = "agent.googleapis.com/memory/percent_used"`, instanceID, zone)

	return c.fetchMetricData(ctx, filter, duration)
}

// GetNetworkTraffic fetches network traffic metrics
func (c *MonitoringClient) GetNetworkTraffic(ctx context.Context, instanceID, zone string, duration time.Duration) (NetworkMetrics, error) {
	var metrics NetworkMetrics

	// Fetch sent bytes
	sentFilter := fmt.Sprintf(`resource.type = "gce_instance"
		AND resource.labels.instance_id = "%s"
		AND resource.labels.zone = "%s"
		AND metric.type = "compute.googleapis.com/instance/network/sent_bytes_count"`, instanceID, zone)

	sentData, err := c.fetchMetricData(ctx, sentFilter, duration)
	if err != nil {
		return metrics, fmt.Errorf("failed to fetch sent bytes: %w", err)
	}

	// Fetch received bytes
	recvFilter := fmt.Sprintf(`resource.type = "gce_instance"
		AND resource.labels.instance_id = "%s"
		AND resource.labels.zone = "%s"
		AND metric.type = "compute.googleapis.com/instance/network/received_bytes_count"`, instanceID, zone)

	recvData, err := c.fetchMetricData(ctx, recvFilter, duration)
	if err != nil {
		return metrics, fmt.Errorf("failed to fetch received bytes: %w", err)
	}

	// Calculate rates and totals
	if len(sentData) > 0 {
		metrics.SentBytesPerSec = sentData[len(sentData)-1].Value
		for _, dp := range sentData {
			metrics.TotalSentBytes += dp.Value
		}
	}

	if len(recvData) > 0 {
		metrics.ReceivedBytesPerSec = recvData[len(recvData)-1].Value
		for _, dp := range recvData {
			metrics.TotalReceivedBytes += dp.Value
		}
	}

	return metrics, nil
}

// GetDiskIO fetches disk I/O metrics for all attached disks
func (c *MonitoringClient) GetDiskIO(ctx context.Context, instanceID, zone string, duration time.Duration) ([]DiskMetrics, error) {
	// Fetch read operations
	readOpsFilter := fmt.Sprintf(`resource.type = "gce_instance"
		AND resource.labels.instance_id = "%s"
		AND resource.labels.zone = "%s"
		AND metric.type = "compute.googleapis.com/instance/disk/read_ops_count"`, instanceID, zone)

	// Fetch write operations
	writeOpsFilter := fmt.Sprintf(`resource.type = "gce_instance"
		AND resource.labels.instance_id = "%s"
		AND resource.labels.zone = "%s"
		AND metric.type = "compute.googleapis.com/instance/disk/write_ops_count"`, instanceID, zone)

	// Fetch read bytes
	readBytesFilter := fmt.Sprintf(`resource.type = "gce_instance"
		AND resource.labels.instance_id = "%s"
		AND resource.labels.zone = "%s"
		AND metric.type = "compute.googleapis.com/instance/disk/read_bytes_count"`, instanceID, zone)

	// Fetch write bytes
	writeBytesFilter := fmt.Sprintf(`resource.type = "gce_instance"
		AND resource.labels.instance_id = "%s"
		AND resource.labels.zone = "%s"
		AND metric.type = "compute.googleapis.com/instance/disk/write_bytes_count"`, instanceID, zone)

	readOpsData, err := c.fetchMetricData(ctx, readOpsFilter, duration)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch read ops: %w", err)
	}

	writeOpsData, err := c.fetchMetricData(ctx, writeOpsFilter, duration)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch write ops: %w", err)
	}

	readBytesData, err := c.fetchMetricData(ctx, readBytesFilter, duration)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch read bytes: %w", err)
	}

	writeBytesData, err := c.fetchMetricData(ctx, writeBytesFilter, duration)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch write bytes: %w", err)
	}

	// Aggregate by disk (for now, assume one disk; extend later for multiple)
	var metrics []DiskMetrics
	diskMetric := DiskMetrics{
		DiskName: "boot-disk",
	}

	if len(readOpsData) > 0 {
		diskMetric.ReadOpsPerSec = readOpsData[len(readOpsData)-1].Value
		for _, dp := range readOpsData {
			diskMetric.TotalReadBytes += dp.Value
		}
	}

	if len(writeOpsData) > 0 {
		diskMetric.WriteOpsPerSec = writeOpsData[len(writeOpsData)-1].Value
		for _, dp := range writeOpsData {
			diskMetric.TotalWriteBytes += dp.Value
		}
	}

	if len(readBytesData) > 0 {
		diskMetric.ReadBytesPerSec = readBytesData[len(readBytesData)-1].Value
	}

	if len(writeBytesData) > 0 {
		diskMetric.WriteBytesPerSec = writeBytesData[len(writeBytesData)-1].Value
	}

	metrics = append(metrics, diskMetric)
	return metrics, nil
}

// fetchMetricData is a helper to fetch time series data
func (c *MonitoringClient) fetchMetricData(ctx context.Context, filter string, duration time.Duration) ([]DataPoint, error) {
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
			AlignmentPeriod:  durationpb.New(60 * time.Second), // 1-minute intervals
			PerSeriesAligner: monitoringpb.Aggregation_ALIGN_MEAN,
		},
	}

	it := c.metricsClient.ListTimeSeries(ctx, req)
	var dataPoints []DataPoint

	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to fetch time series: %w", err)
		}

		// Extract points from the time series
		for _, point := range resp.Points {
			timestamp := point.Interval.EndTime.AsTime()
			var value float64

			// Handle different value types
			switch v := point.Value.Value.(type) {
			case *monitoringpb.TypedValue_DoubleValue:
				value = v.DoubleValue
			case *monitoringpb.TypedValue_Int64Value:
				value = float64(v.Int64Value)
			}

			dataPoints = append(dataPoints, DataPoint{
				Timestamp: timestamp,
				Value:     value,
			})
		}
	}

	return dataPoints, nil
}
