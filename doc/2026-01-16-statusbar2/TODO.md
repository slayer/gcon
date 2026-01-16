# Task: Improve Footer Line with Multi-Section Layout

## Goal
Create a flexible footer with multiple sections and powerline separators:
```
[left1 | left2 | left3]    [center1 | center2 | center3]    [right1 | right2 | right3]
```

## Implementation Plan

- [x] Analyze gh-dash footer implementation
- [x] Create Footer component with fixed slots
- [x] Add powerline separators (Unicode code points)
- [x] Add colored section styles (GCP-inspired)
- [x] Integrate Footer into App
- [x] Replace renderFooter() with new component
- [x] Wire task status to right section
- [x] Write tests for footer component
- [x] Run linter and fix issues
- [x] Update documentation

## Design Decisions

1. **Fixed Slots vs Flexible Groups**: Chose fixed slots for simplicity. Each slot is a `*string` pointer (nil = hidden).

2. **Powerline Separators**: Using Unicode code points:
   - `\ue0b0` - Right arrow (solid) for left-to-right transitions
   - `\ue0b1` - Right arrow (thin) for same-color separators
   - `\ue0b2` - Left arrow (solid) for right group
   - `\ue0b3` - Left arrow (thin)

3. **Color Scheme**: GCP-inspired:
   - Left1: Blue (#4285F4) - navigation context
   - Left2: Light blue (#5A95F5) - mode indicator
   - Left3: Dark (#303134) - help text
   - Right: Dark with muted text, task status uses its own colored badges

4. **Pre-rendered Content**: Right2/Right3 support pre-rendered styled content for task status with custom colors.
