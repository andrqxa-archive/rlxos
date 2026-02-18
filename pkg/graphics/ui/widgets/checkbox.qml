component Checkbox {
    property text: ""
    property textRole: "paragraph"
    property checked: false
    signal changed

    checkable: true
    focusable: true
    minHeight: 36
    background: transparent
    boxBackground: "theme.color.control.fill"
    borderColor: transparent
    focusedBorderColor: transparent
    boxBorderColor: "theme.color.stroke.hairline"
    focusedBoxBorderColor: "theme.color.stroke.focus"
    checkColor: "theme.color.accent"
    textColor: "theme.color.text.primary"
}
