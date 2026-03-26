package views

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogsViewNew(t *testing.T) {
	v := NewLogsView("test-project", nil)

	assert.Equal(t, "test-project", v.projectID)
	assert.Equal(t, logsStateIdle, v.state)
	assert.Equal(t, time.Hour, v.timeRange)
	assert.Equal(t, logsFocusEntries, v.focus)
	assert.False(t, v.queryFocused)
	assert.False(t, v.tailMode)
	assert.Empty(t, v.query)
	assert.Empty(t, v.selectedResources)
	assert.Empty(t, v.selectedLogNames)
	assert.Empty(t, v.selectedSeverities)
	assert.NotNil(t, v.logViewer)
	assert.NotNil(t, v.filterSelected)
	assert.Equal(t, filterDropdownNone, v.activeFilter)
}

func TestLogsViewWithFilters(t *testing.T) {
	v := NewLogsView("test-project", nil)

	query := `resource.type="cloud_run_revision"` + "\n" + `resource.labels.service_name="my-svc"`
	severities := []string{"WARNING", "ERROR"}
	v.WithFilters(query, severities, 6*time.Hour)

	assert.Equal(t, query, v.query)
	// textinput is single-line, so newlines become spaces in the input display
	assert.Contains(t, v.queryInput.Value(), `resource.type="cloud_run_revision"`)
	assert.Equal(t, severities, v.selectedSeverities)
	assert.Equal(t, 6*time.Hour, v.timeRange)
}

func TestLogsViewWithFilters_NormalizeSeverityOrder(t *testing.T) {
	// Severities provided in reverse / non-deterministic order (e.g., from a map)
	// must be stored in the canonical allSeverities order so that slicesEqual()
	// comparisons in applyFilterDropdown() remain deterministic and don't
	// trigger spurious re-queries.
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "reverse order normalized",
			input:    []string{"ERROR", "WARNING"},
			expected: []string{"WARNING", "ERROR"},
		},
		{
			name:     "duplicates removed and ordered",
			input:    []string{"ERROR", "WARNING", "ERROR"},
			expected: []string{"WARNING", "ERROR"},
		},
		{
			name:     "already ordered is unchanged",
			input:    []string{"WARNING", "ERROR"},
			expected: []string{"WARNING", "ERROR"},
		},
		{
			name:     "all severities kept in canonical order",
			input:    []string{"EMERGENCY", "DEBUG", "CRITICAL", "INFO"},
			expected: []string{"DEBUG", "INFO", "CRITICAL", "EMERGENCY"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := NewLogsView("test-project", nil)
			v.WithFilters("", tc.input, 0)
			assert.Equal(t, tc.expected, v.selectedSeverities)
		})
	}
}

func TestLogsViewWithFilters_Defaults(t *testing.T) {
	v := NewLogsView("test-project", nil)

	// Empty values should not override defaults
	v.WithFilters("", nil, 0)

	assert.Empty(t, v.query)
	assert.Empty(t, v.selectedSeverities)
	assert.Equal(t, time.Hour, v.timeRange)
}

func TestLogsViewHasTextInputFocused(t *testing.T) {
	v := NewLogsView("test-project", nil)

	// Not focused by default
	assert.False(t, v.HasTextInputFocused())

	// Focused after setting query focus
	v.queryFocused = true
	assert.True(t, v.HasTextInputFocused())

	// Back to false
	v.queryFocused = false
	assert.False(t, v.HasTextInputFocused())
}

func TestLogsViewBuildEffectiveQuery(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(v *LogsView)
		wantParts []string // substrings that must appear
		dontWant  []string // substrings that must not appear
	}{
		{
			name:  "default has only timestamp",
			setup: func(_ *LogsView) {},
			wantParts: []string{
				`timestamp >= "`,
			},
			dontWant: []string{
				"resource.type",
				"logName",
				"severity",
			},
		},
		{
			name: "single resource filter",
			setup: func(v *LogsView) {
				v.selectedResources = []string{"gce_instance"}
			},
			wantParts: []string{
				`resource.type = "gce_instance"`,
			},
		},
		{
			name: "multiple resource filters use OR",
			setup: func(v *LogsView) {
				v.selectedResources = []string{"gce_instance", "cloud_run_revision"}
			},
			wantParts: []string{
				`resource.type = "gce_instance"`,
				`resource.type = "cloud_run_revision"`,
				" OR ",
			},
		},
		{
			name: "single severity filter",
			setup: func(v *LogsView) {
				v.selectedSeverities = []string{"ERROR"}
			},
			wantParts: []string{
				`severity = "ERROR"`,
			},
		},
		{
			name: "multiple severities use OR",
			setup: func(v *LogsView) {
				v.selectedSeverities = []string{"ERROR", "WARNING"}
			},
			wantParts: []string{
				`severity = "ERROR"`,
				`severity = "WARNING"`,
				" OR ",
			},
		},
		{
			name: "log name filter",
			setup: func(v *LogsView) {
				v.selectedLogNames = []string{"projects/test/logs/stderr"}
			},
			wantParts: []string{
				`logName = "projects/test/logs/stderr"`,
			},
		},
		{
			name: "user query appended",
			setup: func(v *LogsView) {
				v.query = `textPayload : "error connecting"`
			},
			wantParts: []string{
				`timestamp >= "`,
				`textPayload : "error connecting"`,
			},
		},
		{
			name: "all filters combined",
			setup: func(v *LogsView) {
				v.selectedResources = []string{"gce_instance"}
				v.selectedLogNames = []string{"projects/test/logs/syslog"}
				v.selectedSeverities = []string{"ERROR"}
				v.query = `textPayload : "disk full"`
			},
			wantParts: []string{
				`timestamp >= "`,
				`resource.type = "gce_instance"`,
				`logName = "projects/test/logs/syslog"`,
				`severity = "ERROR"`,
				`textPayload : "disk full"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewLogsView("test-project", nil)
			tt.setup(v)

			query := v.buildEffectiveQuery()

			for _, part := range tt.wantParts {
				assert.Contains(t, query, part, "query should contain %q", part)
			}
			for _, part := range tt.dontWant {
				assert.NotContains(t, query, part, "query should not contain %q", part)
			}
		})
	}
}

func TestLogsViewFilterDropdown(t *testing.T) {
	t.Run("open severity filter populates options", func(t *testing.T) {
		v := NewLogsView("test-project", nil)
		v.selectedSeverities = []string{"ERROR", "WARNING"}

		v.openFilterDropdown(filterDropdownSeverities)

		assert.Equal(t, filterDropdownSeverities, v.activeFilter)
		assert.Equal(t, allSeverities, v.filterOptions)
		assert.True(t, v.filterSelected["ERROR"])
		assert.True(t, v.filterSelected["WARNING"])
		assert.False(t, v.filterSelected["INFO"])
		assert.Equal(t, 0, v.filterCursor)
	})

	t.Run("open resource filter populates options", func(t *testing.T) {
		v := NewLogsView("test-project", nil)
		v.availableResources = []string{"gce_instance", "cloud_run_revision", "gcs_bucket"}
		v.selectedResources = []string{"gce_instance"}

		v.openFilterDropdown(filterDropdownResources)

		assert.Equal(t, filterDropdownResources, v.activeFilter)
		assert.Equal(t, v.availableResources, v.filterOptions)
		assert.True(t, v.filterSelected["gce_instance"])
		assert.False(t, v.filterSelected["cloud_run_revision"])
	})

	t.Run("apply filter dropdown saves selections", func(t *testing.T) {
		v := NewLogsView("test-project", nil)
		v.activeFilter = filterDropdownSeverities
		v.filterOptions = allSeverities
		v.filterSelected = map[string]bool{
			"ERROR":    true,
			"CRITICAL": true,
		}

		v.applyFilterDropdown()

		require.Len(t, v.selectedSeverities, 2)
		assert.Contains(t, v.selectedSeverities, "ERROR")
		assert.Contains(t, v.selectedSeverities, "CRITICAL")
	})
}

func TestLogsViewFilterDropdownFullFlow(t *testing.T) {
	v := NewLogsView("test-project", nil)
	// Simulate resource types already loaded
	v.availableResources = []string{"gce_instance", "cloud_run_revision", "gcs_bucket"}
	v.resourcesLoaded = true

	// Open resource dropdown
	v.openFilterDropdown(filterDropdownResources)
	assert.Equal(t, filterDropdownResources, v.activeFilter)
	assert.Equal(t, 3, len(v.filterOptions))

	// Navigate to cloud_run_revision (index 1) and toggle with Enter
	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, 1, v.filterCursor)
	v.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, v.filterSelected["cloud_run_revision"], "should be selected after enter")

	// Close with Esc (applies + closes)
	v.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, filterDropdownNone, v.activeFilter, "dropdown should be closed")
	require.Len(t, v.selectedResources, 1)
	assert.Equal(t, "cloud_run_revision", v.selectedResources[0])

	// Verify the query includes the resource filter
	query := v.buildEffectiveQuery()
	assert.Contains(t, query, `resource.type = "cloud_run_revision"`)
}

func TestLogsViewFilterDropdownLazyLoad(t *testing.T) {
	v := NewLogsView("test-project", nil)
	// Resources NOT loaded yet
	assert.False(t, v.resourcesLoaded)

	// Open resource dropdown — triggers lazy load, options are empty
	v.openFilterDropdown(filterDropdownResources)
	assert.Equal(t, filterDropdownResources, v.activeFilter)
	assert.Empty(t, v.filterOptions, "options should be empty before async load")

	// Simulate async response arriving while dropdown is open
	v.Update(logsResourceTypesLoadedMsg{types: []string{"gce_instance", "cloud_run_revision"}})
	assert.Equal(t, 2, len(v.filterOptions), "options should update from async load")
	assert.True(t, v.resourcesLoaded)

	// Navigate to cloud_run_revision and toggle
	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, 1, v.filterCursor)
	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	assert.True(t, v.filterSelected["cloud_run_revision"])

	// Close dropdown with Esc
	v.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, filterDropdownNone, v.activeFilter)
	require.Len(t, v.selectedResources, 1)
	assert.Equal(t, "cloud_run_revision", v.selectedResources[0])

	// Query should include the filter
	query := v.buildEffectiveQuery()
	assert.Contains(t, query, `resource.type = "cloud_run_revision"`)

	// Pill should show count
	pills := v.renderFilterPills()
	assert.Contains(t, pills, "Resource Types: 1")
}

func TestLogsViewSetContext(t *testing.T) {
	v := NewLogsView("test-project", nil)
	ctx := context.New()
	ctx.ContentWidth = 120
	ctx.ContentHeight = 40
	ctx.EmojiWidthBudget = 2 // simulate sidebar with 2 wide emojis

	v.SetContext(ctx)

	// width = ContentWidth - EmojiWidthBudget - 1 (for ▸ indicator)
	assert.Equal(t, 117, v.width)
	assert.Equal(t, 40, v.height)
}

func TestLogsViewAppendFilterToQuery(t *testing.T) {
	t.Run("empty query gets new clause", func(t *testing.T) {
		v := NewLogsView("test-project", nil)
		v.appendFilterToQuery("resource.type", "gce_instance")

		assert.Equal(t, `resource.type = "gce_instance"`, v.query)
	})

	t.Run("existing query gets appended clause", func(t *testing.T) {
		v := NewLogsView("test-project", nil)
		v.query = `severity = "ERROR"`
		v.queryInput.SetValue(v.query)

		v.appendFilterToQuery("resource.type", "gce_instance")

		assert.Contains(t, v.query, `severity = "ERROR"`)
		assert.Contains(t, v.query, `resource.type = "gce_instance"`)
	})

	t.Run("value with double quotes is escaped", func(t *testing.T) {
		v := NewLogsView("test-project", nil)
		v.appendFilterToQuery("textPayload", `error: "not found"`)

		assert.Equal(t, `textPayload = "error: \"not found\""`, v.query)
	})
}

func TestLogsViewEntriesLoaded(t *testing.T) {
	v := NewLogsView("test-project", nil)
	v.state = logsStateLoading

	entries := []gcp.LogEntry{
		{Timestamp: time.Now(), Severity: "INFO", Message: "test log"},
		{Timestamp: time.Now(), Severity: "ERROR", Message: "error log"},
	}

	v.Update(logsEntriesLoadedMsg{
		entries:   entries,
		nextToken: "next-page",
	})

	assert.Equal(t, logsStateIdle, v.state)
	assert.Nil(t, v.err)
	assert.Len(t, v.entries, 2)
	assert.Equal(t, "next-page", v.nextPageToken)
	assert.Equal(t, int64(2), v.totalCount)
	assert.Equal(t, 2, v.logViewer.EntryCount())
}

func TestLogsViewEntriesError(t *testing.T) {
	v := NewLogsView("test-project", nil)
	v.state = logsStateLoading

	v.Update(logsEntriesErrorMsg{err: assert.AnError})

	assert.Equal(t, logsStateError, v.state)
	assert.Equal(t, assert.AnError, v.err)
}

func TestLogsViewClose(t *testing.T) {
	v := NewLogsView("test-project", nil)

	// Start tail mode
	v.tailMode = true
	v.tailTicker = time.NewTicker(15 * time.Second)
	v.tailDone = make(chan struct{})

	// Close should clean up
	v.Close()

	assert.False(t, v.tailMode)
	assert.Nil(t, v.tailTicker)
	assert.Nil(t, v.tailDone)
}

func TestLogsViewRenderFilterPills(t *testing.T) {
	v := NewLogsView("test-project", nil)

	// Default: all filters show "all"
	pills := v.renderFilterPills()
	assert.Contains(t, pills, "Resource Types: all")
	assert.Contains(t, pills, "Log Names: all")
	assert.Contains(t, pills, "Severities: all")

	// With selections: shows count
	v.selectedResources = []string{"gce_instance", "gcs_bucket"}
	v.selectedSeverities = []string{"ERROR"}

	pills = v.renderFilterPills()
	assert.Contains(t, pills, "Resource Types: 2")
	assert.Contains(t, pills, "Severities: 1")
	assert.Contains(t, pills, "Log Names: all")
}
