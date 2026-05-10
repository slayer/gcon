# Object Lifecycle & TTL

## Goal

Show TTL/expiration and retention properties for GCS objects in the Object Details view.

GCS does not have per-object TTLs; expiration comes from a combination of:
- Bucket **lifecycle rules** (age, storage class, custom time, etc.) → estimated action date
- Per-object **retention** (`Retention.Mode`, `Retention.RetainUntil`) → hard deletion floor
- Bucket retention policy → `RetentionExpirationTime` per object
- Holds (`EventBasedHold`, `TemporaryHold`) → block deletion entirely

## Plan

### Phase 1 — GCP layer
- Extend `ObjectMetadata` with retention/hold/lifecycle fields from `storage.ObjectAttrs`
- Add `BucketLifecycleRule` / `LifecycleAction` / `LifecycleCondition` types
- Add `GetBucketLifecycle(ctx, bucket)` on `StorageClient` with `sync.Map` cache + 5min TTL
- Add `InvalidateBucketLifecycle(bucket)` for mutations
- Add `EstimateLifecycleAction(rules, *ObjectMetadata)` pure function
  - Walks rules, finds matching ones, picks earliest applicable Delete or SetStorageClass
  - Returns `*EstimatedLifecycleAction` (nil if no rule applies)
- Add `GetObjectMetadataAndLifecycle` convenience method that runs both calls and populates the estimate

### Phase 2 — UI
- Object Details: add "Lifecycle & Retention" section showing:
  - Estimated next action (Delete / SetStorageClass / etc.) with computed date and human delta
  - Per-object retention (mode + retain-until) when present
  - Bucket retention expiration when present
  - Holds (event-based / temporary) when active
  - Custom time when set
  - KMS key + component count (under Technical Details)

### Phase 3 — Tests
- Table-driven tests for `EstimateLifecycleAction` covering:
  - Age-based delete
  - Custom-time-based delete
  - Storage-class transition
  - Multiple rules → earliest applicable wins
  - Hold blocks the estimate
- Cache TTL test using injectable clock

## Out of scope
- Editing lifecycle rules (read-only)
- Editing per-object retention (read-only)
- Soft-deleted object listing (separate feature)
