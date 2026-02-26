---
name: bubbletea-designer
description: Reference guide for Bubble Tea TUI design patterns, component selection, and architecture. Adapted for gcon's project-specific abstractions (custom View interface, CreateViewBase, TableClickDelegate, forms framework). Use when designing new views, planning component architecture, or needing design guidance for TUI development.
---

# Bubble Tea TUI Designer (gcon Reference)

Design reference for building Bubble Tea terminal user interfaces in gcon. Provides component selection guidance, architecture patterns, and implementation examples adapted to gcon's project-specific abstractions.

## When to Use This Skill

Use this reference when you need help designing, planning, or structuring new views in gcon:

### Design & Planning

- **Design a new view** for a GCP resource
- **Choose the right base type** (TableClickDelegate, CreateViewBase, or manual)
- **Select appropriate components** from gcon's component library
- **Plan message flow** between views and the App router
- **Understand architecture patterns** used across the codebase

### Typical Questions

- "How should I structure a new list view for GKE clusters?"
- "What base type should I use for a resource creation form?"
- "How do I add a detail view with tabs?"
- "Which components do I need for an action with confirmation?"
- "How does the three-phase message flow work?"

### View Types in gcon

| Type | Base Type | Examples |
|------|-----------|----------|
| **Resource list** | `TableClickDelegate` | instances, disks, buckets, SQL instances, service accounts |
| **Resource details** | — (manual) | instance_details, network_details, sql_instance_details |
| **Resource creation** | `CreateViewBase` | snapshot_create, disk_create, image_create, service_account_create |
| **Resource editor** | — (manual, complex) | bucket_create (diff preview), instance_editor (labels) |
| **Policy/read-only** | — (manual) | iam_policy, custom_role_details |

## Reference Documents

### `references/design-patterns.md`

Core architecture patterns adapted for gcon:

1. **View Interface** — gcon's simplified `tea.Model` alternative
2. **App as Central Router** — centralized message dispatch and navigation
3. **List View with TableClickDelegate** — resource list pattern
4. **Creation View with CreateViewBase** — form → submit → async lifecycle
5. **Three-Phase Message Flow** — Selection → Request → Result
6. **Master-Detail with Tabs** — tabbed resource detail views
7. **Form Flow** — gcon's forms framework with sections, validators, field types
8. **Navigation** — view stack, sidebar, command palette
9. **Error Handling** — inline vs full error display, SetError() propagation
10. **TextInputFocusable** — preventing 'q' quit during text input

### `references/architecture-best-practices.md`

Best practices covering:

- gcon-specific helpers (NewGCPSpinner, renderLoading, RenderError, etc.)
- Base types to embed (CreateViewBase, TableClickDelegate)
- Model design, Update/View patterns
- Performance (caching, debouncing, async)
- Error handling and recovery
- Testing with testify and table-driven tests
- Code organization and file structure
- Common pitfalls and how to avoid them

### `references/bubbletea-components-guide.md`

Quick reference for 14 Bubble Tea ecosystem components:

- **Input**: textinput, textarea, filepicker
- **Display**: viewport, table, list, paginator
- **Feedback**: spinner, progress, timer, stopwatch
- **Navigation**: tabs, help
- **Layout**: lipgloss

Plus component initialization patterns, message handling, and best practices.

### `references/example-designs.md`

5 practical examples for gcon:

1. **GKE Clusters List** — TableClickDelegate + custom table
2. **Cloud Run Service Creation** — CreateViewBase + forms
3. **Resource Detail View with Tabs** — tabs + viewport + links
4. **Action with Confirmation** — confirm.TypeConfirmDialog
5. **Sidebar Navigation Entry** — sidebar + command palette integration

Plus component selection guide and complexity matrix for gcon view types.

### `assets/component-taxonomy.json`

Component categorization including:

- Standard Bubble Tea components (input, display, feedback, navigation, layout)
- gcon custom components (actionmenu, commandpalette, confirm, diff, forms, sidebar, sortmenu, etc.)
- Archetype mappings: resource-list, resource-details, resource-create, resource-editor, policy-viewer
- Common component pairings

### `assets/pattern-templates.json`

Implementation templates with:

- Files to create and update for each view type
- Base types to embed
- Required components
- Complexity ratings
- Cross-references to `.claude/rules/` checklists

## Relationship to `.claude/rules/`

This skill supplements but does not override the project rules:

| Topic | Authoritative Source | This Skill |
|-------|---------------------|------------|
| Adding new views checklist | `.claude/rules/adding-new-views.md` | Design patterns and examples |
| Async error handling | `.claude/rules/async-operations-error-handling.md` | Architecture context |
| Forms framework | `.claude/rules/forms-framework.md` | Component selection guidance |
| Rendering | `.claude/rules/bubble-tea-rendering.md` | Layout patterns |
| Component patterns | `.claude/rules/component-patterns.md` | Best practices supplement |
| Key bindings | `.claude/rules/key-bindings.md` | — |

**When in doubt, follow `.claude/rules/`** — they are the canonical reference. This skill provides broader architectural context and design guidance.

## Component Coverage

### gcon Custom Components (`internal/ui/components/`)

| Component | Purpose |
|-----------|---------|
| `actionmenu` | Context menu for item actions (opened with `.` key) |
| `commandpalette` | Navigation and command palette (`:` or `Ctrl+K`) |
| `confirm` | Type-to-confirm dialogs for dangerous actions |
| `diff` | Diff viewer for comparing changes |
| `filepicker` | File selection dialog |
| `footer` | Status bar with spinner, key hints, running tasks |
| `forms` | Form framework (7 field types, validators, sections) |
| `header` | App header with breadcrumbs and project info |
| `inputdialog` | Simple text input dialog |
| `labeledit` | Label key-value editor |
| `links` | Clickable navigation links to related resources |
| `projectselector` | Project switching modal |
| `sidebar` | Tree-based navigation sidebar |
| `sortmenu` | Column sort menu for table views |
| `sparkline` | Sparkline chart for metrics |
| `table` | Enhanced table with sort, filter, mouse support |
| `tabs` | Tab switcher component |
| `viewport` | Scrollable content viewport |

### Standard Bubble Tea Components Used

- `spinner.Model` — via `components.NewGCPSpinner()` wrapper
- `help.Model` — help overlay
- `key.Binding` — keyboard shortcut definitions

## Quick Decision Guide

```
Need a new view?
├── Listing GCP resources?
│   └── Use TableClickDelegate + table.Model
│       Example: instances.go, disks.go
├── Creating a GCP resource?
│   └── Use CreateViewBase + forms.Form
│       Example: snapshot_create.go, disk_create.go
├── Showing resource details?
│   └── Use tabs.Tabs + viewport.Model
│       Example: instance_details.go, network_details.go
├── Complex editor with preview?
│   └── Manual implementation with forms + diff
│       Example: bucket_create.go, instance_editor.go
└── Read-only data view?
    └── Tabs + viewport (no actions)
        Example: iam_policy.go, custom_role_details.go
```
