package gcp

import (
	"context"
	"fmt"
	"time"
)

// FetchBucketUsage returns the most recent (within ~36h) total bytes and
// object count for a bucket via Cloud Monitoring. These metrics are published
// roughly daily by GCP, so values may be up to ~24h stale.
//
// Returns (0, 0, zero-time, nil) if no data exists for the bucket (e.g. brand
// new bucket whose first metric publication has not happened yet). Callers
// should treat zero asOf as "no data yet" rather than as "0 bytes".
func (c *MonitoringClient) FetchBucketUsage(ctx context.Context, bucket string) (bytes, count int64, asOf time.Time, err error) {
	const window = 36 * time.Hour

	bytesFilter := fmt.Sprintf(
		`metric.type="storage.googleapis.com/storage/total_bytes" `+
			`resource.type="gcs_bucket" `+
			`resource.labels.bucket_name="%s"`, bucket)
	countFilter := fmt.Sprintf(
		`metric.type="storage.googleapis.com/storage/object_count" `+
			`resource.type="gcs_bucket" `+
			`resource.labels.bucket_name="%s"`, bucket)

	bytesPoints, err := c.fetch(ctx, bytesFilter, window)
	if err != nil {
		return 0, 0, time.Time{}, fmt.Errorf("fetch bucket bytes: %w", err)
	}
	countPoints, err := c.fetch(ctx, countFilter, window)
	if err != nil {
		return 0, 0, time.Time{}, fmt.Errorf("fetch bucket object count: %w", err)
	}

	// fetchMetricData sorts ascending; the last element is the most recent.
	if n := len(bytesPoints); n > 0 {
		bytes = int64(bytesPoints[n-1].Value)
		asOf = bytesPoints[n-1].Timestamp
	}
	if n := len(countPoints); n > 0 {
		count = int64(countPoints[n-1].Value)
		// Prefer the freshest of the two timestamps if both exist.
		if t := countPoints[n-1].Timestamp; t.After(asOf) {
			asOf = t
		}
	}
	return bytes, count, asOf, nil
}
