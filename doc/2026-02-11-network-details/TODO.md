# VPC Network Details View

## Task Description
Add a Network Details page with two tabs: Details (network configuration) and Subnets (subnet table with navigable links), following the instance_details.go pattern.

## Implementation Plan
1. [x] Create branch and TODO file
2. [x] Extend GCP client with NetworkDetails, Subnet structs and methods
3. [x] Add GCP client tests
4. [x] Update Networks list view with Enter key → NetworkSelectedMsg
5. [x] Create Network Details view (tabs, focus, links)
6. [x] Integrate into app layer (app.go, app_render.go, app_navigation.go)
7. [x] Add view tests and update documentation
8. [x] Run tests, lint, final verification
