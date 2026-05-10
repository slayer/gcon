package views

// UsageMonitoringRequestMsg asks the App to fetch monitoring metrics for a
// bucket via the usage.Scanner. The App routes ReadyMsg back to the originator
// based on which views are currently mounted and care about this bucket.
type UsageMonitoringRequestMsg struct {
	Bucket string
}

// UsageDeepScanRequestMsg asks the App to start (or join) a deep scan of the
// given (bucket, prefix). Empty Prefix means the entire bucket.
type UsageDeepScanRequestMsg struct {
	Bucket string
	Prefix string
}

// BucketDetailsRequestMsg asks the App to navigate to the BucketDetailsView
// for the named bucket. (Wired to a no-op in Phase 2; the view lands in Phase 3.)
type BucketDetailsRequestMsg struct {
	Bucket string
}
