component Toggle {
    property text: ""
    property textRole: "paragraph"
    property checked: false
    signal changed

    toggleable: true
    focusable: true
    minHeight: 36
    background: transparent
    onTrackColor: "#2F6BFFB8"
    offTrackColor: "theme.color.control.fill"
    thumbColor: "#FFFFFF"
    borderColor: transparent
    focusedBorderColor: transparent
    trackBorderColor: "theme.color.stroke.hairline"
    focusedTrackBorderColor: "theme.color.stroke.focus"
    textColor: "theme.color.text.primary"
    shadow: true
    shadowColor: "#10182810"
    shadowOffsetY: 2
    shadowSpread: 4
}
