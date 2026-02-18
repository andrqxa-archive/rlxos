component DangerButton {
    property text: ""
    property textRole: "paragraph"
    signal clicked

    interactive: true
    focusable: true
    minHeight: 36
    padding: "8 14"
    background: "theme.color.semantic.danger"
    hoverBackground: "theme.color.semantic.danger"
    pressedBackground: "theme.color.semantic.danger"
    borderColor: "theme.color.semantic.danger"
    focusedBorderColor: "theme.color.stroke.focus"
    textColor: "#FFFFFF"
    shadow: false
}
