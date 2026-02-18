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
    background: "theme.color.control.fill"
    gradientTop: "#FFFFFFD9"
    gradientBottom: "#ECF2FFC0"
    textColor: "theme.color.text.secondary"
    borderColor: "theme.color.stroke.hairline"
    focusedBorderColor: "theme.color.stroke.hairline"
    focusRing: true
    focusRingColor: "theme.color.stroke.focus"
    focusRingWidth: 2
    focusRingOffset: 1
    headerBackground: "theme.color.surface.glassraised"
    dividerColor: "theme.color.stroke.divider"
    headerTextColor: "theme.color.text.primary"
    borderRadius: 10
    padding: "8 10"
    shadow: true
    shadowColor: "#10182818"
    shadowOffsetY: 2
    shadowSpread: 6
}
