component Chip {
    property text: ""
    property textRole: "paragraph"
    signal clicked

    interactive: true
    focusable: true
    expand: false
    minHeight: 28
    padding: "4 10"
    borderRadius: 14
    background: "theme.color.control.fill"
    hoverBackground: "theme.color.control.hover"
    pressedBackground: "theme.color.control.pressed"
    borderColor: "theme.color.stroke.hairline"
    focusRing: true
    focusRingColor: "theme.color.stroke.focus"
    focusRingWidth: 2
    focusRingOffset: 1
    textColor: "theme.color.text.primary"
}
