package gcp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLBFilterContainsRuleAndMetric(t *testing.T) {
	f := lbFilter("https_lb_rule", "my-rule", "loadbalancing.googleapis.com/https/request_count")
	assert.Contains(t, f, `resource.type = "https_lb_rule"`)
	assert.Contains(t, f, `forwarding_rule_name = "my-rule"`)
	assert.Contains(t, f, `metric.type = "loadbalancing.googleapis.com/https/request_count"`)
	// GCP rejects OR between resource.type values — regression check.
	assert.False(t, strings.Contains(f, " OR "), "filter must not contain OR between resource types")
}

func TestLBFilterInternalResourceType(t *testing.T) {
	f := lbFilter("internal_http_lb_rule", "rule-int", "loadbalancing.googleapis.com/https/request_count")
	assert.Contains(t, f, `resource.type = "internal_http_lb_rule"`)
	assert.NotContains(t, f, "https_lb_rule")
}

func TestGetLBRequestCountFilter(t *testing.T) {
	f := lbFilter("https_lb_rule", "rule-x", "loadbalancing.googleapis.com/https/request_count")
	assert.Contains(t, f, "https/request_count")
	assert.Contains(t, f, `forwarding_rule_name = "rule-x"`)
}

func TestGetLBRequestCountByCodeClassFilter(t *testing.T) {
	// response_code_class is an INTEGER label on LB metrics — must not be
	// quoted, otherwise the API rejects with "cannot be parsed as an
	// integer." Use lbFilterWithIntLabel, not lbFilterWithLabel.
	f := lbFilterWithIntLabel("https_lb_rule", "rule-x", "loadbalancing.googleapis.com/https/request_count", "response_code_class", 5)
	assert.Contains(t, f, `metric.labels.response_code_class = 5`)
	assert.NotContains(t, f, `response_code_class = "5"`)
	assert.Contains(t, f, "https/request_count")
}

func TestGetLBRequestLatenciesFilter(t *testing.T) {
	f := lbFilter("https_lb_rule", "rule-x", "loadbalancing.googleapis.com/https/total_latencies")
	assert.Contains(t, f, "https/total_latencies")
}

func TestGetLBBackendLatenciesFilter(t *testing.T) {
	f := lbFilter("https_lb_rule", "rule-x", "loadbalancing.googleapis.com/https/backend_latencies")
	assert.Contains(t, f, "https/backend_latencies")
}

func TestGetLBThroughputFilters(t *testing.T) {
	reqF := lbFilter("https_lb_rule", "rule-x", "loadbalancing.googleapis.com/https/request_bytes_count")
	respF := lbFilter("https_lb_rule", "rule-x", "loadbalancing.googleapis.com/https/response_bytes_count")
	assert.Contains(t, reqF, "https/request_bytes_count")
	assert.Contains(t, respF, "https/response_bytes_count")
}
