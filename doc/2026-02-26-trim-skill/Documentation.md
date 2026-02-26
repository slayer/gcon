# Trim & Adapt bubbletea-designer Skill

## Summary

Trimmed the `bubbletea-designer` skill from a generic Python-based TUI design automation tool to a focused reference guide adapted for gcon's project-specific patterns.

## Changes Made

### Removed (unnecessary files)
- `scripts/` — entire directory (Python scaffolding scripts, incompatible with gcon's View interface)
- `tests/` — tests for the removed Python scripts
- `__pycache__/` — compiled Python cache
- `.claude-plugin/` — plugin marketplace configuration
- `assets/keywords.json` — activation keyword metadata
- `INSTALLATION.md` — already installed
- `DECISIONS.md` — documented Python script architecture decisions (no longer relevant)

### Updated

1. **`references/design-patterns.md`** — Replaced generic `tea.Model` examples with gcon's View interface, CreateViewBase, TableClickDelegate, App router, three-phase message flow, forms framework, navigation stack, error handling, and TextInputFocusable patterns.

2. **`references/architecture-best-practices.md`** — Added gcon-specific helpers table (NewGCPSpinner, renderLoading, RenderError, etc.), base types reference, gcon file structure, and 9 common pitfalls specific to the project.

3. **`references/example-designs.md`** — Replaced generic examples with 5 gcon-specific designs: GKE clusters list (TableClickDelegate), Cloud Run creation (CreateViewBase), detail view with tabs, action with confirmation dialog, and sidebar navigation entry. Added gcon component selection guide and complexity matrix.

4. **`assets/component-taxonomy.json`** — Added `gcon_custom` category with all 24 custom components. Replaced generic archetypes with gcon-specific ones: resource-list, resource-details, resource-create, resource-editor, policy-viewer, each with base type and example view references.

5. **`assets/pattern-templates.json`** — Replaced generic templates with gcon implementation templates including files to create/update, base types to embed, and cross-references to `.claude/rules/` checklists.

6. **`SKILL.md`** — Trimmed from ~1,500 lines to ~170 lines. Removed all Python script documentation, validation details, and performance sections. Restructured as a reference-only guide with view type table, reference document index, component coverage table, relationship to `.claude/rules/`, and quick decision guide.

7. **`README.md`** — Updated to reflect trimmed scope. Removed Python testing instructions, dependency info, and marketplace installation. Added reference document table, quick start decision guide, and `.claude/rules/` relationship note.

### Kept (unchanged)
- `references/bubbletea-components-guide.md` — generic 14-component reference, still useful as-is
- `.skillfish.json` — skill identity metadata
- `VERSION` — 1.0.0
- `CHANGELOG.md` — version history

## Design Decision

Chose **Option A (Adapt Skill to Match gcon)** over Option B (Refactor gcon to match generic patterns) because gcon's custom abstractions solve real problems and are well-documented. Refactoring would risk regressions across 42 view files for no user-facing value.

## Verification

- `make test` — all tests pass
- `make lint` — 0 issues
- No code changes — only documentation/skill files modified
