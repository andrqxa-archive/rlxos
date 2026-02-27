component Slider {
    property value: 0.5
    property min: 0.0
    property max: 1.0
    property step: 0.0
    property orientation: "horizontal"
    signal changed

    slidable: true
    focusable: true
    borderColor: "theme.color.stroke.hairline"
    focusedBorderColor: "theme.color.stroke.hairline"
    focusRing: true
    focusRingColor: "theme.color.stroke.focus"
    focusRingWidth: 2
    focusRingOffset: 1
    trackColor: "theme.color.control.fill"
    thumbColor: "theme.color.accent"
    thumbHoverColor: "theme.color.accent.hover"
    thumbActiveColor: "theme.primaryactive"
    shadow: true
    shadowColor: "#101A2B17"
    shadowOffsetY: 2
    shadowSpread: 4
}
