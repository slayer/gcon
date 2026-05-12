package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeriveLoadBalancerType(t *testing.T) {
	tests := []struct {
		name   string
		target string
		scheme string
		proto  string
		want   string
	}{
		{
			name:   "external https managed",
			target: "https://www.googleapis.com/compute/v1/projects/p/global/targetHttpsProxies/x",
			scheme: "EXTERNAL_MANAGED",
			want:   "HTTPS (external)",
		},
		{
			name:   "external http managed",
			target: "https://www.googleapis.com/compute/v1/projects/p/global/targetHttpProxies/x",
			scheme: "EXTERNAL_MANAGED",
			want:   "HTTP (external)",
		},
		{
			name:   "internal https managed",
			target: "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/targetHttpsProxies/x",
			scheme: "INTERNAL_MANAGED",
			want:   "HTTPS (internal)",
		},
		{
			name:   "tcp proxy external",
			target: "https://www.googleapis.com/compute/v1/projects/p/global/targetTcpProxies/x",
			scheme: "EXTERNAL_MANAGED",
			want:   "TCP proxy (external)",
		},
		{
			name:   "ssl proxy external",
			target: "https://www.googleapis.com/compute/v1/projects/p/global/targetSslProxies/x",
			scheme: "EXTERNAL_MANAGED",
			want:   "SSL proxy (external)",
		},
		{
			name:   "network LB proxy",
			target: "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/backendServices/x",
			scheme: "EXTERNAL_MANAGED",
			proto:  "TCP",
			want:   "Network LB (proxy)",
		},
		{
			name:   "network LB passthrough",
			target: "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/backendServices/x",
			scheme: "EXTERNAL",
			proto:  "TCP",
			want:   "Network LB (passthrough)",
		},
		{
			name:   "internal passthrough network LB",
			target: "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/backendServices/x",
			scheme: "INTERNAL",
			proto:  "TCP",
			want:   "Network LB (passthrough)",
		},
		{
			name:   "legacy target pool",
			target: "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/targetPools/x",
			scheme: "EXTERNAL",
			want:   "Network LB (legacy)",
		},
		{
			name:   "unknown target kind falls back to raw segment",
			target: "https://www.googleapis.com/compute/v1/projects/p/global/targetGrpcProxies/x",
			scheme: "EXTERNAL_MANAGED",
			want:   "targetGrpcProxies",
		},
		{
			name:   "empty target",
			target: "",
			scheme: "EXTERNAL",
			want:   "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveLoadBalancerType(tc.target, tc.scheme, tc.proto)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestTargetKind(t *testing.T) {
	got := targetKind("https://www.googleapis.com/compute/v1/projects/p/global/targetHttpsProxies/my-proxy")
	assert.Equal(t, "targetHttpsProxies", got)

	got = targetKind("https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/backendServices/my-bes")
	assert.Equal(t, "backendServices", got)

	got = targetKind("")
	assert.Equal(t, "", got)
}

func TestShortName(t *testing.T) {
	assert.Equal(t, "my-proxy", shortName("https://www.googleapis.com/compute/v1/projects/p/global/targetHttpsProxies/my-proxy"))
	assert.Equal(t, "", shortName(""))
	assert.Equal(t, "x", shortName("x"))
}
