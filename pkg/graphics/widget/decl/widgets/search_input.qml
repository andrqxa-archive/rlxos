component SearchInput {
    property text: ""
    property textRole: "paragraph"
    property placeholder: "Search"
    property readOnly: false
    signal changed
    signal submitted

    editable: true
    focusable: true
    minHeight: 42
    background: "theme.color.surface.card"
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
    padding: "9 14"
}
