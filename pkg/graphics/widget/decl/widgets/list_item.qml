component ListItem {
    property text: ""
    property textRole: "paragraph"
    property selected: false
    signal clicked

    interactive: true
    focusable: true
    minHeight: 40
    padding: "8 12"
    textAlign: "left"
    borderRadius: 12

    background: "theme.color.surface.glass"
    gradientTop: "transparent"
    gradientBottom: "transparent"
    hoverBackground: "theme.color.accent.subtle"
    hoverGradientTop: "#0D63F326"
    hoverGradientBottom: "#0D63F31A"
    pressedBackground: "theme.color.control.pressed"
    selectedBackground: "theme.color.accent.subtle"
    selectedGradientTop: "#0D63F33D"
    selectedGradientBottom: "#0D63F32B"
    selectedIndicatorColor: "theme.color.accent"
    selectedIndicatorGradientTop: "theme.color.accent"
    selectedIndicatorGradientBottom: "theme.color.accent.alt"
    selectedIndicatorWidth: 3

    borderColor: "transparent"
    focusedBorderColor: "theme.color.stroke.hairline"
    focusRing: false
    textColor: "theme.color.text.primary"
}
