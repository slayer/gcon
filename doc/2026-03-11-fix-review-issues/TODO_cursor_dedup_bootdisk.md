# Fix Code Review Issues: Cursor Blink, Deduplicate regionFromZone, Boot Disk Name

## Issues

### Issue 1: Cursor blink messages not forwarded to form in edit view
- [x] Add form delegation at end of `Update()` in `instance_config_edit.go`

### Issue 2: Duplicate regionFromZone function
- [x] Export `regionFromZone` as `RegionFromZone` in `compute.go`
- [x] Update internal caller in `compute.go`
- [x] Update test in `compute_test.go`
- [x] Delete `regionFromZoneName` from `instance_form.go`
- [x] Update `instance_create.go` to use `gcp.RegionFromZone()`
- [x] Remove unused `strings` import from `instance_form.go`

### Issue 3: Boot disk name assumption in edit submit
- [x] Add `BootDiskName` field to `InstanceConfigEditSubmitMsg`
- [x] Set `BootDiskName` from loaded instance details in `emitSubmit()`
- [x] Use `msg.BootDiskName` in `handleInstanceConfigEditSubmit`

## Verification
- [x] `go fmt ./...`
- [x] `go vet ./...`
- [x] `go test ./...`
- [x] `make lint`
