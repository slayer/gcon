# Viewport-Wrapped Content Views

## Task Description

Wrap content views (InstancesView, BucketsView, ObjectsView) with `bubbles/viewport` component to provide consistent height handling and automatic scrolling for content overflow.

## Implementation Plan

### Phase 1: InstancesView
- [x] Add viewport and ready fields
- [x] Add viewport import
- [x] Update SetSize() for lazy viewport initialization
- [x] Update Update() to pass messages to viewport
- [x] Update View() to wrap content in viewport
- [x] Update renderLoading() to use viewport when ready
- [x] Run tests

### Phase 2: BucketsView
- [x] Add viewport and ready fields
- [x] Add viewport import
- [x] Update SetSize()
- [x] Update Update()
- [x] Update View()
- [x] Update renderLoading()
- [x] Run tests

### Phase 3: ObjectsView
- [x] Add viewport and ready fields
- [x] Add viewport import
- [x] Update SetSize()
- [x] Update Update()
- [x] Update View() - ensure overlays render AFTER viewport
- [x] Update renderLoading()
- [x] Run tests

### Phase 4: Final Verification
- [x] Run full test suite
- [x] Run linter
- [ ] Manual testing

## Key Design Decisions

1. **Lazy viewport initialization**: Using `ready` flag pattern from InstanceDetailsView
2. **Overlay handling**: In ObjectsView, overlays (progress, filepicker, confirm) render AFTER viewport.View()
3. **Height contract**: Maintain `height-1` newlines for sidebar alignment
4. **Viewport height**: Set viewport height = full content height so viewport.View() outputs height-1 newlines (matching sidebar)
5. **Minimum height fallback**: For edge cases (height < 11), fall back to manual padding to ensure minimum 10 newlines
