# TODO: Display GCloud Config Profile

## Task ID
2026-01-17-gcloud-profile

## Objective
Display the active gcloud configuration profile name in the footer center slot.

## Implementation Steps

### Phase 1: Complete ResolveActiveConfigName()
- [x] Update `internal/config/resolver.go` to fully resolve profile name
  - Check env var first (CLOUDSDK_ACTIVE_CONFIG_NAME)
  - Fall back to LoadGcloudConfig()
  - Return "default" as final fallback

### Phase 2: Store Profile in App
- [x] Add `configProfile` field to App struct
- [x] Initialize in NewApp() using ResolveActiveConfigName()

### Phase 3: Display in Footer
- [x] Update `syncFooter()` in `internal/ui/app_footer.go`
- [x] Display profile in center slot with `[profile]` format
- [x] Hide "default" profile for cleaner UI

### Phase 4: Testing
- [x] Add tests for ResolveActiveConfigName()
- [x] Add test for NewApp() profile storage
- [x] Run full test suite
- [x] Run linter
- [x] Manual verification with different profiles

## Files Modified
- `internal/config/resolver.go` - Completed ResolveActiveConfigName() function
- `internal/ui/app.go` - Added configProfile field and initialization
- `internal/ui/app_footer.go` - Display profile in center slot
- `internal/config/resolver_test.go` - Added comprehensive tests
- `internal/ui/app_test.go` - Added profile storage tests

## Verification
- [x] Default profile: center slot is empty
- [x] Named profile via env var: shows [profile-name]
- [x] Named profile via gcloud config: shows [profile-name]
- [x] All tests pass
- [x] No linting errors

## Bug Fix Applied ⚠️

**Issue Found:** The implementation was reading from a non-existent `properties` file. Gcloud actually stores the active configuration in `~/.config/gcloud/active_config`.

**Fix Applied:**
- ✓ Updated `getActiveConfig()` in `internal/config/gcloud.go` to read from `active_config` file
- ✓ Updated all tests to use `active_config` file instead of `properties` file
- ✓ All tests passing with the fix
- ✓ No linting errors

## Implementation Complete ✓

All tasks have been successfully completed:
- ✓ ResolveActiveConfigName() fully implemented with priority resolution
- ✓ Configuration name stored in App struct
- ✓ Footer center slot displays profile (hiding "default" for cleaner UI)
- ✓ Comprehensive tests added and passing
- ✓ **Bug fixed:** Reads from correct `active_config` file
- ✓ No linting errors
- ✓ Manual verification completed with multiple profiles
