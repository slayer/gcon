# Task: Add Bubbles Components (Viewport, TextArea, Timer)

## Task Description

Add three new bubbles components to the project as wrappers with GCP styling:

1. **Viewport** - Scrollable content container for long text (logs, details)
2. **TextArea** - Multi-line text editor for future editing features
3. **Timer** - Countdown/timing component for operation duration display

## Implementation Plan

### 1. Viewport Component (`internal/ui/components/viewport/viewport.go`)
- Wrap bubbles/viewport with GCP styling
- Add optional title/header
- Configure key bindings for scroll navigation
- Support mouse wheel scrolling

### 2. TextArea Component (`internal/ui/components/textarea/textarea.go`)
- Wrap bubbles/textarea with GCP styling
- Configure placeholder text support
- Add character limit configuration
- Support read-only mode for viewing

### 3. Timer Component (`internal/ui/components/timer/timer.go`)
- Wrap bubbles/timer for countdown functionality
- Add elapsed time display mode (stopwatch-like)
- GCP-styled formatting
- Support start/stop/reset operations

### 4. Tests
- Unit tests for each component
- Test initialization, updates, and rendering

### 5. Integration
- Add elapsed time display to Progress component
- Integrate with file transfer operations (download/upload/delete)

## Progress

- [x] Create branch and TODO.md
- [x] Implement Viewport component
- [x] Implement TextArea component
- [x] Implement Timer component
- [x] Write tests for new components
- [x] Run tests and linter
- [x] Create documentation
- [x] Add elapsed time to Progress component
- [x] Integrate elapsed time with file operations
- [x] Add tests for elapsed time functionality
