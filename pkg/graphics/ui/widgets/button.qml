component Button {
    property text: ""
    property textRole: "paragraph"
    signal clicked

    interactive: true
    focusable: true
    background: "theme.color.control.fill"
    gradientTop: "#FFFFFFE6"
    gradientBottom: "#EEF3FFC0"
    hoverBackground: "theme.color.control.hover"
    pressedBackground: "theme.color.control.pressed"
    borderColor: "theme.color.stroke.hairline"
    focusedBorderColor: "theme.color.stroke.hairline"
    focusRing: true
    focusRingColor: "theme.color.stroke.focus"
    focusRingWidth: 2
    focusRingOffset: 1
    borderRadius: 10
    minHeight: 36
    padding: "8 14"
    textColor: "theme.color.text.primary"
    textAlign: "center"
    shadow: true
    shadowColor: "#1018281A"
    shadowOffsetY: 2
    shadowSpread: 6
}
