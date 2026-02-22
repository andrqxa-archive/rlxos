component Toggle {
    property text: ""
    property textRole: "paragraph"
    property checked: false
    signal changed

    toggleable: true
    focusable: true
    minHeight: 40
    background: transparent
    onTrackColor: "theme.color.accent"
    offTrackColor: "theme.color.surface.card"
    thumbColor: "theme.color.surface.card"
    borderColor: transparent
    focusedBorderColor: transparent
    trackBorderColor: "theme.color.stroke.hairline"
    focusedTrackBorderColor: "theme.color.stroke.focus"
    textColor: "theme.color.text.primary"
    shadow: true
    shadowColor: "#101A2B17"
    shadowOffsetY: 2
    shadowSpread: 6
}
