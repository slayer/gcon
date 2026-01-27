# Label Editor Key Handling Flow

## Before Fix (Bug)

```mermaid
flowchart TD
    A[User types 'q' in label field] --> B[Key message sent to App]
    B --> C{hasTextInputFocused?}
    C -->|Always FALSE - not implemented| D[Process global shortcuts]
    D --> E[Match 'q' to Quit binding]
    E --> F[Application exits ❌]

    style F fill:#f88,stroke:#f44
```

## After Fix (Working)

```mermaid
flowchart TD
    A[User types 'q' in label field] --> B[Key message sent to App]
    B --> C{hasTextInputFocused?}
    C -->|Check InstanceEditorView| D{view.HasTextInputFocused?}
    D -->|state == stateForm AND labelEditor.IsEditing| E[Returns TRUE]
    E --> F[Pass key to view/component]
    F --> G[Character typed in field ✅]

    D2[User in navigation mode] --> H{view.HasTextInputFocused?}
    H -->|state != stateForm OR not editing| I[Returns FALSE]
    I --> J[Process global shortcuts]
    J --> K{Match 'q'?}
    K -->|Yes| L[Application exits ✅]

    style G fill:#8f8,stroke:#4f4
    style L fill:#8f8,stroke:#4f4
```

## State Transitions

```mermaid
stateDiagram-v2
    [*] --> NavigationMode: View loaded
    NavigationMode --> EditingMode: Press 'a' (add) or 'e' (edit)
    EditingMode --> NavigationMode: Press Esc or Enter

    state NavigationMode {
        [*] --> HasTextInputFocused_FALSE
        note right of HasTextInputFocused_FALSE
            Global shortcuts active:
            - q: quit
            - ?: help
            - /: search
        end note
    }

    state EditingMode {
        [*] --> HasTextInputFocused_TRUE
        note right of HasTextInputFocused_TRUE
            All characters go to input:
            - q: types 'q'
            - ?: types '?'
            - /: types '/'
            Only Esc/Enter/Tab handled specially
        end note
    }
```

## Implementation Details

### InstanceEditorView.HasTextInputFocused()

```go
func (v *InstanceEditorView) HasTextInputFocused() bool {
    // ┌─────────────────────────────────────────┐
    // │ Check 1: Must be in form state         │
    // │ (not loading, saving, or showing diff) │
    // └─────────────────────────────────────────┘
    if v.state == stateForm && v.labelEditor != nil {
        // ┌───────────────────────────────────────────┐
        // │ Check 2: Label editor must be editing    │
        // │ (user pressed 'a' or 'e', inputs active) │
        // └───────────────────────────────────────────┘
        return v.labelEditor.IsEditing()
    }
    return false
}
```

### labelEditor.IsEditing()

```go
func (e *Editor) IsEditing() bool {
    return e.editing || e.adding
    //     ^^^^^^^^^^    ^^^^^^^^^
    //     User pressed  User pressed
    //     'e' to edit   'a' to add
}
```

## Key Message Flow in App

```mermaid
sequenceDiagram
    participant User
    participant BubbleTea
    participant App
    participant View as InstanceEditorView
    participant Editor as labelEditor

    User->>BubbleTea: Type 'q'
    BubbleTea->>App: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
    App->>App: hasTextInputFocused()?
    App->>View: HasTextInputFocused()?
    View->>View: Check state == stateForm
    View->>Editor: IsEditing()?
    Editor-->>View: true (editing mode active)
    View-->>App: true
    App->>App: Skip global shortcuts
    App->>View: Update(KeyMsg)
    View->>Editor: Update(KeyMsg)
    Editor->>Editor: textInput.Update(KeyMsg)
    Editor-->>User: Display 'q' in input field ✅
```

## Test Coverage

```mermaid
mindmap
  root((HasTextInputFocused Tests))
    State Tests
      stateLoading → false
      stateSaving → false
      stateDiff → false
      stateError → false
      stateForm not editing → false
      stateForm editing → true
    Integration Tests
      Navigation mode → false
      Add label mode → true
      Edit label mode → true
      After Esc → false
    Edge Cases
      labelEditor == nil → false
      Empty form → false
```
