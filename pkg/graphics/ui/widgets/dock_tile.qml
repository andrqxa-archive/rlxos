component DockTile {
    property text: ""
    property textRole: "paragraph"
    signal clicked

    interactive: true
    focusable: true
    background: "theme.color.surface.glass"
    gradientTop: "transparent"
    gradientBottom: "transparent"
    hoverBackground: "theme.color.control.hover"
    pressedBackground: "theme.color.control.pressed"
    borderColor: "theme.color.stroke.hairline"
    focusedBorderColor: "theme.color.stroke.hairline"
    focusRing: true
    focusRingColor: "theme.color.stroke.focus"
    focusRingWidth: 3
    focusRingOffset: 1
    borderRadius: 20
    minWidth: 56
    minHeight: 56
    padding: "0"
    textColor: "theme.color.text.primary"
    textAlign: "center"
    shadow: true
    shadowColor: "#101A2B1A"
    shadowOffsetY: 2
    shadowSpread: 8
}
