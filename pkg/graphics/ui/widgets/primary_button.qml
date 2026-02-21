component PrimaryButton {
    property text: ""
    property textRole: "paragraph"
    signal clicked

    interactive: true
    focusable: true
    gradientTop: "#6FA4FF"
    gradientBottom: "theme.color.accent"
    hoverBackground: "theme.color.accent.hover"
    pressedBackground: "theme.primaryactive"
    borderColor: "theme.color.accent"
    textColor: "#FFFFFF"
    borderRadius: 10
    minHeight: 36
    padding: "8 14"
    textColor: "theme.color.bg"
    textAlign: "center"
    shadow: true
    shadowColor: "#1018282A"
    shadowOffsetY: 3
    shadowSpread: 8
}
