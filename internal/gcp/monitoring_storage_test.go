package gcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFetchBucketUsage_BothMetricsPresent verifies bytes and count are joined.
func TestFetchBucketUsage_BothMetricsPresent(t *testing.T) {
	now := time.Now()
	c := &MonitoringClient{
		projectID: "test-project",
		fetchFunc: func(ctx context.Context, filter string, duration time.Duration) ([]DataPoint, error) {
			// Two filters in sequence: total_bytes then object_count.
			if strings.Contains(filter, "total_bytes") {
				return []DataPoint{{Timestamp: now.Add(-2 * time.Hour), Value: 1.5e9}}, nil
			}
			if strings.Contains(filter, "object_count") {
				return []DataPoint{{Timestamp: now.Add(-2 * time.Hour), Value: 4321}}, nil
			}
			return nil, nil
		},
	}
	bytes, count, asOf, err := c.FetchBucketUsage(context.Background(), "my-bucket")
	require.NoError(t, err)
	assert.Equal(t, int64(1_500_000_000), bytes)
	assert.Equal(t, int64(4321), count)
	assert.WithinDuration(t, now.Add(-2*time.Hour), asOf, time.Second)
}

func TestFetchBucketUsage_NoData(t *testing.T) {
	c := &MonitoringClient{
		projectID: "test-project",
		fetchFunc: func(ctx context.Context, filter string, duration time.Duration) ([]DataPoint, error) {
			return nil, nil
		},
	}
	bytes, count, asOf, err := c.FetchBucketUsage(context.Background(), "new-bucket")
	require.NoError(t, err)
	assert.Equal(t, int64(0), bytes)
	assert.Equal(t, int64(0), count)
	assert.True(t, asOf.IsZero())
}

func TestFetchBucketUsage_OnlyBytes(t *testing.T) {
	now := time.Now()
	c := &MonitoringClient{
		projectID: "test-project",
		fetchFunc: func(ctx context.Context, filter string, duration time.Duration) ([]DataPoint, error) {
			if strings.Contains(filter, "total_bytes") {
				return []DataPoint{{Timestamp: now, Value: 100}}, nil
			}
			return nil, nil
		},
	}
	bytes, count, asOf, err := c.FetchBucketUsage(context.Background(), "b")
	require.NoError(t, err)
	assert.Equal(t, int64(100), bytes)
	assert.Equal(t, int64(0), count)
	assert.WithinDuration(t, now, asOf, time.Second)
}

