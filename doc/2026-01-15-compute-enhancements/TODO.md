# GCP Compute Engine Instance Enhancements

## Task Description

Enhance the Compute Engine instance views with additional metadata and functionality:
1. **Uptime calculation**: Display how long running instances have been up
2. **Location column**: Show region in the instances list table
3. **Boot disk indicator**: Mark boot disks with "[BOOT]" prefix

## Implementation Plan

### Phase 1: Add struct fields and parsing helpers
- [ ] Add `Region` field to `Instance` struct
- [ ] Add fields to `InstanceDetails`: `Region`, `MachineSeries`, `VCPUs`, `MemoryMB`, `Hostname`, `LastStartTime`, `LastStopTime`
- [ ] Implement `RegionFromZone(zone string) string` helper
- [ ] Implement `ParseMachineType(machineType string) (series, vcpus, memoryMB)` helper

### Phase 2: Create timeutil package
- [ ] Create `internal/ui/timeutil/` package
- [ ] Implement `CalculateUptime(startTimeISO string) string` function
- [ ] Write comprehensive tests for uptime calculation

### Phase 3: Update instance conversion functions
- [ ] Update `instanceFromAPI` to populate `Region` field
- [ ] Update `instanceDetailsFromAPI` to populate all new fields

### Phase 4: Update UI views
- [ ] Add "REGION" column to instances table
- [ ] Update instance details view with new operational section
- [ ] Add "[BOOT]" prefix to boot disk in storage section

### Phase 5: Testing and validation
- [ ] Write tests for `RegionFromZone`
- [ ] Write tests for `ParseMachineType` (all machine type variants)
- [ ] Update existing view tests
- [ ] Run full test suite
- [ ] Run linter and fix issues

## Requirements

### Machine Type Parsing

Handle these formats:
- Standard: `n2-standard-4` → series="n2", vcpus=4, memory=15360MB
- Custom: `custom-4-16384` → series="custom", vcpus=4, memory=16384MB
- Predefined: `e2-medium`, `f1-micro`, `g1-small`

Memory mappings:
- e2-micro: 1024MB, e2-small: 2048MB, e2-medium: 4096MB, e2-standard-X: X*4096MB
- f1-micro: 614MB, g1-small: 1740MB
- n1/n2-standard-X: X*3840MB (3.75GB per vCPU)
- n1/n2-highmem-X: X*6656MB (6.5GB per vCPU)
- n1/n2-highcpu-X: X*924MB (0.9GB per vCPU)

### Uptime Format

Examples:
- "2d 5h 30m" (days, hours, minutes)
- "5h 30m" (hours, minutes)
- "30m" (minutes only)
- "< 1m" (less than a minute)

### Region Extraction

Extract region from zone:
- `us-central1-a` → `us-central1`
- `europe-west1-b` → `europe-west1`

## Testing Notes

- Test all machine type formats including edge cases
- Test uptime calculation with various time differences
- Verify region extraction from different zone formats
- Ensure existing tests still pass
