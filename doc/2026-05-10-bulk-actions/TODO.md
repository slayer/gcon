# Bulk Actions in Objects Browser

## Goal

Multi-select files/folders in the GCS Objects browser and run bulk
operations on the selection.

## Selection UX

- `Space` toggles selection on the cursor row.
- `*` toggles select-all on the currently visible (post-filter) rows.
- `Esc` clears selection first; if selection is empty, falls through
  to existing back-navigation.
- A leading "Sel" column shows `[✓]` / `[ ]`.
- Status bar shows "N selected" when selection is non-empty.
- The `..` row is never selectable.
- Folder navigation (`enter` on folder, `→`, `←`, `..`, `r` refresh,
  post-upload/post-delete reload) clears the selection. Cursor
  movement (`j`/`k`/arrow keys) preserves it so the user can build a
  selection while scrolling.

## Single vs bulk routing

The existing single-row keys stay the same; they just operate on the
selection when it's non-empty:

- `D` (delete): bulk if selection non-empty, else cursor row
- `d` (download): same
- Action menu `.` swaps to a bulk-actions menu when selection exists

## Bulk actions in this PR

- [x] Selection infrastructure (Space, *, Esc-clears, status)
- [x] Bulk delete — single confirmation dialog ("Delete N objects?")
- [x] Bulk download — iterate, progress per file, summary on completion
- [x] Change storage class — picker (STANDARD/NEARLINE/COLDLINE/ARCHIVE);
      server-side rewrite per object

## Deferred to a follow-up

- Copy / move to another folder — needs a destination-folder picker
  dialog that doesn't exist yet. Worth a separate PR.
- Bulk hold toggles, custom-time updates, content-type fixes —
  user said "skip unless asked".

## Implementation notes

- Selection is `map[string]struct{}` keyed by the GCS object full name
  (= row.ID). Lifetime: cleared on every navigation gesture
  (`beginNavigation`).
- Re-building rows on toggle is cheap (a few hundred at most) and
  preserves the table's bookkeeping.
- Bulk delete / download reuse the per-object code paths; the only
  new wiring is iterating + aggregating progress.
- Storage-class change is a server-side rewrite: `obj.CopierFrom(obj)`
  with `Copier.StorageClass` set to the desired class. (GCS does not
  expose a metadata-only StorageClass update; it's part of object
  data placement and requires a rewrite. For typical objects this is
  fast and doesn't transfer bytes off Google's network.)
