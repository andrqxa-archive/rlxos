package input

// HasMod returns true if the modifier is set.
func (e Event) HasMod(mod Modifiers) bool {
	return e.Modifiers&mod != 0
}

// IsShift returns true if shift is held.
func (e Event) IsShift() bool {
	return e.HasMod(ModShift)
}

// IsCtrl returns true if ctrl is held.
func (e Event) IsCtrl() bool {
	return e.HasMod(ModCtrl)
}

// IsAlt returns true if alt is held.
func (e Event) IsAlt() bool {
	return e.HasMod(ModAlt)
}
