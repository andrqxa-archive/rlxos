component ListView {
    property items: ""
    property textRole: "paragraph"
    property selected: -1
    signal changed

    listView: true
    focusable: true
    borderRadius: 10
    rowRadius: 10
    rowHeight: 38
    background: "theme.color.control.fill"
    gradientTop: "#FFFFFFD9"
    gradientBottom: "#ECF2FFC0"
    borderColor: "theme.color.stroke.hairline"
    focusedBorderColor: "theme.color.stroke.hairline"
    focusRing: true
    focusRingColor: "theme.color.stroke.focus"
    focusRingWidth: 2
    focusRingOffset: 1
    dividerColor: "theme.color.stroke.divider"
    hoverBackground: "theme.color.accent.subtle"
    selectedBackground: "theme.color.accent.subtle"
    selectedIndicatorColor: "theme.color.accent"
    selectedIndicatorWidth: 3
    textColor: "theme.color.text.primary"
    shadow: true
    shadowColor: "#10182818"
    shadowOffsetY: 2
    shadowSpread: 6
}
