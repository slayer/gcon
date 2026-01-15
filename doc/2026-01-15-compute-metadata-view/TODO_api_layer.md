# Phase 1: API & Data Layer

## Objective
Implement GCP API methods for reading and writing instance metadata.

## Tasks

- [x] Add `GetInstanceMetadata` method to ComputeClient
  - Fetch metadata from instance
  - Return metadata map and fingerprint
  - Handle API errors

- [x] Add `SetInstanceMetadata` method to ComputeClient
  - Accept metadata map and fingerprint
  - Update instance metadata via API
  - Handle conflict errors (412 Precondition Failed)

- [x] Add `GetProjectMetadata` method to ComputeClient
  - Fetch project-level metadata (for SSH keys)
  - Return metadata map
  - Handle API errors

- [x] Create metadata-related structs and types
  - `InstanceMetadata` struct with metadata map and fingerprint
  - `SSHKey` struct for parsed SSH key data

- [x] Implement SSH key parsing utility
  - Parse SSH keys from metadata value
  - Format: `username:ssh-rsa KEY user@host`
  - Extract username, key type, key data
  - Handle multiple keys (newline-separated)

- [x] Add unit tests for API methods
  - Test GetInstanceMetadata
  - Test SetInstanceMetadata
  - Test GetProjectMetadata
  - Test SSH key parsing
  - Test error handling

## Files to Create/Modify

- `internal/gcp/compute.go` - Add metadata methods
- `internal/gcp/compute_test.go` - Add tests
- `internal/gcp/metadata.go` (new) - Metadata types and utilities
- `internal/gcp/metadata_test.go` (new) - Metadata tests

## Acceptance Criteria

- All API methods implemented and tested
- SSH key parsing works correctly
- Error handling covers common scenarios
- Tests pass with `go test`
