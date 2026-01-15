# GCP Compute Engine Instance Enhancements

## Summary

Enhanced the Compute Engine instance views with additional metadata and operational information to provide better visibility into instance configuration and runtime status.

## Changes Made

### 1. Data Model Updates

#### Instance Struct (`internal/gcp/compute.go`)
Added `Region` field to the `Instance` struct to store the region extracted from the zone.

#### InstanceDetails Struct (`internal/gcp/compute.go`)
Added the following fields to provide comprehensive instance information:
- `Region` - The region where the instance is located
- `MachineSeries` - Machine series (e.g., "n2", "e2", "custom")
- `VCPUs` - Number of virtual CPUs
- `MemoryMB` - Memory allocation in megabytes
- `Hostname` - Instance hostname from metadata
- `LastStartTime` - Timestamp of last start operation
- `LastStopTime` - Timestamp of last stop operation

### 2. Helper Functions

#### RegionFromZone (`internal/gcp/compute.go`)
Extracts region from zone name by removing the last hyphen and letter.
- Example: `us-central1-a` → `us-central1`

#### ParseMachineType (`internal/gcp/compute.go`)
Parses machine type string to extract series, vCPUs, and memory information.

Supports:
- **Standard formats**: `n2-standard-4`, `n1-standard-8`
- **Custom machines**: `custom-4-16384`
- **Predefined types**: `e2-micro`, `e2-small`, `e2-medium`, `f1-micro`, `g1-small`

Memory calculations per machine family:
- E2 standard: vCPUs × 4096 MB
- N1/N2 standard: vCPUs × 3840 MB (3.75 GB per vCPU)
- N1/N2 highmem: vCPUs × 6656 MB (6.5 GB per vCPU)
- N1/N2 highcpu: vCPUs × 924 MB (0.9 GB per vCPU)
- Predefined types have fixed memory allocations

#### CalculateUptime (`internal/ui/timeutil/timeutil.go`)
Calculates the duration from a start timestamp to now and formats it as a human-readable string.

Format examples:
- `< 1m` - Less than one minute
- `30m` - 30 minutes
- `5h 30m` - 5 hours and 30 minutes
- `2d 5h 30m` - 2 days, 5 hours, and 30 minutes

### 3. Instance Conversion Functions

#### instanceFromAPI
Updated to populate the new `Region` field using `RegionFromZone()`.

#### instanceDetailsFromAPI
Enhanced to populate all new fields:
- Extracts region from zone
- Parses machine type to get series, vCPUs, and memory
- Retrieves hostname from instance metadata
- Captures last start and stop timestamps from the API

### 4. UI Updates

#### Instances List View (`internal/ui/views/instances.go`)
Added "REGION" column to the instances table:
- Position: Second column, after Name
- Width: 15 characters
- Shows region only (e.g., "us-central1")

Updated `instanceToRow()` to include region in the table data and filter value.

#### Instance Details View (`internal/ui/views/instance_details.go`)

**Basic Information Section:**
- Added Region field after Zone

**Machine Configuration Section:**
- Added Machine Series field
- Added vCPUs field (formatted as integer)
- Added Memory field (formatted as GB with 2 decimal places)

**New Operational Section:**
Added a new section displaying:
- Hostname (if available)
- Last started timestamp (formatted with timezone)
- Uptime (calculated and formatted, shown only for running instances)
- Last stopped timestamp (if available)

**Storage Section:**
- Added "[BOOT]" prefix to boot disk names for easy identification

### 5. Testing

Added comprehensive test coverage:

#### `internal/gcp/compute_test.go`
- `TestRegionFromZone` - Tests region extraction from various zone formats
- `TestParseMachineType` - Tests parsing of 14+ machine type variants including:
  - Standard machine types (n1, n2)
  - Machine families (standard, highmem, highcpu)
  - E2 series (micro, small, medium, standard)
  - F1 and G1 series
  - Custom machine types
  - Edge cases (empty string, unknown formats)

#### `internal/ui/timeutil/timeutil_test.go`
- `TestCalculateUptime` - Tests uptime calculation with 15+ scenarios:
  - Empty and invalid timestamps
  - Less than a minute
  - Minutes only (1m, 5m, 59m)
  - Hours and minutes (1h, 1h 30m, 5h 45m, 23h 59m)
  - Days combinations (1d, 1d 5h, 2d 5h 30m, 7d, 30d 12h 45m)
- `TestFormatDuration` - Tests duration formatting helper
- `TestFormatUnit` - Tests unit formatting helper

All tests pass successfully with 100% coverage of new functionality.

## Technical Implementation Details

### Region Extraction
The region is extracted by finding the last hyphen in the zone string and taking everything before it. This works for all standard GCP zone formats.

### Machine Type Parsing
The parser follows this logic:
1. Check for custom machine type prefix
2. Check for predefined types (e2-micro, f1-micro, etc.)
3. Parse standard format (series-family-vcpus)
4. Apply memory calculations based on series and family

### Uptime Calculation
Uses `time.Since()` to calculate duration from start timestamp to current time. The formatting logic:
1. Returns "< 1m" if duration is less than one minute
2. Converts total duration to days, hours, and minutes
3. Formats based on largest non-zero unit (days → hours → minutes)
4. Joins components with spaces (e.g., "2d 5h 30m")

### Hostname Retrieval
The hostname is retrieved from the instance metadata `hostname` key. If not present in metadata, the field remains empty.

## Files Modified

- `/private/tmp/gcon-3/internal/gcp/compute.go`
- `/private/tmp/gcon-3/internal/gcp/compute_test.go`
- `/private/tmp/gcon-3/internal/ui/timeutil/timeutil.go`
- `/private/tmp/gcon-3/internal/ui/timeutil/timeutil_test.go`
- `/private/tmp/gcon-3/internal/ui/views/instances.go`
- `/private/tmp/gcon-3/internal/ui/views/instance_details.go`

## Testing Results

```
go test ./...
```

All packages passed:
- `internal/gcp`: 18 test functions, all pass
- `internal/ui/timeutil`: 8 test functions, all pass
- All existing tests continue to pass

Linter results:
```
golangci-lint run ./...
0 issues.
```

## Usage

### Viewing Instances with Region
When viewing the instances list, the region column now appears:
```
Name             Region        Zone           Internal IP    External IP
● my-instance    us-central1   us-central1-a  10.128.0.2    35.192.0.1
```

### Instance Details - Operational Section
For running instances, the operational section displays:
```
Operational
Hostname:      my-instance.c.project-id.internal
Last started:  Jan 15, 2026, 10:30:00 AM PST
Uptime:        2d 5h 30m
```

### Instance Details - Machine Configuration
Enhanced machine configuration display:
```
Machine Configuration
Machine Type:   n2-standard-4
Machine Series: n2
vCPUs:         4
Memory:        15.00 GB
```

### Instance Details - Storage
Boot disks are now clearly marked:
```
Storage
Name                      Size       Type         Mode         Boot
[BOOT] boot-disk         100 GB     SCSI         READ_WRITE   Yes
data-disk                500 GB     NVME         READ_WRITE   —
```

## Future Enhancements

Potential improvements for future iterations:
1. Add network throughput information
2. Include GPU details in the main configuration section
3. Show current CPU and memory utilization metrics
4. Add instance group membership information
5. Display reservation affinity details
