component TextInput {
    property text: ""
    property textRole: "paragraph"
    property placeholder: ""
    property password: false
    property readOnly: false
    signal changed
    signal submitted

    editable: true
    focusable: true
    minHeight: 40
    background: "theme.color.surface.card"
    gradientTop: "transparent"
    gradientBottom: "transparent"
    hoverBackground: "theme.color.control.hover"
    textColor: "theme.color.text.primary"
    placeholderColor: "theme.color.text.muted"
    borderColor: "theme.color.stroke.hairline"
    focusedBorderColor: "theme.color.stroke.hairline"
    focusRing: true
    focusRingColor: "theme.color.stroke.focus"
    focusRingWidth: 3
    focusRingOffset: 1
    borderRadius: 14
    padding: "10 12"
    shadow: false
}
