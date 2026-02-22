component MenuItem {
    property name: ""
    property action: ""
    signal clicked

    interactive: true
    focusable: true
    minHeight: 38
    padding: "8 12"
    textAlign: "left"

    background: "transparent"
    gradientTop: "transparent"
    gradientBottom: "transparent"
    borderColor: "transparent"
    focusedBorderColor: "theme.color.stroke.hairline"
    focusRing: false
    borderRadius: 10

    hoverBackground: "theme.color.accent.subtle"
    pressedBackground: "theme.color.control.pressed"
    textColor: "theme.color.text.primary"
}
