component GhostButton {
    property text: ""
    property textRole: "paragraph"
    signal clicked

    interactive: true
    focusable: true
    minHeight: 36
    padding: "6 8"
    textAlign: "left"
    background: "transparent"
    gradientTop: "transparent"
    gradientBottom: "transparent"
    borderColor: "transparent"
    focusedBorderColor: "transparent"
    hoverBackground: "theme.color.accent.subtle"
    pressedBackground: "theme.color.control.pressed"
    shadow: false
    focusRing: false
}
