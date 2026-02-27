component IconButton {
    property text: ""
    property textRole: "paragraph"
    signal clicked

    interactive: true
    focusable: true
    minWidth: 42
    minHeight: 42
    padding: "0"
    textAlign: "center"
    background: "theme.color.surface.glass"
    hoverBackground: "theme.color.control.hover"
    pressedBackground: "theme.color.control.pressed"
    borderColor: "theme.color.stroke.hairline"
    focusedBorderColor: "theme.color.stroke.hairline"
    focusRing: true
    focusRingColor: "theme.color.stroke.focus"
    focusRingWidth: 3
    focusRingOffset: 1
    borderRadius: 14
    shadow: true
    shadowColor: "#101A2B17"
    shadowOffsetY: 2
    shadowSpread: 8
}
