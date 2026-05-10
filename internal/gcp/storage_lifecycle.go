package gcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/storage"
)

// bucketLifecycleTTL controls how long lifecycle rules are cached on the
// StorageClient. Rules are admin-level config that rarely change, so a
// generous TTL is fine; we keep it short enough that a console-side change
// shows up reasonably quickly.
const bucketLifecycleTTL = 5 * time.Minute

type bucketLifecycleCacheEntry struct {
	rules     []LifecycleRule
	fetchedAt time.Time
}

// GetBucketLifecycle returns the lifecycle rules for a bucket, using a
// short-lived in-memory cache. The cache key is the bucket name.
func (c *StorageClient) GetBucketLifecycle(ctx context.Context, bucketName string) ([]LifecycleRule, error) {
	now := c.nowFn()
	if v, ok := c.lifecycleCache.Load(bucketName); ok {
		if entry, ok := v.(bucketLifecycleCacheEntry); ok && now.Sub(entry.fetchedAt) < bucketLifecycleTTL {
			return entry.rules, nil
		}
	}

	attrs, err := c.client.Bucket(bucketName).Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bucket attrs: %w", err)
	}
	rules := convertLifecycle(attrs.Lifecycle)
	c.lifecycleCache.Store(bucketName, bucketLifecycleCacheEntry{rules: rules, fetchedAt: now})
	return rules, nil
}

// InvalidateBucketLifecycle removes the cached lifecycle rules for a bucket.
// Callers that mutate lifecycle rules should call this after a successful
// patch so the next read sees fresh data.
func (c *StorageClient) InvalidateBucketLifecycle(bucketName string) {
	c.lifecycleCache.Delete(bucketName)
}

// nowFn returns the client's clock, defaulting to time.Now.
func (c *StorageClient) nowFn() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func convertLifecycle(lc storage.Lifecycle) []LifecycleRule {
	if len(lc.Rules) == 0 {
		return nil
	}
	rules := make([]LifecycleRule, 0, len(lc.Rules))
	for i := range lc.Rules {
		r := &lc.Rules[i]
		rules = append(rules, LifecycleRule{
			Action: LifecycleAction{
				Type:         r.Action.Type,
				StorageClass: r.Action.StorageClass,
			},
			Condition: convertCondition(r.Condition),
		})
	}
	return rules
}

func convertCondition(c storage.LifecycleCondition) LifecycleCondition {
	out := LifecycleCondition{
		CreatedBefore:         c.CreatedBefore,
		CustomTimeBefore:      c.CustomTimeBefore,
		MatchesStorageClasses: append([]string(nil), c.MatchesStorageClasses...),
		MatchesPrefix:         append([]string(nil), c.MatchesPrefix...),
		MatchesSuffix:         append([]string(nil), c.MatchesSuffix...),
	}
	// AgeInDays: SDK uses int64 with -1 (or 0 + LiveCondition trick); the SDK
	// sets a sentinel via a struct field. Pre-1.34 SDKs expose AgeInDays as
	// int64 where 0 + AllObjects=true means "applies always". To keep our
	// model simple we treat any AgeInDays > 0 as "applies", and additionally
	// honor AllObjects.
	if c.AgeInDays > 0 {
		out.AgeInDays = c.AgeInDays
		out.AgeApplies = true
	} else if c.AllObjects {
		out.AgeInDays = 0
		out.AgeApplies = true
	}
	if c.DaysSinceCustomTime > 0 {
		out.DaysSinceCustomTime = c.DaysSinceCustomTime
		out.DaysSinceCustomTimeUsed = true
	}
	switch c.Liveness {
	case storage.Live:
		out.Liveness = "live"
	case storage.Archived:
		out.Liveness = "archived"
	default:
		out.Liveness = ""
	}
	if c.NumNewerVersions > 0 {
		out.NumNewerVersions = c.NumNewerVersions
		out.NumNewerVersionsApplies = true
	}
	return out
}

// EstimateLifecycleAction walks the supplied lifecycle rules and returns the
// soonest action expected to apply to the object. Returns nil if no rule
// matches or if the matching rules cannot be evaluated (e.g. require version
// counts we don't fetch). The function is pure and safe for tests.
//
// Holds (event-based or temporary) and per-object retention block deletion at
// the GCS layer regardless of lifecycle rules. They are not factored into
// this estimate — the UI displays them as separate signals so users see both
// the "would-trigger" date and the "actually-blocked" status.
func EstimateLifecycleAction(rules []LifecycleRule, meta *ObjectMetadata, now time.Time) *EstimatedLifecycleAction {
	if meta == nil || len(rules) == 0 {
		return nil
	}
	var best *EstimatedLifecycleAction
	for i := range rules {
		rule := rules[i]
		eff, ok, reason := evaluateRule(rule, meta, now)
		if !ok {
			continue
		}
		candidate := &EstimatedLifecycleAction{
			Action:      rule.Action,
			EffectiveAt: eff,
			Reason:      reason,
		}
		if best == nil || effectiveBefore(candidate.EffectiveAt, best.EffectiveAt) {
			best = candidate
		}
	}
	return best
}

// effectiveBefore reports whether a is "earlier" than b for ordering
// candidate actions. A zero a means "now" (run immediately) and is always
// earliest.
func effectiveBefore(a, b time.Time) bool {
	if a.IsZero() {
		return true
	}
	if b.IsZero() {
		return false
	}
	return a.Before(b)
}

// evaluateRule reports whether the given rule applies to the object and, if
// so, the time the action is expected to run plus a short reason describing
// the matching condition. Returns ok=false when the rule cannot be evaluated
// or does not match.
func evaluateRule(rule LifecycleRule, meta *ObjectMetadata, now time.Time) (effectiveAt time.Time, ok bool, reason string) {
	cond := rule.Condition

	// Storage-class filter — short-circuit if specified and the object doesn't match.
	if len(cond.MatchesStorageClasses) > 0 && !containsFold(cond.MatchesStorageClasses, effectiveStorageClass(meta.StorageClass)) {
		return time.Time{}, false, ""
	}
	// Name filters.
	if len(cond.MatchesPrefix) > 0 && !anyHasPrefix(cond.MatchesPrefix, meta.Name) {
		return time.Time{}, false, ""
	}
	if len(cond.MatchesSuffix) > 0 && !anyHasSuffix(cond.MatchesSuffix, meta.Name) {
		return time.Time{}, false, ""
	}
	// Liveness — without versioning context all current objects are "live".
	if cond.Liveness == "archived" {
		return time.Time{}, false, ""
	}
	// We don't fetch noncurrent versions, so any rule depending on counts is unevaluable.
	if cond.NumNewerVersionsApplies {
		return time.Time{}, false, ""
	}

	// Determine the earliest time at which any specified condition becomes true.
	// All specified time-based conditions must hold; we take the latest of them.
	var triggerAt time.Time
	reasons := make([]string, 0, 3)

	if cond.AgeApplies {
		eff := meta.Created.AddDate(0, 0, int(cond.AgeInDays))
		triggerAt = laterTime(triggerAt, eff)
		reasons = append(reasons, fmt.Sprintf("age ≥ %dd", cond.AgeInDays))
	}
	if !cond.CreatedBefore.IsZero() {
		// Matches if Created < CreatedBefore. If already true, no future
		// trigger date — the rule applies from "now". Otherwise the rule will
		// never apply to this object.
		if !meta.Created.Before(cond.CreatedBefore) {
			return time.Time{}, false, ""
		}
		reasons = append(reasons, "created before "+cond.CreatedBefore.Format("2006-01-02"))
	}
	if !cond.CustomTimeBefore.IsZero() {
		if meta.CustomTime.IsZero() || !meta.CustomTime.Before(cond.CustomTimeBefore) {
			return time.Time{}, false, ""
		}
		reasons = append(reasons, "customTime before "+cond.CustomTimeBefore.Format("2006-01-02"))
	}
	if cond.DaysSinceCustomTimeUsed {
		if meta.CustomTime.IsZero() {
			return time.Time{}, false, ""
		}
		eff := meta.CustomTime.AddDate(0, 0, int(cond.DaysSinceCustomTime))
		triggerAt = laterTime(triggerAt, eff)
		reasons = append(reasons, fmt.Sprintf("daysSinceCustomTime ≥ %d", cond.DaysSinceCustomTime))
	}

	// If no time-based condition was specified at all, the rule applies "now".
	if len(reasons) == 0 {
		// Storage-class/prefix/suffix-only rules with no time component apply
		// continuously. Surface them as immediate.
		reasons = append(reasons, "matches always")
	}

	// If the trigger date is already past, the action is effectively "now".
	if !triggerAt.IsZero() && triggerAt.Before(now) {
		triggerAt = time.Time{}
	}

	return triggerAt, true, strings.Join(reasons, ", ")
}

// effectiveStorageClass returns a normalized storage class for matching.
// GCS reports an empty string for the bucket default in some cases; we treat
// it as the matcher would (no class filter).
func effectiveStorageClass(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s)
}

func containsFold(haystack []string, needle string) bool {
	for _, h := range haystack {
		if strings.EqualFold(h, needle) {
			return true
		}
	}
	return false
}

func anyHasPrefix(prefixes []string, s string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func anyHasSuffix(suffixes []string, s string) bool {
	for _, sfx := range suffixes {
		if strings.HasSuffix(s, sfx) {
			return true
		}
	}
	return false
}

func laterTime(a, b time.Time) time.Time {
	if a.IsZero() {
		return b
	}
	if b.IsZero() {
		return a
	}
	if b.After(a) {
		return b
	}
	return a
}

// GetObjectMetadataAndLifecycle fetches object metadata and bucket lifecycle
// rules (using the cache), then attaches the estimated next lifecycle action.
// Lifecycle fetch errors are swallowed — they shouldn't prevent showing the
// object's own metadata. The estimate is reported separately.
func (c *StorageClient) GetObjectMetadataAndLifecycle(ctx context.Context, bucketName, objectName string) (*ObjectMetadata, *EstimatedLifecycleAction, error) {
	meta, err := c.GetObjectMetadata(ctx, bucketName, objectName)
	if err != nil {
		return nil, nil, err
	}
	rules, err := c.GetBucketLifecycle(ctx, bucketName)
	if err != nil {
		// Non-fatal: lifecycle isn't always readable (permissions) and we
		// shouldn't fail the whole details view because of it. The estimate
		// is omitted but the object metadata is still returned.
		return meta, nil, nil //nolint:nilerr // intentional: lifecycle is best-effort
	}
	est := EstimateLifecycleAction(rules, meta, c.nowFn())
	return meta, est, nil
}
