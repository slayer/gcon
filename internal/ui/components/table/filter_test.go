package table

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveFilterKeys(t *testing.T) {
	tests := []struct {
		title    string
		expected []string
	}{
		{
			title:    "Zone",
			expected: []string{"zone"},
		},
		{
			title:    "Machine Type",
			expected: []string{"machine_type", "machinetype", "type"},
		},
		{
			title:    "Internal IP",
			expected: []string{"internal_ip", "internalip", "ip"},
		},
		{
			title:    "Storage Class",
			expected: []string{"storage_class", "storageclass", "class"},
		},
		{
			title:    "Name",
			expected: []string{"name"},
		},
		{
			title:    "Source Disk",
			expected: []string{"source_disk", "sourcedisk", "disk"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			keys := deriveFilterKeys(tt.title)
			assert.Equal(t, tt.expected, keys)
		})
	}
}

func newTestTableWithColumns() Model {
	cols := []Column{
		{Title: "Name", Width: 20, Grow: true},
		{Title: "Zone", Width: 15},
		{Title: "Status", Width: 10},
		{Title: "Machine Type", Width: 15},
	}
	m := NewWithColumns(cols, "Test")

	rows := []Row{
		{Data: []string{"● vm-web-1", "us-central1-a", "RUNNING", "e2-medium"}, FilterValue: "vm-web-1 us-central1-a RUNNING e2-medium", ID: "vm-web-1"},
		{Data: []string{"● vm-web-2", "us-east1-b", "RUNNING", "e2-small"}, FilterValue: "vm-web-2 us-east1-b RUNNING e2-small", ID: "vm-web-2"},
		{Data: []string{"○ vm-db-1", "us-central1-a", "STOPPED", "n2-standard-4"}, FilterValue: "vm-db-1 us-central1-a STOPPED n2-standard-4", ID: "vm-db-1"},
		{Data: []string{"● vm-api", "europe-west1-c", "RUNNING", "e2-micro"}, FilterValue: "vm-api europe-west1-c RUNNING e2-micro", ID: "vm-api"},
	}
	m.SetRows(rows)
	return m
}

func TestParseFilter_PlainText(t *testing.T) {
	m := newTestTableWithColumns()
	spec := m.parseFilter("vm-web")

	assert.Empty(t, spec.fieldFilters)
	assert.Equal(t, "vm-web", spec.freeText)
}

func TestParseFilter_SingleFieldFilter(t *testing.T) {
	m := newTestTableWithColumns()
	spec := m.parseFilter("status:running")

	require.Len(t, spec.fieldFilters, 1)
	assert.Equal(t, "running", spec.fieldFilters[2]) // Status is visible column index 2
	assert.Empty(t, spec.freeText)
}

func TestParseFilter_MultipleFieldFilters(t *testing.T) {
	m := newTestTableWithColumns()
	spec := m.parseFilter("zone:us status:running")

	require.Len(t, spec.fieldFilters, 2)
	assert.Equal(t, "us", spec.fieldFilters[1])      // Zone is visible column index 1
	assert.Equal(t, "running", spec.fieldFilters[2]) // Status is visible column index 2
	assert.Empty(t, spec.freeText)
}

func TestParseFilter_MixedFieldAndFreeText(t *testing.T) {
	m := newTestTableWithColumns()
	spec := m.parseFilter("status:running vm-web")

	require.Len(t, spec.fieldFilters, 1)
	assert.Equal(t, "running", spec.fieldFilters[2])
	assert.Equal(t, "vm-web", spec.freeText)
}

func TestParseFilter_UnknownFieldTreatedAsFreeText(t *testing.T) {
	m := newTestTableWithColumns()
	spec := m.parseFilter("foo:bar vm-web")

	// "foo" is not a known filter key, so "foo:bar" becomes free text
	assert.Empty(t, spec.fieldFilters)
	assert.Equal(t, "foo:bar vm-web", spec.freeText)
}

func TestParseFilter_DerivedShortKey(t *testing.T) {
	m := newTestTableWithColumns()
	// "type" is a derived short key for "Machine Type"
	spec := m.parseFilter("type:e2")

	require.Len(t, spec.fieldFilters, 1)
	assert.Equal(t, "e2", spec.fieldFilters[3]) // Machine Type is visible column index 3
}

func TestParseFilter_NoColDefs(t *testing.T) {
	// Legacy table without enhanced columns
	cols := []Column{}
	m := Model{colDefs: cols}
	spec := m.parseFilter("status:running")

	// Without colDefs, everything is free text
	assert.Empty(t, spec.fieldFilters)
	assert.Equal(t, "status:running", spec.freeText)
}

func TestApplyFilter_PlainTextBackwardCompat(t *testing.T) {
	m := newTestTableWithColumns()

	m.filter.SetValue("vm-web")
	m.applyFilter()

	assert.Equal(t, 2, m.RowCount()) // vm-web-1 and vm-web-2
}

func TestApplyFilter_FieldFilter(t *testing.T) {
	m := newTestTableWithColumns()

	m.filter.SetValue("status:running")
	m.applyFilter()

	assert.Equal(t, 3, m.RowCount()) // 3 running instances
}

func TestApplyFilter_FieldFilterAndFreeText(t *testing.T) {
	m := newTestTableWithColumns()

	m.filter.SetValue("status:running vm-web")
	m.applyFilter()

	assert.Equal(t, 2, m.RowCount()) // vm-web-1 and vm-web-2 (both running)
}

func TestApplyFilter_FieldFilterANDLogic(t *testing.T) {
	m := newTestTableWithColumns()

	m.filter.SetValue("zone:us status:running")
	m.applyFilter()

	// Only vm-web-1 is both in "us" zone AND running (vm-web-2 is us-east1 but also running)
	// us-central1-a contains "us" -> vm-web-1, vm-db-1
	// us-east1-b contains "us" -> vm-web-2
	// Running: vm-web-1, vm-web-2, vm-api
	// AND: vm-web-1 (us + running), vm-web-2 (us + running)
	assert.Equal(t, 2, m.RowCount())
}

func TestApplyFilter_CaseInsensitive(t *testing.T) {
	m := newTestTableWithColumns()

	m.filter.SetValue("STATUS:RUNNING")
	m.applyFilter()

	assert.Equal(t, 3, m.RowCount())
}

func TestApplyFilter_EmptyFilter(t *testing.T) {
	m := newTestTableWithColumns()

	m.filter.SetValue("")
	m.applyFilter()

	assert.Equal(t, 4, m.RowCount()) // All rows
}

func TestFilterKeyCollision_AmbiguousKeysRemoved(t *testing.T) {
	// "Internal IP" and "External IP" both derive "ip" as shorthand
	cols := []Column{
		{Title: "Name", Width: 20},
		{Title: "Internal IP", Width: 15},
		{Title: "External IP", Width: 15},
	}
	m := NewWithColumns(cols, "Test")

	// "ip" should be removed from both columns since it's ambiguous
	assert.NotContains(t, m.colDefs[1].FilterKeys, "ip")
	assert.NotContains(t, m.colDefs[2].FilterKeys, "ip")

	// Unique keys should survive
	assert.Contains(t, m.colDefs[1].FilterKeys, "internal_ip")
	assert.Contains(t, m.colDefs[2].FilterKeys, "external_ip")
}

func TestFilterKeyCollision_UniqueKeysPreserved(t *testing.T) {
	cols := []Column{
		{Title: "Name", Width: 20},
		{Title: "Machine Type", Width: 15},
		{Title: "Zone", Width: 15},
	}
	m := NewWithColumns(cols, "Test")

	// "type" is unique shorthand — should survive
	assert.Contains(t, m.colDefs[1].FilterKeys, "type")
	assert.Contains(t, m.colDefs[1].FilterKeys, "machine_type")
}

func TestFilterKeyCollision_ParseFilterIgnoresAmbiguous(t *testing.T) {
	// With collision removed, "ip:10" should become free text
	cols := []Column{
		{Title: "Name", Width: 20},
		{Title: "Internal IP", Width: 15},
		{Title: "External IP", Width: 15},
	}
	m := NewWithColumns(cols, "Test")
	m.SetRows([]Row{
		{Data: []string{"vm-1", "10.0.0.1", "35.1.2.3"}, FilterValue: "vm-1", ID: "1"},
	})

	spec := m.parseFilter("ip:10")
	// "ip" is ambiguous and was removed, so "ip:10" becomes free text
	assert.Empty(t, spec.fieldFilters)
	assert.Equal(t, "ip:10", spec.freeText)

	// But fully qualified keys still work
	spec = m.parseFilter("internal_ip:10")
	require.Len(t, spec.fieldFilters, 1)
	assert.Equal(t, "10", spec.fieldFilters[1])
}

func TestMatchesFilterSpec_EmptySpec(t *testing.T) {
	row := Row{Data: []string{"test"}, FilterValue: "test"}
	spec := filterSpec{fieldFilters: map[int]string{}}

	assert.True(t, matchesFilterSpec(row, spec))
}

func TestMatchesFilterSpec_FieldOutOfBounds(t *testing.T) {
	row := Row{Data: []string{"test"}, FilterValue: "test"}
	spec := filterSpec{fieldFilters: map[int]string{5: "value"}}

	// Column index 5 is out of bounds for a 1-element Data slice
	assert.False(t, matchesFilterSpec(row, spec))
}

func TestParseFilter_LabelKeyValue(t *testing.T) {
	m := newTestTableWithColumns()
	spec := m.parseFilter("label:env=prod")

	assert.Empty(t, spec.fieldFilters)
	require.Len(t, spec.labelFilters, 1)
	assert.Equal(t, "prod", spec.labelFilters["env"])
	assert.Empty(t, spec.freeText)
}

func TestParseFilter_LabelKeyOnly(t *testing.T) {
	m := newTestTableWithColumns()
	spec := m.parseFilter("label:env")

	require.Len(t, spec.labelFilters, 1)
	assert.Equal(t, "", spec.labelFilters["env"], "key-only label match accepts any value")
}

func TestParseFilter_LabelCombinedWithFieldAndFreeText(t *testing.T) {
	m := newTestTableWithColumns()
	spec := m.parseFilter("status:running label:env=prod vm-web")

	require.Len(t, spec.fieldFilters, 1)
	assert.Equal(t, "running", spec.fieldFilters[2])
	require.Len(t, spec.labelFilters, 1)
	assert.Equal(t, "prod", spec.labelFilters["env"])
	assert.Equal(t, "vm-web", spec.freeText)
}

func TestParseFilter_LabelCaseInsensitive(t *testing.T) {
	m := newTestTableWithColumns()
	spec := m.parseFilter("LABEL:Env=PROD")

	require.Len(t, spec.labelFilters, 1)
	// keys lowercased on parse so values can be looked up consistently
	assert.Equal(t, "prod", spec.labelFilters["env"])
}

func TestApplyFilter_LabelKeyValueMatch(t *testing.T) {
	m := newTestTableWithColumns()
	rows := []Row{
		{Data: []string{"vm-a", "us", "RUNNING", "e2"}, FilterValue: "vm-a", ID: "a", Labels: map[string]string{"env": "prod", "team": "backend"}},
		{Data: []string{"vm-b", "us", "RUNNING", "e2"}, FilterValue: "vm-b", ID: "b", Labels: map[string]string{"env": "staging"}},
		{Data: []string{"vm-c", "us", "RUNNING", "e2"}, FilterValue: "vm-c", ID: "c"}, // no labels
	}
	m.SetRows(rows)

	m.filter.SetValue("label:env=prod")
	m.applyFilter()
	assert.Equal(t, 1, m.RowCount())
}

func TestApplyFilter_LabelKeyExists(t *testing.T) {
	m := newTestTableWithColumns()
	rows := []Row{
		{Data: []string{"vm-a", "us", "RUNNING", "e2"}, FilterValue: "vm-a", ID: "a", Labels: map[string]string{"team": "backend"}},
		{Data: []string{"vm-b", "us", "RUNNING", "e2"}, FilterValue: "vm-b", ID: "b", Labels: map[string]string{"env": "prod"}},
		{Data: []string{"vm-c", "us", "RUNNING", "e2"}, FilterValue: "vm-c", ID: "c"},
	}
	m.SetRows(rows)

	m.filter.SetValue("label:team")
	m.applyFilter()
	assert.Equal(t, 1, m.RowCount(), "only vm-a has the team label")
}

func TestApplyFilter_LabelANDCombinesWithField(t *testing.T) {
	m := newTestTableWithColumns()
	rows := []Row{
		{Data: []string{"vm-a", "us", "RUNNING", "e2"}, FilterValue: "vm-a us RUNNING e2", ID: "a", Labels: map[string]string{"env": "prod"}},
		{Data: []string{"vm-b", "us", "STOPPED", "e2"}, FilterValue: "vm-b us STOPPED e2", ID: "b", Labels: map[string]string{"env": "prod"}},
		{Data: []string{"vm-c", "us", "RUNNING", "e2"}, FilterValue: "vm-c us RUNNING e2", ID: "c", Labels: map[string]string{"env": "staging"}},
	}
	m.SetRows(rows)

	m.filter.SetValue("status:running label:env=prod")
	m.applyFilter()
	assert.Equal(t, 1, m.RowCount(), "only vm-a is both running and labeled env=prod")
}

func TestMatchesFilterSpec_LabelCaseInsensitiveLookup(t *testing.T) {
	row := Row{Data: []string{"x"}, FilterValue: "x", Labels: map[string]string{"Env": "Prod"}}
	spec := filterSpec{labelFilters: map[string]string{"env": "prod"}}
	assert.True(t, matchesFilterSpec(row, spec))
}

func TestMatchesFilterSpec_NilLabelsRejectsLabelFilter(t *testing.T) {
	row := Row{Data: []string{"x"}, FilterValue: "x"}
	spec := filterSpec{labelFilters: map[string]string{"env": ""}}
	assert.False(t, matchesFilterSpec(row, spec), "row without labels cannot satisfy label predicate")
}
