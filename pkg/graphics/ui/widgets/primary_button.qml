component PrimaryButton {
    property text: ""
    property textRole: "paragraph"
    signal clicked

    interactive: true
    focusable: true
    gradientTop: "theme.color.accent"
    gradientBottom: "theme.color.accent.alt"
    hoverBackground: "theme.color.accent.hover"
    pressedBackground: "theme.primaryactive"
    borderColor: "theme.color.accent"
    textColor: "#FFFFFF"
    borderRadius: 14
    minHeight: 40
    padding: "10 16"
    textAlign: "center"
    shadow: true
    shadowColor: "#0D63F34D"
    shadowOffsetY: 4
    shadowSpread: 12
}
