# Add ntcharts Time Series Charts to Observability Views

## Task Description
Replace minimal sparklines with ntcharts time series charts in observability tabs for both Compute Engine and Cloud Run.

## Implementation Plan

### Phase 1: Add dependency + create wrapper component
- [x] Add ntcharts dependency
- [x] Create `internal/ui/components/metricchart/metricchart.go` wrapper
- [x] Create `internal/ui/components/metricchart/stats.go` formatters
- [x] Create `internal/ui/components/metricchart/metricchart_test.go` tests

### Phase 2: Replace Compute Engine metrics
- [x] Replace CPU sparkline with chart in `instance_details.go`
- [x] Replace Memory sparkline with chart in `instance_details.go`

### Phase 3: Replace Cloud Run metrics
- [x] Replace request count sparkline with chart
- [x] Replace latency sparklines (p50/p95/p99) with multi-series chart
- [x] Replace error rate sparklines (4xx/5xx) with multi-series chart
- [x] Replace CPU/resource sparklines with charts

### Bug Fixes (Code Review)
- [x] Fix chart width off-by-1 (use viewportWidth instead of width)
- [x] Fix auto-refresh leak (add tabActive flag)

### Verification
- [x] `make test` passes
- [x] `make lint` passes
