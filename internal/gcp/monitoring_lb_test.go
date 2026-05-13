package gcp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLBFilterContainsRuleAndMetric(t *testing.T) {
	f := lbFilter("my-rule", "loadbalancing.googleapis.com/https/request_count")
	assert.Contains(t, f, `forwarding_rule_name = "my-rule"`)
	assert.Contains(t, f, `metric.type = "loadbalancing.googleapis.com/https/request_count"`)
	assert.True(t, strings.Contains(f, `https_lb_rule`))
	assert.True(t, strings.Contains(f, `internal_http_lb_rule`))
}

func TestLBFilterWithLabel(t *testing.T) {
	f := lbFilterWithLabel("my-rule", "loadbalancing.googleapis.com/https/request_count", "response_code_class", "4xx")
	assert.Contains(t, f, `forwarding_rule_name = "my-rule"`)
	assert.Contains(t, f, `metric.labels.response_code_class = "4xx"`)
}

func TestGetLBRequestCountFilter(t *testing.T) {
	f := lbFilter("rule-x", "loadbalancing.googleapis.com/https/request_count")
	assert.Contains(t, f, "https/request_count")
	assert.Contains(t, f, `forwarding_rule_name = "rule-x"`)
}

func TestGetLBRequestCountByCodeClassFilter(t *testing.T) {
	f := lbFilterWithLabel("rule-x", "loadbalancing.googleapis.com/https/request_count", "response_code_class", "5xx")
	assert.Contains(t, f, `metric.labels.response_code_class = "5xx"`)
	assert.Contains(t, f, "https/request_count")
}
