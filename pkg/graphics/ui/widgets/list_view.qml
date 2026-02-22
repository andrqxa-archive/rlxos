component ListView {
    property items: ""
    property textRole: "paragraph"
    property selected: -1
    signal changed

    listView: true
    focusable: true
    borderRadius: 14
    rowRadius: 12
    rowHeight: 40
    background: "theme.color.surface.card"
    gradientTop: "transparent"
    gradientBottom: "transparent"
    borderColor: "theme.color.stroke.hairline"
    focusedBorderColor: "theme.color.stroke.hairline"
    focusRing: true
    focusRingColor: "theme.color.stroke.focus"
    focusRingWidth: 3
    focusRingOffset: 1
    dividerColor: "theme.color.stroke.divider"
    hoverBackground: "theme.color.accent.subtle"
    selectedBackground: "theme.color.accent.subtle"
    selectedIndicatorColor: "theme.color.accent"
    selectedIndicatorWidth: 3
    textColor: "theme.color.text.primary"
    shadow: true
    shadowColor: "#101A2B17"
    shadowOffsetY: 2
    shadowSpread: 8
}
