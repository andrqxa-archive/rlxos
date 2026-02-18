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
    background: "theme.color.control.fill"
    gradientTop: "#FFFFFFD9"
    gradientBottom: "#ECF2FFC0"
    textColor: "theme.color.text.primary"
    placeholderColor: "theme.color.text.muted"
    borderColor: "theme.color.stroke.hairline"
    focusedBorderColor: "theme.color.stroke.hairline"
    focusRing: true
    focusRingColor: "theme.color.stroke.focus"
    focusRingWidth: 2
    focusRingOffset: 1
    borderRadius: 10
    padding: "8 10"
    shadow: true
    shadowColor: "#10182814"
    shadowOffsetY: 2
    shadowSpread: 6
}
