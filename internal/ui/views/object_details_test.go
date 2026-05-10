package views

import (
	"strings"
	"testing"
	"time"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
)

func TestRenderDetailsTab_ShowsLifecycleSectionWhenEstimatePresent(t *testing.T) {
	v := NewObjectDetailsView("bkt", "logs/foo.log", "foo.log", nil, ObjectActionView)
	v.width = 100
	v.metadata = &gcp.ObjectMetadata{
		Name:         "logs/foo.log",
		Bucket:       "bkt",
		Created:      time.Now().Add(-30 * 24 * time.Hour),
		Updated:      time.Now().Add(-1 * time.Hour),
		StorageClass: "STANDARD",
	}
	v.lifecycleEstimate = &gcp.EstimatedLifecycleAction{
		Action:      gcp.LifecycleAction{Type: "Delete"},
		EffectiveAt: time.Now().Add(30 * 24 * time.Hour),
		Reason:      "age ≥ 60d",
	}

	out := v.renderDetailsTab()
	assert.Contains(t, out, "Lifecycle & Retention")
	assert.Contains(t, out, "Delete")
	assert.Contains(t, out, "Matches")
	// formatLifecycleEffective recomputes time.Now() inside, so the rounded
	// day count can land on either side of the boundary depending on
	// scheduling. Accept either.
	assert.True(t,
		strings.Contains(out, "in 30 days") || strings.Contains(out, "in 29 days"),
		"expected ~30-day relative formatting, got: %s", out)
}

func TestRenderDetailsTab_ShowsHoldsAndRetention(t *testing.T) {
	v := NewObjectDetailsView("bkt", "obj", "obj", nil, ObjectActionView)
	v.width = 100
	v.metadata = &gcp.ObjectMetadata{
		Name:                 "obj",
		Bucket:               "bkt",
		Created:              time.Now(),
		Updated:              time.Now(),
		StorageClass:         "STANDARD",
		EventBasedHold:       true,
		TemporaryHold:        true,
		RetentionMode:        "Locked",
		RetentionRetainUntil: time.Now().Add(7 * 24 * time.Hour),
	}

	out := v.renderDetailsTab()
	assert.Contains(t, out, "Lifecycle & Retention")
	assert.Contains(t, out, "Event-based hold")
	assert.Contains(t, out, "Temporary hold")
	assert.Contains(t, out, "deletion blocked")
	assert.Contains(t, out, "Retention (Locked)")
}

func TestRenderDetailsTab_OmitsLifecycleSectionWhenNoData(t *testing.T) {
	v := NewObjectDetailsView("bkt", "obj", "obj", nil, ObjectActionView)
	v.width = 100
	v.metadata = &gcp.ObjectMetadata{
		Name:         "obj",
		Bucket:       "bkt",
		Created:      time.Now(),
		Updated:      time.Now(),
		StorageClass: "STANDARD",
	}

	out := v.renderDetailsTab()
	assert.NotContains(t, out, "Lifecycle & Retention")
}

func TestFormatLifecycleAction(t *testing.T) {
	cases := []struct {
		in   gcp.LifecycleAction
		want string
	}{
		{gcp.LifecycleAction{Type: "Delete"}, "Delete"},
		{gcp.LifecycleAction{Type: "SetStorageClass", StorageClass: "NEARLINE"}, "Set storage class → NEARLINE"},
		{gcp.LifecycleAction{Type: "SetStorageClass"}, "Set storage class"},
		{gcp.LifecycleAction{Type: "AbortIncompleteMultipartUpload"}, "Abort incomplete multipart upload"},
		{gcp.LifecycleAction{Type: "Unknown"}, "Unknown"},
		{gcp.LifecycleAction{}, "—"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, formatLifecycleAction(tc.in))
	}
}

func TestFormatLifecycleEffective(t *testing.T) {
	assert.Equal(t, "due now", formatLifecycleEffective(time.Time{}))

	future := formatLifecycleEffective(time.Now().Add(48 * time.Hour))
	assert.True(t, strings.Contains(future, "in 2 days") || strings.Contains(future, "in 1 day"),
		"got %q", future)

	past := formatLifecycleEffective(time.Now().Add(-25 * time.Hour))
	assert.Contains(t, past, "overdue")
}
