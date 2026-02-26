# Bubble Tea Components Guide

Complete reference for Bubble Tea ecosystem components.

## Core Input Components

### textinput.Model
**Purpose**: Single-line text input
**Use Cases**: Search boxes, single field forms, command input
**Key Methods**:
- `Focus()` / `Blur()` - Focus management
- `SetValue(string)` - Set text programmatically
- `Value()` - Get current text

**Example Pattern**:
```go
input := textinput.New()
input.Placeholder = "Search..."
input.Focus()
```

### textarea.Model
**Purpose**: Multi-line text editing
**Use Cases**: Message composition, text editing, large text input
**Key Features**: Line wrapping, scrolling, cursor management

### filepicker.Model
**Purpose**: File system navigation
**Use Cases**: File selection, file browsers
**Key Features**: Directory traversal, file type filtering, path resolution

## Display Components

### viewport.Model
**Purpose**: Scrollable content display
**Use Cases**: Log viewers, document readers, large text display
**Key Methods**:
- `SetContent(string)` - Set viewable content
- `GotoTop()` / `GotoBottom()` - Navigation
- `LineUp()` / `LineDown()` - Scroll control

### table.Model
**Purpose**: Tabular data display
**Use Cases**: Data tables, structured information
**Key Features**: Column definitions, row selection, styling

### list.Model
**Purpose**: Filterable, navigable lists
**Use Cases**: Item selection, menus, file lists
**Key Features**: Filtering, pagination, custom item delegates

### paginator.Model
**Purpose**: Page-based navigation
**Use Cases**: Paginated content, chunked display

## Feedback Components

### spinner.Model
**Purpose**: Loading/waiting indicator
**Styles**: Dot, Line, Minidot, Jump, Pulse, Points, Globe, Moon, Monkey

### progress.Model
**Purpose**: Progress indication
**Modes**: Determinate (0-100%), Indeterminate
**Styling**: Gradient, solid color, custom

### timer.Model
**Purpose**: Countdown timer
**Use Cases**: Timeouts, timed operations

### stopwatch.Model
**Purpose**: Elapsed time tracking
**Use Cases**: Duration measurement, time tracking

## Navigation Components

### tabs
**Purpose**: Tab-based view switching
**Pattern**: Lipgloss-based tab rendering

### help.Model
**Purpose**: Help text and keyboard shortcuts
**Modes**: Short (inline), Full (overlay)

## Layout with Lipgloss

**JoinVertical**: Stack components vertically
**JoinHorizontal**: Place components side-by-side
**Place**: Position with alignment
**Border**: Add borders and padding

## Component Initialization Pattern

```go
type model struct {
    component1 component1.Model
    component2 component2.Model
}

func (m model) Init() tea.Cmd {
    return tea.Batch(
        m.component1.Init(),
        m.component2.Init(),
    )
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    // Update each component
    var cmd tea.Cmd
    m.component1, cmd = m.component1.Update(msg)
    cmds = append(cmds, cmd)

    m.component2, cmd = m.component2.Update(msg)
    cmds = append(cmds, cmd)

    return m, tea.Batch(cmds...)
}
```

## Message Handling

**Standard Messages**:
- `tea.KeyMsg` - Keyboard input
- `tea.MouseMsg` - Mouse events
- `tea.WindowSizeMsg` - Terminal resize
- `tea.QuitMsg` - Quit signal

**Component Messages**:
- `progress.FrameMsg` - Progress/spinner animation
- `spinner.TickMsg` - Spinner tick
- `textinput.ErrMsg` - Input errors

## Best Practices

1. **Always delegate**: Let components handle their own messages
2. **Batch commands**: Use `tea.Batch()` for multiple commands
3. **Focus management**: Only one component focused at a time
4. **Dimension tracking**: Update component sizes on `WindowSizeMsg`
5. **State separation**: Keep UI state in model, business logic separate
