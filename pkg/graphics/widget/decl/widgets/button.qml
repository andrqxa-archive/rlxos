component Button {
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
    borderRadius: 14
    minHeight: 40
    padding: "10 16"
    textColor: "theme.color.text.primary"
    textAlign: "center"
    shadow: true
    shadowColor: "#101A2B1A"
    shadowOffsetY: 2
    shadowSpread: 8
}
