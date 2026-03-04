package gcp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CloudRunMetrics aggregates all Cloud Run observability data
type CloudRunMetrics struct {
	RequestCount  []DataPoint // requests/sec over time
	Latency50     []DataPoint // p50 latency in ms
	Latency95     []DataPoint // p95 latency in ms
	Latency99     []DataPoint // p99 latency in ms
	ErrorCount4xx []DataPoint // 4xx error count over time
	ErrorCount5xx []DataPoint // 5xx error count over time
	CPU           []DataPoint // CPU utilization (0-1)
	Memory        []DataPoint // Memory utilization (0-1)
	InstanceCount []DataPoint // Active instance count
	LastFetch     time.Time
}

// cloudRunFilter builds a filter scoped to a specific Cloud Run service.
func cloudRunFilter(serviceName, metricType string) string {
	return fmt.Sprintf(
		`resource.type = "cloud_run_revision" AND resource.labels.service_name = "%s" AND metric.type = "%s"`,
		serviceName, metricType,
	)
}

// GetCloudRunRequestCount fetches total request count for a Cloud Run service.
func (c *MonitoringClient) GetCloudRunRequestCount(ctx context.Context, serviceName string, duration time.Duration) ([]DataPoint, error) {
	filter := cloudRunFilter(serviceName, "run.googleapis.com/request_count")
	return c.fetchMetricData(ctx, filter, duration)
}

// GetCloudRunRequestCountByCode fetches request count filtered by response code class (e.g. "2xx", "4xx", "5xx").
func (c *MonitoringClient) GetCloudRunRequestCountByCode(ctx context.Context, serviceName, codeClass string, duration time.Duration) ([]DataPoint, error) {
	filter := fmt.Sprintf(
		`resource.type = "cloud_run_revision" AND resource.labels.service_name = "%s" AND metric.type = "run.googleapis.com/request_count" AND metric.labels.response_code_class = "%s"`,
		serviceName, codeClass,
	)
	return c.fetchMetricData(ctx, filter, duration)
}

// GetCloudRunRequestLatencies fetches p50, p95, and p99 request latencies in milliseconds.
func (c *MonitoringClient) GetCloudRunRequestLatencies(ctx context.Context, serviceName string, duration time.Duration) (p50, p95, p99 []DataPoint, err error) {
	filter := cloudRunFilter(serviceName, "run.googleapis.com/request_latencies")

	p50, err = c.fetchPercentileData(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_PERCENTILE_50)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to fetch p50 latency: %w", err)
	}

	p95, err = c.fetchPercentileData(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_PERCENTILE_95)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to fetch p95 latency: %w", err)
	}

	p99, err = c.fetchPercentileData(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_PERCENTILE_99)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to fetch p99 latency: %w", err)
	}

	// Convert from seconds to milliseconds
	scaleToMs(p50)
	scaleToMs(p95)
	scaleToMs(p99)

	return p50, p95, p99, nil
}

// GetCloudRunCPUUtilization fetches container CPU utilization (0-1 range).
func (c *MonitoringClient) GetCloudRunCPUUtilization(ctx context.Context, serviceName string, duration time.Duration) ([]DataPoint, error) {
	filter := cloudRunFilter(serviceName, "run.googleapis.com/container/cpu/utilization")
	return c.fetchMetricData(ctx, filter, duration)
}

// GetCloudRunMemoryUtilization fetches container memory utilization (0-1 range).
func (c *MonitoringClient) GetCloudRunMemoryUtilization(ctx context.Context, serviceName string, duration time.Duration) ([]DataPoint, error) {
	filter := cloudRunFilter(serviceName, "run.googleapis.com/container/memory/utilization")
	return c.fetchMetricData(ctx, filter, duration)
}

// GetCloudRunInstanceCount fetches active instance count over time.
func (c *MonitoringClient) GetCloudRunInstanceCount(ctx context.Context, serviceName string, duration time.Duration) ([]DataPoint, error) {
	filter := cloudRunFilter(serviceName, "run.googleapis.com/container/instance_count")
	return c.fetchMetricData(ctx, filter, duration)
}

// fetchPercentileData fetches time series using a percentile aligner.
// Unlike fetchMetricData which uses ALIGN_MEAN, this uses the specified
// percentile aligner (e.g. ALIGN_PERCENTILE_50) for distribution metrics.
func (c *MonitoringClient) fetchPercentileData(ctx context.Context, filter string, duration time.Duration, aligner monitoringpb.Aggregation_Aligner) ([]DataPoint, error) {
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
			AlignmentPeriod:  durationpb.New(60 * time.Second),
			PerSeriesAligner: aligner,
		},
	}

	it := c.metricsClient.ListTimeSeries(ctx, req)
	var dataPoints []DataPoint

	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to fetch time series: %w", err)
		}

		for _, point := range resp.Points {
			timestamp := point.Interval.EndTime.AsTime()
			var value float64

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

// scaleToMs converts data point values from seconds to milliseconds in-place.
func scaleToMs(points []DataPoint) {
	for i := range points {
		points[i].Value *= 1000
	}
}
