component IconButton {
    property text: ""
    property textRole: "paragraph"
    signal clicked

    interactive: true
    focusable: true
    minWidth: 36
    minHeight: 36
    padding: "0"
    textAlign: "center"
    background: "theme.color.control.fill"
    hoverBackground: "theme.color.control.hover"
    pressedBackground: "theme.color.control.pressed"
    borderColor: "theme.color.stroke.hairline"
    focusedBorderColor: "theme.color.stroke.hairline"
    focusRing: true
    focusRingColor: "theme.color.stroke.focus"
    focusRingWidth: 2
    focusRingOffset: 1
    borderRadius: 18
    shadow: true
    shadowColor: "#10182814"
    shadowOffsetY: 2
    shadowSpread: 4
}
