# Avyos Design Guide 
Version: 0.1  
Style: Professional workstation • Glass surfaces • Cool neutral + restrained blue accent

---

## 1) Design principles
1. **Clarity over flair**: glass is a *container*, not the feature. Content remains legible in any wallpaper.
2. **Stable hierarchy**: strong typographic rhythm + consistent spacing beats heavy borders.
3. **Soft geometry, sharp intent**: rounded corners + subtle shadows, but crisp alignment and predictable states.
4. **Restrained color**: one primary accent, semantic colors used sparingly, low chroma neutrals everywhere else.
5. **Fast feedback**: transitions are quick and minimal; avoid playful bounces.

---

## 2) Foundations

### 2.1 Color system
**Color is tokenized** (never hardcode). Tokens are defined separately for Light and Dark modes.

#### Light mode tokens
**Base**
- `bg`: **#EEF2FA** — `rgba(238,242,250,1.00)`
- `scrim.wallpaper`: **#0B1220** @ **8%** — `rgba(11,18,32,0.08)`

**Glass surfaces**
- `surface.glass`: **#F7F9FF** @ **72%** — `rgba(247,249,255,0.72)`
- `surface.glassRaised`: **#FFFFFF** @ **82%** — `rgba(255,255,255,0.82)`
- `surface.sidebar`: **#F3F6FF** @ **76%** — `rgba(243,246,255,0.76)`

**Strokes**
- `stroke.hairline`: **#1B2A4A** @ **14%** — `rgba(27,42,74,0.14)`
- `stroke.divider`: **#1B2A4A** @ **10%** — `rgba(27,42,74,0.10)`
- `stroke.focus`: **#2F6BFF** @ **35%** — `rgba(47,107,255,0.35)`

**Text**
- `text.primary`: **#121A2B** — `rgba(18,26,43,1.00)`
- `text.secondary`: **#3A4963** — `rgba(58,73,99,1.00)`
- `text.muted`: **#63708A** — `rgba(99,112,138,1.00)`
- `text.disabled`: **#121A2B** @ **35%** — `rgba(18,26,43,0.35)`

**Controls**
- `control.fill`: **#FFFFFF** @ **70%** — `rgba(255,255,255,0.70)`
- `control.hover`: **#FFFFFF** @ **82%** — `rgba(255,255,255,0.82)`
- `control.pressed`: **#DCE6FF** @ **70%** — `rgba(220,230,255,0.70)`

**Brand + semantic**
- `accent`: **#2F6BFF** — `rgba(47,107,255,1.00)`
- `accent.hover`: **#2A5FE7** — `rgba(42,95,231,1.00)`
- `accent.subtle`: **#2F6BFF** @ **14%** — `rgba(47,107,255,0.14)`
- `success`: **#2FBF6D** — `rgba(47,191,109,1.00)` | `success.subtle`: `rgba(47,191,109,0.12)`
- `warning`: **#FF8A3D** — `rgba(255,138,61,1.00)` | `warning.subtle`: `rgba(255,138,61,0.14)`
- `danger`: **#FF4D5E** — `rgba(255,77,94,1.00)` | `danger.subtle`: `rgba(255,77,94,0.14)`


#### Dark mode tokens
**Base**
- `bg`: **#0B1220** — `rgba(11,18,32,1.00)`
- `fog.wallpaper`: **#FFFFFF** @ **4%** — `rgba(255,255,255,0.04)`

**Glass surfaces**
- `surface.glass`: **#121B2F** @ **72%** — `rgba(18,27,47,0.72)`
- `surface.glassRaised`: **#18233D** @ **82%** — `rgba(24,35,61,0.82)`
- `surface.sidebar`: **#0F1830** @ **76%** — `rgba(15,24,48,0.76)`

**Strokes**
- `stroke.hairline`: **#EAF0FF** @ **12%** — `rgba(234,240,255,0.12)`
- `stroke.divider`: **#EAF0FF** @ **8%** — `rgba(234,240,255,0.08)`
- `stroke.focus`: **#3B7CFF** @ **45%** — `rgba(59,124,255,0.45)`

**Text**
- `text.primary`: **#EAF0FF** — `rgba(234,240,255,1.00)`
- `text.secondary`: **#B7C2DA** — `rgba(183,194,218,1.00)`
- `text.muted`: **#8F9AB4** — `rgba(143,154,180,1.00)`
- `text.disabled`: **#EAF0FF** @ **40%** — `rgba(234,240,255,0.40)`

**Controls**
- `control.fill`: **#1A2744** @ **72%** — `rgba(26,39,68,0.72)`
- `control.hover`: **#223255** @ **78%** — `rgba(34,50,85,0.78)`
- `control.pressed`: **#2A3D66** @ **82%** — `rgba(42,61,102,0.82)`

**Brand + semantic**
- `accent`: **#3B7CFF** — `rgba(59,124,255,1.00)`
- `accent.hover`: **#3871E3** — `rgba(56,113,227,1.00)`
- `accent.subtle`: **#3B7CFF** @ **18%** — `rgba(59,124,255,0.18)`
- `success`: **#2FBF6D** — `rgba(47,191,109,1.00)` | `success.subtle`: `rgba(47,191,109,0.18)`
- `warning`: **#FF8A3D** — `rgba(255,138,61,1.00)` | `warning.subtle`: `rgba(255,138,61,0.18)`
- `danger`: **#FF4D5E** — `rgba(255,77,94,1.00)` | `danger.subtle`: `rgba(255,77,94,0.18)`


### 2.2 Transparency rules (glass discipline)
- **Primary window glass**: 0.68–0.76 alpha
- **Raised surfaces (menus, popovers)**: 0.78–0.88 alpha
- **Sidebars**: 0.72–0.80 alpha
- Never go below **0.60** alpha for surfaces containing dense text.
- Use **scrims** behind modals instead of lowering glass alpha too much.

### 2.3 Shadows (elevation system)
Shadows are **soft and realistic**, with minimal spread. Use two-layer shadows (ambient + key) where supported.

#### Light mode shadows
- `shadow.0` (flat): none
- `shadow.1` (cards/controls):  
  - key: `rgba(16,24,40,0.10) 0px 6px 18px`  
  - ambient: `rgba(16,24,40,0.06) 0px 2px 6px`
- `shadow.2` (windows):  
  - key: `rgba(16,24,40,0.14) 0px 14px 40px`  
  - ambient: `rgba(16,24,40,0.08) 0px 6px 18px`
- `shadow.3` (modals/popovers):  
  - key: `rgba(16,24,40,0.18) 0px 22px 60px`  
  - ambient: `rgba(16,24,40,0.10) 0px 10px 28px`

#### Dark mode shadows
Dark UI needs less visible shadows; rely more on **stroke + subtle lift**:
- `shadow.1`: `rgba(0,0,0,0.35) 0px 8px 20px`
- `shadow.2`: `rgba(0,0,0,0.45) 0px 16px 42px`
- `shadow.3`: `rgba(0,0,0,0.55) 0px 26px 70px`

### 2.4 Corner radius (curveness)
Workstation-friendly: rounded, not bubbly.

**Radius scale**
- `r.2` = 6px  (chips, small fields)
- `r.3` = 10px (buttons, textfields, list rows)
- `r.4` = 14px (cards, panels)
- `r.5` = 18px (windows, dialogs)
- `r.6` = 22px (dock tiles, large surfaces)

**Rules**
- Windows: `r.5`
- Menus/Popovers: `r.4`
- Inputs/Buttons: `r.3`
- Small toggles/checkbox bases: `r.2`

### 2.5 Blur, noise, and material
**Glass recipe**
- Background blur: **24–32px**
- Saturation boost: **1.15–1.25**
- Noise: **1–2%** (very fine grain) to avoid banding
- Optional: slight vertical gradient inside glass (top +2–4% brightness)

### 2.6 Spacing & layout grid
- Base unit: **4px**
- Common spacing: 8 / 12 / 16 / 20 / 24 / 32
- Window padding: **20–24px**
- Section gap: **16–20px**
- Control height:
  - Dense: **32px**
  - Default: **36px**
  - Comfortable: **40px**
- Icon sizes: 16, 20, 24, 32

### 2.7 Typography
Goal: crisp, neutral, readable.

**Type scale**
- Title / Window title: 18–20px, semibold
- Section header: 14–15px, semibold
- Body: 13–14px, regular
- Caption/meta: 12px, regular

**Line-height**
- UI text: 1.25–1.35
- Lists/tables: 1.2–1.3

**Rules**
- Use **secondary text** for hints and metadata, never reduce opacity of primary text to create hierarchy.
- Avoid ultra-light weights; workstation UIs need legibility.

---

## 3) Interaction states (all components)
Use consistent states everywhere:

**Default**
- Fill: `control.fill`
- Stroke: `stroke.hairline`
- Text: `text.primary`

**Hover**
- Fill: `control.hover`
- Stroke: `stroke.hairline` (+ optional 2–4% more alpha)
- Shadow: add `shadow.1` if clickable surface is otherwise flat

**Pressed**
- Fill: `control.pressed`
- Shadow: reduce intensity slightly (feels “pushed in”)

**Focused**
- Focus ring: `stroke.focus` (2px outer ring)
- Inner stroke remains subtle; focus ring must be visible on any wallpaper

**Disabled**
- Reduce contrast via `text.disabled` and muted strokes
- Avoid lowering whole component opacity below 0.55 (it looks broken on glass)

---

## 4) Components (widget spec)

### 4.1 Window
- Radius: `r.5`
- Glass: `surface.glass`
- Border: `stroke.hairline`
- Shadow: `shadow.2`
- Titlebar height: 44–52px
- Title: left-aligned (workstation) or centered if you prefer; be consistent
- Controls: 12–14px hit targets minimum 28×28px

### 4.2 Buttons
**Primary**
- Fill: `accent`
- Text: white (`rgba(255,255,255,0.95)`)
- Hover: `accent.hover`
- Pressed: slightly darker (or `accent.hover` + 6% darken)
- Radius: `r.3`
- Shadow: `shadow.1` (light) / minimal (dark)

**Secondary**
- Fill: `control.fill`
- Stroke: `stroke.hairline`
- Text: `text.primary`

**Danger**
- Fill: `danger` for destructive confirmation only
- Otherwise use subtle danger outline + danger text

### 4.3 Text field
- Height: 36px default
- Fill: `control.fill`
- Stroke: `stroke.hairline`
- Placeholder: `text.muted`
- Focus: add 2px focus ring `stroke.focus`

### 4.4 Toggle / Switch
- Track (off): `stroke.hairline` + subtle fill (light: white@55%, dark: #1A2744@60%)
- Track (on): `accent` @ 60–75% alpha (so it still feels “glass”)
- Thumb: solid (light: white, dark: near-white) + tiny shadow

### 4.5 Checkbox / Radio
- Base: `control.fill`
- Stroke: `stroke.hairline`
- Checked: fill `accent`, icon white @ 95%
- Radio dot: use accent, keep dot size ~40–45% of circle

### 4.6 List row
- Row height: 36–40px
- Hover: `accent.subtle` + keep text `text.primary`
- Selected: `accent.subtle` + stronger left indicator (2–3px accent bar)

### 4.7 Table
- Header: `surface.glassRaised` + `stroke.divider`
- Rows: transparent, with `stroke.divider` separators
- Hover row: `accent.subtle`
- Align numeric columns right, text left

### 4.8 Progress bar
- Track: `stroke.hairline` + subtle fill
- Fill: `accent` (optionally gradient, but keep it restrained)
- Radius: `r.2`
- Value label: `text.secondary` or on-fill white if placed inside

### 4.9 Dialog / Modal
- Modal surface: `surface.glassRaised`
- Shadow: `shadow.3`
- Scrim:
  - Light: `rgba(11,18,32,0.20–0.28)`
  - Dark: `rgba(0,0,0,0.40–0.55)`
- Actions: Primary rightmost, Cancel left of it

---

## 5) Motion & timing
Workstation motion = subtle and fast.

- Hover: 120–160ms
- Press: 90–120ms
- Focus ring: 120–160ms
- Window open/close: 180–240ms
- Menu: 140–180ms
- Easing: `cubic-bezier(0.2, 0.8, 0.2, 1)` (smooth but not floaty)

Avoid:
- Overshoot/bounce
- Long fades (> 240ms)
- Large scale animations (> 1.02)

---

## 6) Iconography (3D pack integration)
To keep a professional feel with 3D icons:
- Keep icons **simple shapes**, no tiny details that blur at 24px
- Consistent light direction: **top-left**
- Consistent specular strength (don’t mix matte + glossy randomly)
- Maintain a uniform “material family”: soft plastic / clay with gentle highlights

**Icon grid**
- Master canvas: 1024×1024
- Safe area: 840×840
- Visual baseline aligned across icons (avoid floating inconsistently)

---

## 7) Brand (Avyos logo) usage
- Provide 3 variants:
  1. Full-color (primary)
  2. Simplified small-size mark (for 16–24px)
  3. Monochrome (light/dark)
- Clear space: at least **0.5×** the mark’s width on all sides
- Do not place the detailed mark over busy wallpaper without a solid/blurred chip behind it.

---

## 8) Accessibility & contrast targets
- Primary text contrast target: **≥ 4.5:1** against its effective background (glass included)
- Secondary text: **≥ 3:1**
- Focus indicator must be visible on both light/dark wallpapers.
- Minimum hit target: **36×36px** (prefer 40×40 for window controls and key actions).

---

## 9) Implementation notes (token format suggestion)
Use these token families:
- `color.bg`, `color.surface.*`, `color.text.*`, `color.stroke.*`, `color.accent.*`, `color.semantic.*`
- `radius.*`
- `shadow.*`
- `motion.duration.*`, `motion.easing.standard`
- `material.blur.*`, `material.noise.*`

---

## 10) Defaults (recommended)
- Blur: **28px**
- Window alpha: **0.72**
- Border alpha: **0.12–0.14**
- Corner radius (window): **18px**
- Control height: **36px**
- Accent: **Blue** (single system accent)

---
End.