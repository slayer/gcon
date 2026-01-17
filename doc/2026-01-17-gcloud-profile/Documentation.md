# Display GCloud Config Profile - Documentation

## Task ID
2026-01-17-gcloud-profile

## Summary
Implemented display of the active gcloud configuration profile name in the footer center slot. This helps users identify which gcloud profile they're currently using (e.g., "default", "production", "staging").

## Changes Made

### 1. Fixed `getActiveConfig()` to Read Correct File
**File:** `internal/config/gcloud.go`

**Bug Found:** The original implementation was reading from a `properties` file that doesn't exist in modern gcloud installations. Gcloud actually stores the active configuration name in a file called `active_config`.

**Fix:** Updated `getActiveConfig()` to read from the correct `active_config` file:

```go
func getActiveConfig(configDir string) (string, error) {
	activeConfigPath := filepath.Join(configDir, "active_config")

	// Read the active_config file (contains just the config name)
	data, err := os.ReadFile(activeConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	// The file contains just the config name, possibly with trailing newline
	configName := strings.TrimSpace(string(data))
	return configName, nil
}
```

**Rationale:**
- Gcloud stores active configuration in `~/.config/gcloud/active_config`
- This file contains just the configuration name (e.g., "staging\n")
- Previous implementation looked for `[core]` section in `properties` file which doesn't exist

### 2. Completed `ResolveActiveConfigName()` Function
**File:** `internal/config/resolver.go`

Updated the function to perform full profile name resolution with the following priority:
1. `CLOUDSDK_ACTIVE_CONFIG_NAME` environment variable (highest priority)
2. Active configuration from gcloud properties file
3. "default" as fallback

```go
func ResolveActiveConfigName() string {
	if envConfig := os.Getenv("CLOUDSDK_ACTIVE_CONFIG_NAME"); envConfig != "" {
		return envConfig
	}

	// Fall back to reading from gcloud config
	config, err := LoadGcloudConfig()
	if err != nil || config == nil {
		return "default"
	}

	if config.ActiveConfig != "" {
		return config.ActiveConfig
	}

	return "default"
}
```

**Rationale:**
- Respects environment variable override (allows per-session profile switching)
- Falls back to actual gcloud configuration (reads from `~/.config/gcloud/properties`)
- Returns "default" as safe fallback (matches gcloud's default behavior)

### 2. Added Profile Storage to App
**File:** `internal/ui/app.go`

- Added `configProfile string` field to `App` struct (line 114)
- Initialize profile in `NewApp()` using `config.ResolveActiveConfigName()` (line 136)
- Store in app instance for use in footer rendering (line 156)

**Rationale:**
- Profile resolved once at startup (no performance impact during rendering)
- Stored alongside other identity information (authenticatedIdentity, identityType)
- Accessible throughout app lifecycle for display purposes

### 3. Reordered Footer Slots for Better Organization
**Files:** `internal/ui/app_footer.go`, `internal/ui/components/footer.go`

Reorganized footer slots for improved information hierarchy:

**New Layout:**
- **Center**: Task status (moved from Right3)
  - More prominent position for important running tasks
  - Colored backgrounds (blue=running, green=success, red=error)
  - Uses new `SetCenterStyled()` method for custom styling

- **Right1**: Configuration name (only if not "default")
  - Shows active gcloud configuration (e.g., `[staging]`, `[prod]`)
  - Hidden for "default" configuration (cleaner UI)
  - Color generated from configuration name using `colorFromString()`

- **Right2**: Authenticated identity (moved from Right1)
  - User email or service account with type icon
  - Color generated from identity using `colorFromString()`

- **Right3**: Project ID (moved from Right2)
  - Shows selected project with color-coded background
  - Color generated from project ID using `colorFromString()`

**Code Changes:**
```go
// Center: Task status (pre-rendered with custom styles)
taskStatus, taskBg := a.renderTaskStatus()
if taskStatus != "" {
	a.footer.SetCenterStyled(taskStatus, taskBg)
} else {
	a.footer.ClearCenter()
}

// Right1: GCloud configuration (only if not "default")
if a.configProfile != "" && a.configProfile != "default" {
	bg := colorFromString(a.configProfile)
	configStyle := lipgloss.NewStyle().
		Background(bg).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 1)
	a.footer.SetRight1Styled(configStyle.Render(fmt.Sprintf("[%s]", a.configProfile)), bg)
} else {
	a.footer.ClearRight1Styled()
}

// Right2: Authenticated identity with color from identity string
bgColor := colorFromString(a.authenticatedIdentity)
identityStyle := lipgloss.NewStyle().
	Background(bgColor).
	Foreground(lipgloss.Color("#FFFFFF")).
	Padding(0, 1)

// Right3: Project info with color from project ID
bg := colorFromString(a.selectedProject.ID)
projectStyle := lipgloss.NewStyle().
	Background(bg).
	Foreground(lipgloss.Color("#FFFFFF")).
	Padding(0, 1)
```

**Added to Footer Component:**
- `SetCenterStyled()` method for pre-rendered center content with custom styling
- `CenterRendered` and `CenterRenderedBg` fields
- Updated `renderCenterGroup()` to handle styled content

**Rationale:**
- Task status deserves prominent center position (most dynamic information)
- Configuration → Identity → Project follows natural information flow
- Right-to-left reading: config context → who you are → which project

### 4. Updated All Related Tests
**Files:** `internal/config/resolver_test.go`, `internal/config/gcloud_test.go`, `internal/config/identity_test.go`, `internal/ui/app_test.go`

Updated all tests to use the correct `active_config` file instead of `properties` file:

#### Config Resolver Tests:
- Environment variable takes precedence
- Reads from `active_config` file when env var not set
- Defaults to "default" when no config found
- Defaults to "default" when config dir exists but no `active_config` file
- Environment variable overrides `active_config` file

#### Gcloud Config Tests:
- Loads config from active configuration using `active_config` file
- Environment variable overrides `active_config` file
- Falls back to default config when no active config specified

#### UI App Tests:
- Stores profile from environment variable
- Defaults to "default" when no config
- Stores various profile names correctly

**Test Coverage:**
- All edge cases covered (missing config, env var override, etc.)
- All tests pass successfully ✅
- No linting errors ✅

## Footer Layout

The footer has been reorganized with the following slot assignments:

- **Left Group**: Navigation hints (esc back/quit, sidebar toggle, help shortcuts)
- **Center**: Task status (running/success/error with colored background)
- **Right Group**: Configuration → Identity → Project

### With Non-Default Configuration and Task Running:
```
┌───────────────────────┬──────────────┬─────────────────────────────────┐
│esc back│[ sidebar│help│ ⠋ task...    │[staging]│identity│project-id │
└───────────────────────┴──────────────┴─────────────────────────────────┘
 LEFT GROUP              CENTER (task)   RIGHT GROUP (config→id→project)
```

### With Default Configuration (config hidden):
```
┌───────────────────────┬──────────────┬──────────────────────────┐
│esc back│[ sidebar│help│ ⠋ task...    │identity│project-id     │
└───────────────────────┴──────────────┴──────────────────────────┘
```

### No Active Task:
```
┌───────────────────────┬──────┬─────────────────────────────────┐
│esc back│[ sidebar│help│      │[staging]│identity│project-id │
└───────────────────────┴──────┴─────────────────────────────────┘
```

## Usage Examples

### 1. Using Default Profile
```bash
# No environment variable set, using default config
make run
# Footer center: empty (default not shown)
```

### 2. Using Named Profile via Environment Variable
```bash
# Set profile via env var (highest priority)
export CLOUDSDK_ACTIVE_CONFIG_NAME=production
make run
# Footer center: [production]
```

### 3. Using Named Profile via Gcloud Config
```bash
# Switch profile using gcloud command
gcloud config configurations activate staging
make run
# Footer center: [staging]
```

### 4. Environment Variable Override
```bash
# Even if gcloud config shows "staging"
export CLOUDSDK_ACTIVE_CONFIG_NAME=production
make run
# Footer center: [production] (env var takes precedence)
```

## Benefits

1. **Multi-environment Safety**
   - Visual confirmation of which profile is active
   - Prevents accidental operations in wrong environment

2. **Context Awareness**
   - Users know which configuration settings apply
   - Helpful when switching between multiple profiles

3. **Clean UI**
   - Hidden for default profile (majority use case)
   - Only visible when it matters (non-default profiles)

4. **Debugging Aid**
   - Quick profile identification without running gcloud commands
   - Visible at all times during app usage

## Technical Details

### Resolution Priority
1. `CLOUDSDK_ACTIVE_CONFIG_NAME` env var
2. Active config from `~/.config/gcloud/properties`
3. "default" fallback

### Performance Impact
- **Minimal:** Profile resolved once at startup
- No additional file I/O beyond existing config loading
- No impact on render loop performance
- Simple string comparison for display logic

### Edge Cases Handled
1. No gcloud config directory → Returns "default"
2. Malformed config file → Caught by LoadGcloudConfig(), returns "default"
3. Empty profile name → Returns "default"
4. Environment variable override → Takes precedence
5. Multiple profiles configured → Shows only active one

## Testing Instructions

### Run Tests
```bash
# Test config resolver
go test -v ./internal/config -run TestResolveActiveConfigName

# Test UI app
go test -v ./internal/ui -run TestNewApp_StoresConfigProfile

# Run all tests
make test

# Run linter
make lint
```

### Manual Verification
```bash
# Test 1: Default profile (center empty)
unset CLOUDSDK_ACTIVE_CONFIG_NAME
make run

# Test 2: Named profile via env var
export CLOUDSDK_ACTIVE_CONFIG_NAME=production
make run

# Test 3: Named profile via gcloud config
gcloud config configurations activate staging
unset CLOUDSDK_ACTIVE_CONFIG_NAME
make run

# Test 4: Verify env var precedence
gcloud config configurations activate staging
export CLOUDSDK_ACTIVE_CONFIG_NAME=production
make run  # Should show [production], not [staging]
```

## Future Enhancements (Out of Scope)

Potential improvements for future consideration:
- Color-code profiles by environment type (prod=red, staging=yellow, dev=green)
- Add profile icon/symbol (similar to identity icons)
- Click to switch profiles (if interactive features added)
- Show profile tooltip with full configuration details
- Profile warning for production environments

## Related Files

### Modified Files
- `internal/config/gcloud.go` - **Fixed:** Read active config from correct `active_config` file
- `internal/config/resolver.go` - Profile resolution logic
- `internal/ui/app.go` - Profile storage in app state
- `internal/ui/app_footer.go` - **Reordered:** Footer slot assignments (center=task, right1=config, right2=identity, right3=project)
- `internal/ui/components/footer.go` - **Added:** `SetCenterStyled()` method and styled center rendering
- `internal/config/resolver_test.go` - Resolver tests (updated for `active_config` file)
- `internal/config/gcloud_test.go` - Gcloud config tests (updated for `active_config` file)
- `internal/config/identity_test.go` - Identity tests (updated for `active_config` file)
- `internal/ui/app_test.go` - App profile storage tests

### Related Components
- `internal/config/identity.go` - Identity detection (similar pattern)

## Design Decisions

### Why Hide "Default" Profile?
- Most users use the default profile
- Showing `[default]` adds visual clutter without value
- Non-default profiles are the "special case" worth highlighting
- Matches gcloud's own behavior (doesn't emphasize "default")

**Alternative considered:** Always show profile (including "default")
- Pro: Explicit about which profile is active
- Con: Extra visual noise for majority of users
- Decision: Hide default for cleaner UI (can be easily changed)

### Why Center Slot?
- Semantically separates "context" (left), "profile" (center), "status" (right)
- Doesn't interfere with existing identity/project/task display
- Natural position for "environment" information
- Clean visual separation with powerline arrows

### Why Brackets Format?
- Clear visual indicator that this is a "label" or "tag"
- Distinguishes from email addresses (identity) and IDs (project)
- Common pattern in CLI tools (e.g., `git branch`)
- Short and unobtrusive

### Why Reorder Footer Slots?

**Original Layout:**
- Left: Navigation | Center: Config | Right: Identity → Project → Task

**New Layout:**
- Left: Navigation | Center: Task | Right: Config → Identity → Project

**Reasoning:**
1. **Task status is most dynamic** - Should be in prominent center position
   - Users need to see running/failed tasks immediately
   - Color-coded backgrounds make it highly visible
   - Center position gives it proper emphasis

2. **Right group flows naturally** - Configuration → Identity → Project
   - "What environment?" → "Who am I?" → "Which project?"
   - Natural left-to-right information hierarchy
   - Configuration provides context for identity/project

3. **Improved visual balance** - Center for dynamic, sides for static
   - Left: Controls (navigation, help)
   - Center: Status (task progress/errors)
   - Right: Context (config, identity, project)

4. **Consistency with other tools** - Many CLI tools show status in center
   - tmux: center for active window
   - vim: center for mode/status
   - Common pattern in status bars

### Why Use colorFromString() for All Right Group Items?

**Consistency Approach:**
All three right group items (Configuration, Identity, Project) now use the same `colorFromString()` function to generate their background colors.

**Reasoning:**
1. **Visual Consistency** - All items use the same color generation algorithm
   - Creates a cohesive visual experience
   - Similar saturation and luminosity across all items
   - Professional, unified appearance

2. **Deterministic Colors** - Same input always produces same color
   - Configuration "staging" always gets the same color
   - User "user@example.com" always gets the same color
   - Project "my-project" always gets the same color
   - Helps users quickly recognize their context

3. **Distinct but Harmonious** - Hash-based colors ensure uniqueness
   - Different configurations get different colors (staging ≠ prod)
   - Different identities get different colors
   - Different projects get different colors
   - But all use the same HSL color space for harmony

4. **No Hardcoded Color Logic** - Removed identity type-based coloring
   - Previously: Users got blue, Service Accounts got orange
   - Now: Each identity gets a unique color based on its name
   - More flexible and scalable approach

5. **Better Visual Differentiation** - Easier to distinguish at a glance
   - Multiple projects? Each has its own distinct color
   - Multiple configurations? Each has its own distinct color
   - Multiple identities? Each has its own distinct color
