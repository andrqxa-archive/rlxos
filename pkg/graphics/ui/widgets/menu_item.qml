component MenuItem {
    property name: ""
    property action: ""
    signal clicked

    interactive: true
    focusable: true
    minHeight: 34
    padding: "7 12"
    textAlign: "left"

    background: "transparent"
    gradientTop: "transparent"
    gradientBottom: "transparent"
    borderColor: "transparent"
    focusedBorderColor: "theme.color.stroke.hairline"
    focusRing: false
    borderRadius: 8

    hoverBackground: "theme.color.accent.subtle"
    pressedBackground: "theme.color.control.pressed"
    textColor: "theme.color.text.primary"
}
