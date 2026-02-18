# AvyOS Desktop Guide

The AvyOS desktop is a TUI (Text User Interface) tiling window manager, similar to tmux or i3, running entirely in the terminal.

## Overview

```
╭─ terminal-1 ─────────────────╮╭─ terminal-2 ─────────────────╮
│ $ ls /avyos                  ││ $ read /config/init.conf     │
│ apps  cmd  config  data      ││ [init]                       │
│ services                     ││ services = sutra login       │
│                              ││                              │
│ $                            ││ $                            │
│                              ││                              │
╰──────────────────────────────╯╰──────────────────────────────╯
 terminal-1         Welcome to Avyos - Press Ctrl+A ?     03:45 PM
```

The desktop consists of:
- **Panes** — Terminal windows arranged in a tiled layout
- **Status Bar** — Shows current pane, messages, and time
- **Focus Indicator** — Highlighted border on active pane

## Keyboard Shortcuts

### Prefix Key

All compositor commands use **Ctrl+A** as the prefix key, followed by a command key within 2 seconds.

### Quick Reference

| Shortcut | Action |
|----------|--------|
| **Ctrl+A %** | Split vertical (left \| right) |
| **Ctrl+A "** | Split horizontal (top / bottom) |
| **Ctrl+A x** | Close current pane |
| **Ctrl+A c** | Create new pane |
| **Ctrl+A z** | Toggle zoom (fullscreen) |
| **Ctrl+A o** | Next pane |
| **Ctrl+A n** | Next pane |
| **Ctrl+A p** | Previous pane |
| **Ctrl+A ;** | Previous pane |
| **Ctrl+A Arrow** | Move focus in direction |
| **Ctrl+A Space** | Cycle layout mode |
| **Ctrl+A =** | Increase split ratio |
| **Ctrl+A -** | Decrease split ratio |
| **Ctrl+A 0** | Reset all ratios to 50/50 |
| **Ctrl+A ?** | Show help overlay |
| **Ctrl+A :** | Enter command mode |
| **F1** | Show/hide help |

## Pane Management

### Creating Panes

**Split Vertical** (Ctrl+A %):
```
Before:              After:
╭────────────────╮   ╭───────╮╭───────╮
│                │   │       ││       │
│   terminal-1   │ → │ term-1││ term-2│
│                │   │       ││       │
╰────────────────╯   ╰───────╯╰───────╯
```

**Split Horizontal** (Ctrl+A "):
```
Before:              After:
╭────────────────╮   ╭────────────────╮
│                │   │   terminal-1   │
│   terminal-1   │ → ├────────────────┤
│                │   │   terminal-2   │
╰────────────────╯   ╰────────────────╯
```

### Closing Panes

- **Ctrl+A x** — Close the focused pane
- **exit** — Type in shell to close pane
- When all panes close, the desktop exits

### Zooming

**Ctrl+A z** toggles fullscreen mode for the focused pane:

```
Normal:                       Zoomed:
╭───────╮╭───────╮           ╭────────────────╮
│ term-1││ term-2│   →→→     │                │
├───────┴┴───────┤   ←←←     │   terminal-1   │
│    terminal-3  │  Ctrl+A z │                │
╰────────────────╯           ╰────────────────╯
```

## Navigation

### Arrow Key Navigation

After pressing **Ctrl+A**, use arrow keys to move focus:

- **Ctrl+A ↑** — Focus pane above
- **Ctrl+A ↓** — Focus pane below
- **Ctrl+A ←** — Focus pane to the left
- **Ctrl+A →** — Focus pane to the right

### Sequential Navigation

- **Ctrl+A o** or **Ctrl+A n** — Focus next pane
- **Ctrl+A p** or **Ctrl+A ;** — Focus previous pane

### Mouse Support

- **Left Click** — Focus the clicked pane
- **Scroll Wheel** — Reserved for scrollback (future)

## Layout Modes

Cycle through layouts with **Ctrl+A Space**:

### BSP (Binary Space Partitioning) — Default

Tree-based layout where each split divides the space:

```
╭───────────────────┬───────────────────╮
│                   │                   │
│     terminal-1    │     terminal-2    │
│                   ├─────────┬─────────┤
│                   │ term-3  │ term-4  │
╰───────────────────┴─────────┴─────────╯
```

### Main + Stack

One main pane on the left, others stacked on the right:

```
╭─────────────────────┬─────────────────╮
│                     │    terminal-2   │
│                     ├─────────────────┤
│     terminal-1      │    terminal-3   │
│      (main)         ├─────────────────┤
│                     │    terminal-4   │
╰─────────────────────┴─────────────────╯
```

### Grid

Equal-sized grid arrangement:

```
╭─────────┬─────────╮
│ term-1  │ term-2  │
├─────────┼─────────┤
│ term-3  │ term-4  │
╰─────────┴─────────╯
```

### Spiral

Fibonacci spiral pattern:

```
╭───────────────────┬─────────╮
│                   │ term-2  │
│     terminal-1    ├────┬────┤
│                   │ t-3│ t-4│
╰───────────────────┴────┴────╯
```

## Resizing Panes

Adjust the split ratio of the focused pane's parent:

- **Ctrl+A =** — Increase ratio (make focused pane larger)
- **Ctrl+A -** — Decrease ratio (make focused pane smaller)
- **Ctrl+A 0** — Reset all splits to 50/50

## Command Mode

Press **Ctrl+A :** to enter vim-style command mode:

```
 terminal-1                    :sp                      03:45 PM
```

### Available Commands

| Command | Description |
|---------|-------------|
| `:q` or `:quit` | Exit desktop |
| `:sp` or `:split` | Split horizontal |
| `:vs` or `:vsplit` | Split vertical |
| `:close` or `:cl` | Close current pane |
| `:new [cmd]` | New pane (optional command) |
| `:layout` | Cycle layout mode |
| `:zoom` or `:z` | Toggle zoom |
| `:help` or `:h` | Show help |
| `:resize +` | Increase ratio |
| `:resize -` | Decrease ratio |
| `:resize =` | Reset ratios |
| `:!command` | Run command in new pane |

### Shell Commands

Prefix with `!` to run in a new pane:
```
:!notepad /users/alice/notes.txt
```

Press **Escape** to cancel command mode.

## Help Overlay

Press **F1** or **Ctrl+A ?** to show the help overlay:

```
╔════════════════════════════════════════════════════════════╗
║ AVYOS compositor - Tiling Window Manager                   ║
║                                                            ║
║ PREFIX KEY: Ctrl+A (then press command)                    ║
║                                                            ║
║ PANE SPLITTING:                                            ║
║  Ctrl+A %        Split vertical (left | right)             ║
║  Ctrl+A "        Split horizontal (top / bottom)           ║
║                                                            ║
║ NAVIGATION:                                                ║
║  Ctrl+A Arrows   Move focus in direction                   ║
║  Ctrl+A o / n    Next pane                                 ║
║  Ctrl+A ; / p    Previous pane                             ║
║  Mouse Click     Focus pane under cursor                   ║
║                                                            ║
║ Press F1 or Esc to close this help                       ▼ ║
╚════════════════════════════════════════════════════════════╝
```

- **Arrow Up/Down** or **Page Up/Down** — Scroll help
- **F1** or **Escape** — Close help

## Status Bar

The bottom status bar shows:

```
 terminal-1 [ZOOM]    Closed terminal 2              03:45 PM
 └─ pane name         └─ status message              └─ time
```

- **Left** — Current pane name and zoom indicator
- **Center** — Status messages (commands, errors)
- **Right** — Current time

## Tips

### Efficient Workflows

1. **Split for comparison**: Ctrl+A % to view two files side by side
2. **Zoom for focus**: Ctrl+A z to temporarily maximize a pane
3. **Quick commands**: :!cmd to run something in a new pane

### Common Patterns

**Development setup:**
```
Ctrl+A %          # Split vertical
Ctrl+A "          # Split the right pane horizontal
Ctrl+A ←          # Focus left (editor)
```

**Monitoring:**
```
Ctrl+A c          # New pane for each log
Ctrl+A Space      # Switch to grid layout
```

### Terminal in Panes

Each pane runs a full terminal with the AvyOS shell. You can:
- Run any command in `/avyos/cmd/`
- Use Ctrl+C, Ctrl+D, Ctrl+Z normally
- Arrow keys work for shell history

## Exiting

To exit the desktop:
- Close all panes (Ctrl+A x repeatedly)
- Or use `:q` command
- Or type `exit` in all shells
