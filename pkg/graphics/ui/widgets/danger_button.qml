component DangerButton {
    property text: ""
    property textRole: "paragraph"
    signal clicked

    interactive: true
    focusable: true
    minHeight: 40
    padding: "10 16"
    background: "theme.color.semantic.danger"
    hoverBackground: "theme.color.semantic.danger"
    pressedBackground: "theme.color.semantic.danger"
    borderColor: "theme.color.semantic.danger"
    focusedBorderColor: "theme.color.stroke.focus"
    borderRadius: 14
    textColor: "#FFFFFF"
    shadow: true
    shadowColor: "#101A2B22"
    shadowOffsetY: 2
    shadowSpread: 8
}
