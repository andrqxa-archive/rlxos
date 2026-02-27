component TableView {
    property text: ""
    property textRole: "paragraph"
    property headerTextRole: "subheading"
    signal changed

    editable: true
    multiline: true
    focusable: true
    readOnly: true
    lineNumbers: false
    table: true
    background: "theme.color.surface.card"
    gradientTop: "transparent"
    gradientBottom: "transparent"
    textColor: "theme.color.text.secondary"
    borderColor: "theme.color.stroke.hairline"
    focusedBorderColor: "theme.color.stroke.hairline"
    focusRing: true
    focusRingColor: "theme.color.stroke.focus"
    focusRingWidth: 3
    focusRingOffset: 1
    headerBackground: "theme.color.surface.glass"
    dividerColor: "theme.color.stroke.divider"
    headerTextColor: "theme.color.text.primary"
    borderRadius: 14
    padding: "10 12"
    shadow: true
    shadowColor: "#101A2B17"
    shadowOffsetY: 2
    shadowSpread: 8
}
