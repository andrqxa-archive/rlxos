component TextArea {
    property text: ""
    property textRole: "paragraph"
    property placeholder: ""
    property readOnly: false
    property lineNumbers: false
    property wrap: false
    signal changed

    editable: true
    multiline: true
    focusable: true
    background: "theme.color.surface.card"
    gradientTop: "transparent"
    gradientBottom: "transparent"
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
    shadow: true
    shadowColor: "#101A2B17"
    shadowOffsetY: 2
    shadowSpread: 8
}
