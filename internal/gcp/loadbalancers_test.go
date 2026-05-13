package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	compute "google.golang.org/api/compute/v1"
)

func TestDeriveLoadBalancerType(t *testing.T) {
	tests := []struct {
		name   string
		target string
		scheme string
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
			want:   "Network LB (proxy)",
		},
		{
			name:   "network LB passthrough",
			target: "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/backendServices/x",
			scheme: "EXTERNAL",
			want:   "Network LB (passthrough)",
		},
		{
			name:   "internal passthrough network LB",
			target: "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/backendServices/x",
			scheme: "INTERNAL",
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
			got := DeriveLoadBalancerType(tc.target, tc.scheme)
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

func TestConvertNEG(t *testing.T) {
	in := &compute.NetworkEndpointGroup{
		Name:                "my-neg",
		SelfLink:            "https://www.googleapis.com/compute/v1/projects/p/zones/us-central1-a/networkEndpointGroups/my-neg",
		NetworkEndpointType: "GCE_VM_IP_PORT",
	}
	got := convertNEG(in, "us-central1-a")
	assert.Equal(t, "my-neg", got.Name)
	assert.Equal(t, "us-central1-a", got.Zone)
	assert.Equal(t, "GCE_VM_IP_PORT", got.NetworkEndpointType)
}

func TestConvertNEGServerless(t *testing.T) {
	in := &compute.NetworkEndpointGroup{
		Name:                "cr-neg",
		NetworkEndpointType: "SERVERLESS",
	}
	got := convertNEG(in, "us-central1")
	assert.Equal(t, "SERVERLESS", got.NetworkEndpointType)
}

func TestConvertNEGGlobalEmptyZone(t *testing.T) {
	in := &compute.NetworkEndpointGroup{
		Name:                "global-neg",
		NetworkEndpointType: "INTERNET_FQDN_PORT",
	}
	got := convertNEG(in, "")
	assert.Empty(t, got.Zone)
}

func TestConvertHealthStatusesNil(t *testing.T) {
	got := convertHealthStatuses(nil)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestConvertHealthStatuses(t *testing.T) {
	in := []*compute.HealthStatus{
		{Instance: "https://www.googleapis.com/compute/v1/projects/p/zones/us-central1-a/instances/vm-1", IpAddress: "10.0.0.5", Port: 80, HealthState: "HEALTHY"},
		{Instance: "https://www.googleapis.com/compute/v1/projects/p/zones/us-central1-a/instances/vm-2", IpAddress: "10.0.0.6", Port: 80, HealthState: "UNHEALTHY"},
	}
	got := convertHealthStatuses(in)
	require.Len(t, got, 2)
	assert.Equal(t, "vm-1", got[0].Instance)
	assert.Equal(t, "HEALTHY", got[0].HealthState)
	assert.Empty(t, got[0].FailureReason)
	assert.Equal(t, "vm-2", got[1].Instance)
	assert.Equal(t, "UNHEALTHY", got[1].HealthState)
	assert.Equal(t, "UNHEALTHY", got[1].FailureReason)
}
