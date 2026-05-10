package gcp

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	tt, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return tt
}

func TestEstimateLifecycleAction_AgeBasedDelete(t *testing.T) {
	now := mustParse(t, "2026-05-10T12:00:00Z")
	meta := &ObjectMetadata{
		Name:         "logs/foo.log",
		Created:      mustParse(t, "2026-04-01T00:00:00Z"),
		StorageClass: "STANDARD",
	}
	rules := []LifecycleRule{
		{
			Action:    LifecycleAction{Type: "Delete"},
			Condition: LifecycleCondition{AgeInDays: 60, AgeApplies: true},
		},
	}

	got := EstimateLifecycleAction(rules, meta, now)
	if assert.NotNil(t, got) {
		assert.Equal(t, "Delete", got.Action.Type)
		assert.Equal(t, mustParse(t, "2026-05-31T00:00:00Z"), got.EffectiveAt)
		assert.Contains(t, got.Reason, "age ≥ 60d")
	}
}

func TestEstimateLifecycleAction_AgeAlreadyPastReportsImmediate(t *testing.T) {
	now := mustParse(t, "2026-05-10T12:00:00Z")
	meta := &ObjectMetadata{
		Created:      mustParse(t, "2025-01-01T00:00:00Z"),
		StorageClass: "STANDARD",
	}
	rules := []LifecycleRule{{
		Action:    LifecycleAction{Type: "Delete"},
		Condition: LifecycleCondition{AgeInDays: 30, AgeApplies: true},
	}}

	got := EstimateLifecycleAction(rules, meta, now)
	if assert.NotNil(t, got) {
		assert.True(t, got.EffectiveAt.IsZero(), "expected zero (now), got %v", got.EffectiveAt)
	}
}

func TestEstimateLifecycleAction_PrefersEarliest(t *testing.T) {
	now := mustParse(t, "2026-05-10T00:00:00Z")
	meta := &ObjectMetadata{
		Created:      mustParse(t, "2026-05-01T00:00:00Z"),
		StorageClass: "STANDARD",
	}
	rules := []LifecycleRule{
		{
			Action:    LifecycleAction{Type: "SetStorageClass", StorageClass: "NEARLINE"},
			Condition: LifecycleCondition{AgeInDays: 30, AgeApplies: true},
		},
		{
			Action:    LifecycleAction{Type: "SetStorageClass", StorageClass: "COLDLINE"},
			Condition: LifecycleCondition{AgeInDays: 60, AgeApplies: true},
		},
		{
			Action:    LifecycleAction{Type: "Delete"},
			Condition: LifecycleCondition{AgeInDays: 365, AgeApplies: true},
		},
	}

	got := EstimateLifecycleAction(rules, meta, now)
	if assert.NotNil(t, got) {
		assert.Equal(t, "SetStorageClass", got.Action.Type)
		assert.Equal(t, "NEARLINE", got.Action.StorageClass)
		assert.Equal(t, mustParse(t, "2026-05-31T00:00:00Z"), got.EffectiveAt)
	}
}

func TestEstimateLifecycleAction_StorageClassFilter(t *testing.T) {
	now := mustParse(t, "2026-05-10T00:00:00Z")
	meta := &ObjectMetadata{
		Created:      mustParse(t, "2026-04-01T00:00:00Z"),
		StorageClass: "STANDARD",
	}
	// Rule applies only to NEARLINE — should not match a STANDARD object.
	rules := []LifecycleRule{{
		Action:    LifecycleAction{Type: "Delete"},
		Condition: LifecycleCondition{AgeInDays: 1, AgeApplies: true, MatchesStorageClasses: []string{"NEARLINE"}},
	}}

	assert.Nil(t, EstimateLifecycleAction(rules, meta, now))
}

func TestEstimateLifecycleAction_PrefixSuffix(t *testing.T) {
	now := mustParse(t, "2026-05-10T00:00:00Z")
	meta := &ObjectMetadata{
		Name:         "tmp/build.log",
		Created:      mustParse(t, "2026-05-09T00:00:00Z"),
		StorageClass: "STANDARD",
	}

	matchPrefix := []LifecycleRule{{
		Action:    LifecycleAction{Type: "Delete"},
		Condition: LifecycleCondition{AgeInDays: 1, AgeApplies: true, MatchesPrefix: []string{"tmp/"}},
	}}
	assert.NotNil(t, EstimateLifecycleAction(matchPrefix, meta, now))

	mismatchPrefix := []LifecycleRule{{
		Action:    LifecycleAction{Type: "Delete"},
		Condition: LifecycleCondition{AgeInDays: 1, AgeApplies: true, MatchesPrefix: []string{"keep/"}},
	}}
	assert.Nil(t, EstimateLifecycleAction(mismatchPrefix, meta, now))

	matchSuffix := []LifecycleRule{{
		Action:    LifecycleAction{Type: "Delete"},
		Condition: LifecycleCondition{AgeInDays: 1, AgeApplies: true, MatchesSuffix: []string{".log"}},
	}}
	assert.NotNil(t, EstimateLifecycleAction(matchSuffix, meta, now))
}

func TestEstimateLifecycleAction_CustomTimeConditions(t *testing.T) {
	now := mustParse(t, "2026-05-10T00:00:00Z")

	// DaysSinceCustomTime: object expires 60d after CustomTime.
	withCustomTime := &ObjectMetadata{
		Name:         "obj",
		Created:      mustParse(t, "2026-01-01T00:00:00Z"),
		CustomTime:   mustParse(t, "2026-04-01T00:00:00Z"),
		StorageClass: "STANDARD",
	}
	rules := []LifecycleRule{{
		Action:    LifecycleAction{Type: "Delete"},
		Condition: LifecycleCondition{DaysSinceCustomTime: 60, DaysSinceCustomTimeUsed: true},
	}}
	got := EstimateLifecycleAction(rules, withCustomTime, now)
	if assert.NotNil(t, got) {
		assert.Equal(t, mustParse(t, "2026-05-31T00:00:00Z"), got.EffectiveAt)
	}

	// Same rule, no CustomTime set — should not match.
	noCustomTime := &ObjectMetadata{Name: "obj", Created: withCustomTime.Created, StorageClass: "STANDARD"}
	assert.Nil(t, EstimateLifecycleAction(rules, noCustomTime, now))
}

func TestEstimateLifecycleAction_CreatedBefore(t *testing.T) {
	now := mustParse(t, "2026-05-10T00:00:00Z")
	cutoff := mustParse(t, "2026-01-01T00:00:00Z")

	older := &ObjectMetadata{Created: mustParse(t, "2025-12-01T00:00:00Z"), StorageClass: "STANDARD"}
	newer := &ObjectMetadata{Created: mustParse(t, "2026-02-01T00:00:00Z"), StorageClass: "STANDARD"}

	rules := []LifecycleRule{{
		Action:    LifecycleAction{Type: "Delete"},
		Condition: LifecycleCondition{CreatedBefore: cutoff},
	}}

	assert.NotNil(t, EstimateLifecycleAction(rules, older, now))
	assert.Nil(t, EstimateLifecycleAction(rules, newer, now))
}

func TestEstimateLifecycleAction_ArchivedLivenessSkipped(t *testing.T) {
	now := mustParse(t, "2026-05-10T00:00:00Z")
	meta := &ObjectMetadata{Name: "x", Created: now.AddDate(0, 0, -10), StorageClass: "STANDARD"}
	rules := []LifecycleRule{{
		Action:    LifecycleAction{Type: "Delete"},
		Condition: LifecycleCondition{AgeInDays: 1, AgeApplies: true, Liveness: "archived"},
	}}
	assert.Nil(t, EstimateLifecycleAction(rules, meta, now))
}

func TestEstimateLifecycleAction_NoRulesOrNoMatch(t *testing.T) {
	now := time.Now()
	meta := &ObjectMetadata{Name: "x", StorageClass: "STANDARD"}
	assert.Nil(t, EstimateLifecycleAction(nil, meta, now))
	assert.Nil(t, EstimateLifecycleAction([]LifecycleRule{{
		Action:    LifecycleAction{Type: "Delete"},
		Condition: LifecycleCondition{MatchesPrefix: []string{"never/"}},
	}}, meta, now))
}

func TestCloneRules_DefendsAgainstCallerMutation(t *testing.T) {
	original := []LifecycleRule{{
		Action: LifecycleAction{Type: "Delete"},
		Condition: LifecycleCondition{
			MatchesPrefix:         []string{"logs/"},
			MatchesStorageClasses: []string{"STANDARD"},
		},
	}}
	clone := cloneRules(original)

	clone[0].Condition.MatchesPrefix[0] = "MUTATED"
	clone[0].Condition.MatchesPrefix = append(clone[0].Condition.MatchesPrefix, "extra/")
	clone[0].Action.Type = "MUTATED"

	assert.Equal(t, "logs/", original[0].Condition.MatchesPrefix[0], "original prefix mutated")
	assert.Len(t, original[0].Condition.MatchesPrefix, 1, "original prefix slice grew")
	assert.Equal(t, "Delete", original[0].Action.Type, "original action mutated")
}

func TestStorageClient_LifecycleCacheTTL(t *testing.T) {
	// We can't easily invoke GetBucketLifecycle without a real GCS client, so
	// we exercise the TTL/eviction logic by manipulating the cache directly.
	current := mustParse(t, "2026-05-10T00:00:00Z")
	var clock atomic.Int64
	clock.Store(current.UnixNano())
	c := &StorageClient{
		now: func() time.Time { return time.Unix(0, clock.Load()) },
	}

	// Seed cache.
	c.lifecycleCache.Store("b1", bucketLifecycleCacheEntry{
		rules:     []LifecycleRule{{Action: LifecycleAction{Type: "Delete"}}},
		fetchedAt: current,
	})

	// Within TTL: cache hit.
	v, ok := c.lifecycleCache.Load("b1")
	assert.True(t, ok)
	entry, ok := v.(bucketLifecycleCacheEntry)
	assert.True(t, ok, "cache value type")
	assert.Less(t, c.nowFn().Sub(entry.fetchedAt), bucketLifecycleTTL)

	// Advance past TTL.
	clock.Store(current.Add(bucketLifecycleTTL + time.Second).UnixNano())
	v, _ = c.lifecycleCache.Load("b1")
	entry, ok = v.(bucketLifecycleCacheEntry)
	assert.True(t, ok, "cache value type after TTL")
	assert.GreaterOrEqual(t, c.nowFn().Sub(entry.fetchedAt), bucketLifecycleTTL)

	// Invalidation removes the entry.
	c.InvalidateBucketLifecycle("b1")
	_, ok = c.lifecycleCache.Load("b1")
	assert.False(t, ok)
}
