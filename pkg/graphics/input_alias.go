package graphics

import gfxinput "avyos.dev/pkg/graphics/input"

type (
	Backend      = gfxinput.Backend
	InputHandler = gfxinput.InputHandler
	EventType    = gfxinput.EventType
	MouseButton  = gfxinput.MouseButton
	Key          = gfxinput.Key
	Modifiers    = gfxinput.Modifiers
	Event        = gfxinput.Event
)

const (
	EventNone               = gfxinput.EventNone
	EventMouseMove          = gfxinput.EventMouseMove
	EventMouseButtonPress   = gfxinput.EventMouseButtonPress
	EventMouseButtonRelease = gfxinput.EventMouseButtonRelease
	EventKeyPress           = gfxinput.EventKeyPress
	EventKeyRelease         = gfxinput.EventKeyRelease
	EventFocusIn            = gfxinput.EventFocusIn
	EventFocusOut           = gfxinput.EventFocusOut
	EventQuit               = gfxinput.EventQuit
	EventResize             = gfxinput.EventResize
	EventShortcut           = gfxinput.EventShortcut
)

const (
	MouseButtonNone       = gfxinput.MouseButtonNone
	MouseButtonLeft       = gfxinput.MouseButtonLeft
	MouseButtonMiddle     = gfxinput.MouseButtonMiddle
	MouseButtonRight      = gfxinput.MouseButtonRight
	MouseButtonWheelUp    = gfxinput.MouseButtonWheelUp
	MouseButtonWheelDown  = gfxinput.MouseButtonWheelDown
	MouseButtonWheelLeft  = gfxinput.MouseButtonWheelLeft
	MouseButtonWheelRight = gfxinput.MouseButtonWheelRight
)

const (
	KeyNone       = gfxinput.KeyNone
	KeyEscape     = gfxinput.KeyEscape
	KeyBackspace  = gfxinput.KeyBackspace
	KeyTab        = gfxinput.KeyTab
	KeyEnter      = gfxinput.KeyEnter
	KeySpace      = gfxinput.KeySpace
	KeyLeft       = gfxinput.KeyLeft
	KeyRight      = gfxinput.KeyRight
	KeyUp         = gfxinput.KeyUp
	KeyDown       = gfxinput.KeyDown
	KeyHome       = gfxinput.KeyHome
	KeyEnd        = gfxinput.KeyEnd
	KeyPageUp     = gfxinput.KeyPageUp
	KeyPageDown   = gfxinput.KeyPageDown
	KeyInsert     = gfxinput.KeyInsert
	KeyDelete     = gfxinput.KeyDelete
	KeyF1         = gfxinput.KeyF1
	KeyF2         = gfxinput.KeyF2
	KeyF3         = gfxinput.KeyF3
	KeyF4         = gfxinput.KeyF4
	KeyF5         = gfxinput.KeyF5
	KeyF6         = gfxinput.KeyF6
	KeyF7         = gfxinput.KeyF7
	KeyF8         = gfxinput.KeyF8
	KeyF9         = gfxinput.KeyF9
	KeyF10        = gfxinput.KeyF10
	KeyF11        = gfxinput.KeyF11
	KeyF12        = gfxinput.KeyF12
	KeyLeftShift  = gfxinput.KeyLeftShift
	KeyRightShift = gfxinput.KeyRightShift
	KeyLeftCtrl   = gfxinput.KeyLeftCtrl
	KeyRightCtrl  = gfxinput.KeyRightCtrl
	KeyLeftAlt    = gfxinput.KeyLeftAlt
	KeyRightAlt   = gfxinput.KeyRightAlt
	KeyCapsLock   = gfxinput.KeyCapsLock
	KeyNumLock    = gfxinput.KeyNumLock
	KeyScrollLock = gfxinput.KeyScrollLock
)

const (
	ModShift    = gfxinput.ModShift
	ModCtrl     = gfxinput.ModCtrl
	ModAlt      = gfxinput.ModAlt
	ModCapsLock = gfxinput.ModCapsLock
)
