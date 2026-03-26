package views

import (
	"strings"
	"testing"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSubnetDetailsView(t *testing.T) {
	v := NewSubnetDetailsView("test-project", "us-central1", "my-subnet", nil)

	assert.NotNil(t, v)
	assert.Equal(t, "test-project", v.projectID)
	assert.Equal(t, "us-central1", v.region)
	assert.Equal(t, "my-subnet", v.subnetName)
	assert.True(t, v.loading, "View should start in loading state")
	assert.Nil(t, v.details, "Details should start nil")
	assert.NotNil(t, v.networkLink, "Network link should be initialized")
	assert.False(t, v.menuOpen, "Menu should start closed")
	assert.Nil(t, v.GetComputeClient(), "Compute client should be nil when not set")
	assert.Equal(t, "my-subnet", v.GetSubnetName())
	assert.False(t, v.IsMenuOpen())
	assert.False(t, v.HasTextInputFocused())
}

func TestFormatFlowLogSampling(t *testing.T) {
	tests := []struct {
		rate     float64
		expected string
	}{
		{0.5, "50%"},
		{1.0, "100%"},
		{0.0, "0%"},
		{0.25, "25%"},
		{0.1, "10%"},
		{0.75, "75%"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatFlowLogSampling(tt.rate))
		})
	}
}

func TestSubnetDetailsView_RenderSecondaryRanges(t *testing.T) {
	v := NewSubnetDetailsView("test-project", "us-central1", "my-subnet", nil)
	v.details = &gcp.SubnetDetails{
		Name:        "my-subnet",
		Region:      "us-central1",
		Network:     "default",
		IPCidrRange: "10.128.0.0/20",
		SecondaryIPRanges: []gcp.SecondaryRange{
			{Name: "pods", CidrRange: "10.4.0.0/14"},
			{Name: "services", CidrRange: "10.0.32.0/20"},
		},
	}

	// Set up viewport so renderContent works
	v.width = 80
	v.height = 40
	v.applySize(v.width, v.height)

	content := v.renderContent()
	require.NotEmpty(t, content)

	assert.True(t, strings.Contains(content, "pods"), "Should contain 'pods' range name")
	assert.True(t, strings.Contains(content, "10.4.0.0/14"), "Should contain pods CIDR")
	assert.True(t, strings.Contains(content, "services"), "Should contain 'services' range name")
	assert.True(t, strings.Contains(content, "10.0.32.0/20"), "Should contain services CIDR")
}

func TestSubnetDetailsView_RenderSecondaryRanges_Empty(t *testing.T) {
	v := NewSubnetDetailsView("test-project", "us-central1", "my-subnet", nil)
	v.details = &gcp.SubnetDetails{
		Name:              "my-subnet",
		Region:            "us-central1",
		Network:           "default",
		IPCidrRange:       "10.128.0.0/20",
		SecondaryIPRanges: nil,
	}

	v.width = 80
	v.height = 40
	v.applySize(v.width, v.height)

	content := v.renderContent()
	require.NotEmpty(t, content)

	assert.True(t, strings.Contains(content, "No secondary IP ranges configured"),
		"Should show 'No secondary IP ranges configured' when empty")
}
