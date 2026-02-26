# Bubble Tea TUI Designer (gcon Reference)

Design reference for building Bubble Tea terminal user interfaces in gcon. Adapted from generic Bubble Tea patterns to match gcon's project-specific abstractions.

## What It Provides

1. **Design patterns** adapted for gcon's View interface, CreateViewBase, and TableClickDelegate
2. **Component selection guidance** for gcon's custom component library
3. **Architecture best practices** with gcon-specific helpers and base types
4. **Example designs** showing how to add new GCP resource views
5. **Component taxonomy** mapping gcon's components and archetypes

## Reference Documents

| Document | Contents |
|----------|----------|
| `references/design-patterns.md` | 10 patterns: View interface, App router, list views, creation views, message flow, tabs, forms, navigation, error handling, TextInputFocusable |
| `references/architecture-best-practices.md` | Helpers, base types, performance, testing, code organization, common pitfalls |
| `references/bubbletea-components-guide.md` | 14-component quick reference for Bubble Tea ecosystem |
| `references/example-designs.md` | 5 gcon examples: list view, creation view, detail view, action confirmation, sidebar entry |
| `assets/component-taxonomy.json` | Component categories, gcon custom components, archetype mappings |
| `assets/pattern-templates.json` | Implementation templates with files to create/update per view type |

## Quick Start

To add a new GCP resource view, start with the decision guide:

- **List view** → embed `TableClickDelegate`, use `table.Model`
- **Creation form** → embed `CreateViewBase`, use `forms.Form`
- **Detail view** → use `tabs.Tabs` + `viewport.Model`
- **Complex editor** → manual implementation with `forms` + `diff`

Then follow `.claude/rules/adding-new-views.md` for the full checklist.

## Relationship to Project Rules

This skill supplements `.claude/rules/` — it does **not** override them. The rules are authoritative for implementation details (checklists, error handling, rendering). This skill provides broader design context and component selection guidance.

## Files

```
bubbletea-designer/
├── SKILL.md                           # Main skill documentation
├── README.md                          # This file
├── references/
│   ├── design-patterns.md            # gcon-adapted design patterns
│   ├── architecture-best-practices.md # Best practices for gcon
│   ├── bubbletea-components-guide.md  # Bubble Tea component reference
│   └── example-designs.md            # gcon view examples
├── assets/
│   ├── component-taxonomy.json       # Component categories + archetypes
│   └── pattern-templates.json        # Implementation templates
├── .skillfish.json                    # Skill identity
├── VERSION                            # 1.0.0
└── CHANGELOG.md                       # Version history
```

## Resources

- [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- [Lipgloss](https://github.com/charmbracelet/lipgloss)
- [Bubbles Components](https://github.com/charmbracelet/bubbles)
