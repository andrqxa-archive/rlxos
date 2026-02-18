component SearchInput {
    property text: ""
    property textRole: "paragraph"
    property placeholder: "Search"
    property readOnly: false
    signal changed
    signal submitted

    editable: true
    focusable: true
    minHeight: 36
    background: "theme.color.control.fill"
    hoverBackground: "theme.color.control.hover"
    textColor: "theme.color.text.primary"
    placeholderColor: "theme.color.text.muted"
    borderColor: "theme.color.stroke.hairline"
    focusedBorderColor: "theme.color.stroke.hairline"
    focusRing: true
    focusRingColor: "theme.color.stroke.focus"
    focusRingWidth: 2
    focusRingOffset: 1
    borderRadius: 18
    padding: "8 12"
}
