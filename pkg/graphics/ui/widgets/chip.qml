component Chip {
    property text: ""
    property textRole: "paragraph"
    signal clicked

    interactive: true
    focusable: true
    expand: false
    minHeight: 30
    padding: "6 12"
    borderRadius: 999
    background: "theme.color.surface.glass"
    hoverBackground: "theme.color.control.hover"
    pressedBackground: "theme.color.control.pressed"
    borderColor: "theme.color.stroke.hairline"
    focusRing: true
    focusRingColor: "theme.color.stroke.focus"
    focusRingWidth: 3
    focusRingOffset: 1
    textColor: "theme.color.text.primary"
}
