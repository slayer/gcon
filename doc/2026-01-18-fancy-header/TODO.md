# TODO: Fancy Header with Colorized App Name and Powerline Breadcrumbs

## Task ID: 2026-01-18-fancy-header

## Implementation Checklist

- [x] Step 1: Create powerline symbols for header in `internal/ui/symbols/symbols.go`
- [x] Step 2: Add rainbow Google color styles (moved to header component)
- [x] Step 3: Create header component `internal/ui/components/header.go`
- [x] Step 4: Implement rainbow Google rendering
- [x] Step 5: Implement powerline breadcrumbs
- [x] Step 6: Integrate header component into App
- [x] Step 7: Update header rendering in `app_render.go`
- [x] Step 8: Handle width calculations
- [x] Step 9: Add tests
- [x] Step 10: Update documentation

## Current Status

✅ Implementation complete! All tests passing.

## Notes

- Following footer component pattern for reference
- Width handling critical for powerline symbols
- ASCII mode support required

## Final Implementation Notes

### Breadcrumb Positioning
- Breadcrumbs positioned immediately after app name on the left (not right-aligned)
- Two-space gap between app name and first breadcrumb segment
- All segments flow left-to-right in logical order

### Powerline Separator Styling
- Using solid/fat powerline arrows (`\ue0b0`) for bold visual impact
- Each separator has proper foreground/background colors:
  - Foreground = previous segment's background color
  - Background = next segment's background color
- This creates the seamless "arrow flow" effect characteristic of powerline themes
- Separators are styled individually between each segment pair

### Color Scheme
- Project: Blue (#4285F4)
- Category: Green (#34A853)
- Resources: Yellow (#FBBC05)
- Separators inherit colors from adjacent segments
