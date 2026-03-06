# ntcharts Time Series Charts — Documentation

## Summary

Replaced minimal 1-line Unicode sparklines with ntcharts braille time series charts in both Compute Engine and Cloud Run observability tabs. Charts render with axes, multi-line resolution, and multi-series overlay support.

## Changes Made

### New Package: `internal/ui/components/metricchart/`

A wrapper around `github.com/NimbleMarkets/ntcharts` time series line chart:

- **`metricchart.go`** — `Chart` struct wrapping ntcharts with:
  - Single-series (`SetData`) and multi-series (`SetDataSets`) support
  - Braille rendering via `DrawBrailleAll()`
  - Fixed Y-range (`SetYRange(0, 100)`) for percentage metrics
  - Auto-scaling Y-range with 10% headroom for other metrics
  - Automatic X-axis label formatting (hour vs date based on time span)
  - Color-coded legends for multi-dataset charts
  - 2-char left indent on all output lines
  - Height constants: `HeightStandard=8`, `HeightCompact=5`

- **`stats.go`** — Stats formatters computing current/avg/peak from `[]gcp.DataPoint`:
  - `FormatPercentageStats` — "Current: 45.2% | Avg: 32.1% | Peak: 78.5% (3:45 PM)"
  - `FormatCountStats` — req/s units
  - `FormatLatencyStats` — ms/s/m auto-scaling
  - `FormatVCPUStats` — vCPU units
  - `FormatInstanceCountStats` — integer formatting
  - `FormatGenericStats` — auto-precision

### Modified: `internal/ui/views/instance_details.go`

- Added `cpuChart` and `memoryChart` fields (both `HeightStandard`, Y-range 0-100)
- Charts initialized in `NewInstanceDetailsView()`, resized in `applySize()`
- `renderObservabilityTab()`: CPU and Memory sections now use `chart.SetData()` + `chart.View()` instead of `RenderSparkline()` + `RenderMetricBar()`

### Modified: `internal/ui/views/cloudrun_observability.go`

- Added 6 chart fields: `requestCountChart`, `latencyChart`, `errorRateChart`, `cpuChart`, `billableTimeChart`, `instanceCountChart`
- Charts initialized in `newCloudRunObservability()`, resized via `resizeCharts()` method
- `renderRequestMetrics()`: Request count uses single-series chart; latency uses 3-series overlay (p50/p95/p99 in green/yellow/red); error rates use 2-series overlay (4xx/5xx in yellow/red)
- `renderResourceMetrics()`: CPU, billable time, and instance count each use single-series charts

### Modified: `internal/ui/views/cloudrun_service_details.go`

- Calls `resizeCharts()` when width changes or observability is first created

## Design Decisions

- Charts are **stateful renderers**, not Bubble Tea models. No `tea.Msg` routing needed since time range is controlled by existing key bindings (1-5 keys).
- Percentage metrics (CPU%, Memory%) use fixed Y-range 0-100 to prevent misleading auto-scaling.
- Multi-series charts (latency, error rates) use GCP color palette: green=#34A853, yellow=#FBBC04, red=#EA4335.
- Old sparkline/metric_bar components are NOT deleted — they have zero maintenance cost and may be useful elsewhere.

### Bug Fixes (Code Review)

- **Chart width off-by-1**: `observability.width` was set to the full `width` instead of `viewportWidth` (which accounts for the focus accent bar), causing charts to clip by 1 column. Fixed by using `viewportWidth` in `applySize()` and `max(1, v.width-1)` in the lazy-init path.
- **Auto-refresh leak**: Leaving the Observability tab called `StopAutoRefresh()` but pending `tea.Tick` callbacks would still fire and trigger API polling. Fixed by adding a `tabActive` flag that `StopAutoRefresh()` clears and `tickAutoRefresh()` checks before scheduling the next tick.

## Testing

- 28 new unit tests in `metricchart_test.go` and `stats_test.go`
- All existing tests pass (no regressions)
- `make lint` clean (0 issues)
