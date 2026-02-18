component PrimaryButton {
    property text: ""
    property textRole: "paragraph"
    signal clicked

    interactive: true
    focusable: true
    background: "theme.color.accent"
    hoverBackground: "theme.color.accent.hover"
    pressedBackground: "theme.primaryactive"
    borderColor: "theme.color.accent"
    focusedBorderColor: "theme.color.accent"
    focusRing: true
    focusRingColor: "theme.color.stroke.focus"
    focusRingWidth: 2
    focusRingOffset: 1
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
