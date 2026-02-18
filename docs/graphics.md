# Graphics and UI

This document describes the graphics stack, UI runtime, and reusable widgets.

## Package layout

- `pkg/graphics/core`: Core primitives (`Color`, `Rect`, `Buffer`) and buffer operations.
- `pkg/graphics/input`: Input/backend interfaces and event types.
- `pkg/graphics/raster`: Raster image decoding helpers.
- `pkg/graphics/assets`: Asset helpers (icon path resolution).
- `pkg/graphics/svg`: SVG decode/rasterization engine.
- `pkg/graphics/theme`: Theme model and built-in themes.
- `pkg/graphics/uiparse`: UI lexer/parser/AST and parse/build errors.
- `pkg/graphics/uirender`: Shared UI rendering geometry helpers.
- `pkg/graphics/ui`: UI engine, element model, interaction/rendering, and embedded standard widgets.

Compatibility aliases are provided in `pkg/graphics` and `pkg/graphics/ui` so existing code can continue using legacy entry points.

## UI widget library

Widgets are embedded from `pkg/graphics/ui/widgets/*.qml` and loaded automatically by `ui.NewEngine()`.

### Buttons and actions

- `Button`: Standard clickable control.
  - Properties: `text`, `textRole`.
  - Signals: `clicked`.
- `PrimaryButton`: Accent/emphasis action button.
  - Properties: `text`, `textRole`.
  - Signals: `clicked`.
- `IconButton`: Compact square/round action button for icon glyphs.
  - Properties: `text`, `textRole`.
  - Signals: `clicked`.
- `DockTile`: Larger rounded action tile.
  - Properties: `text`, `textRole`.
  - Signals: `clicked`.
- `Chip`: Small interactive pill/tag.
  - Properties: `text`, `textRole`.
  - Signals: `clicked`.

### Containers and surfaces

- `Card`: Raised glass-like container.
- `Panel`: Medium-elevation surface container.
- `Frame`: Container with optional frame title.
  - Properties: `title`, `titleTextRole`.
- `Page`: Vertical page container with default spacing/padding.
- `Container`: Generic vertical container.

### Layout

- `VBox`: Vertical layout (`direction: column`).
- `Column`: Vertical layout alias (`orientation: vertical`).
- `HBox`: Horizontal layout (`direction: row`).
- `Row`: Horizontal layout alias (`orientation: horizontal`).
- `ToolRow`: Horizontal row for tool controls.
- `Toolbar`: Styled top/action bar container.
- `Grid`: Grid layout.
- `Flow`: Flow layout.
- `Stack`: Stacked layout with active index.
  - Properties: `active`.
- `Spacer`: Expanding spacer element.
- `ScrollView`: Scrollable vertical container.
  - Properties: `overflow`, `scrollStep`.

### Text and labels

- `Label`: Base text element.
  - Properties: `text`, `textColor`, `textAlign`, `textRole`.
- `StatusLabel`: Secondary/muted status text.
- `Paragraph`: Body text preset.
- `Subheading`: Subheading text preset.
- `Heading`: Heading text preset.
- `Title`: Title text preset.
- `StatusBadge`: Non-interactive pill/badge label.

### Inputs and selection

- `TextInput`: Single-line text input.
  - Properties: `text`, `placeholder`, `password`, `readOnly`, `textRole`.
  - Signals: `changed`, `submitted`.
- `SearchInput`: Search-oriented single-line input.
  - Properties: `text`, `placeholder`, `readOnly`, `textRole`.
  - Signals: `changed`, `submitted`.
- `TextArea`: Multi-line editable text area.
  - Properties: `text`, `placeholder`, `readOnly`, `lineNumbers`, `wrap`, `textRole`.
  - Signals: `changed`.
- `Checkbox`: Checkbox with label.
  - Properties: `text`, `checked`, `textRole`.
  - Signals: `changed`.
- `Toggle`: Switch/toggle control.
  - Properties: `text`, `checked`, `textRole`.
  - Signals: `changed`.
- `ListView`: Scrollable/selectable list view.
  - Properties: `items`, `selected`, `textRole`.
  - Signals: `changed`.
- `TableView`: Read-only table-style multiline view.
  - Properties: `text`, `textRole`, `headerTextRole`.
  - Signals: `changed`.
- `Slider`: Range slider.
  - Properties: `value`, `min`, `max`, `step`, `orientation`.
  - Signals: `changed`.
- `ProgressBar`: Progress indicator.
  - Properties: `value`, `showText`, `showProgress`, `textRole`.

### Media and utility

- `Image`: Image display element.
  - Properties: `src`, `scaleMode`, `intrinsicSize`, `background`.
- `HSeparator`: Horizontal divider line.
- `VSeparator`: Vertical divider line.

## Notes for adding widgets

- Keep one component per file in `pkg/graphics/ui/widgets`.
- Prefer role-based text styling (`textRole`) and theme tokens (`theme.*`) over hardcoded colors.
- Add signals only for interaction points needed by handlers (`onXxx` bindings).
