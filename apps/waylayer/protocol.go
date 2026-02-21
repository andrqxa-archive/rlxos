/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

// Wayland protocol constants for the translation-layer (server) side.
// Opcodes are requests (client -> server), events are responses (server -> client).

// --- wl_display (object ID 1) ---

// wl_display requests
const (
	displaySyncOp        = 0
	displayGetRegistryOp = 1
)

// wl_display events
const (
	displayErrorEvent    = 0
	displayDeleteIDEvent = 1
)

// --- wl_registry ---

// wl_registry requests
const (
	registryBindOp = 0
)

// wl_registry events
const (
	registryGlobalEvent       = 0
	registryGlobalRemoveEvent = 1
)

// --- wl_callback ---

// wl_callback events
const (
	callbackDoneEvent = 0
)

// --- wl_compositor ---

// wl_compositor requests
const (
	compositorCreateSurfaceOp = 0
	compositorCreateRegionOp  = 1
)

// --- wl_subcompositor ---

// wl_subcompositor requests
const (
	subcompositorDestroyOp       = 0
	subcompositorGetSubsurfaceOp = 1
)

// --- wl_subsurface ---

// wl_subsurface requests
const (
	subsurfaceDestroyOp     = 0
	subsurfaceSetPositionOp = 1
	subsurfacePlaceAboveOp  = 2
	subsurfacePlaceBelowOp  = 3
	subsurfaceSetSyncOp     = 4
	subsurfaceSetDesyncOp   = 5
)

// --- wl_surface ---

// wl_surface requests
const (
	surfaceDestroyOp         = 0
	surfaceAttachOp          = 1
	surfaceDamageOp          = 2
	surfaceFrameOp           = 3
	surfaceSetOpaqueRegionOp = 4
	surfaceSetInputRegionOp  = 5
	surfaceCommitOp          = 6
	surfaceDamageBufferOp    = 9
)

// wl_surface events
const (
	surfaceEnterEvent = 0
	surfaceLeaveEvent = 1
)

// --- wl_shm ---

// wl_shm requests
const (
	shmCreatePoolOp = 0
)

// --- wl_data_device_manager ---

// wl_data_device_manager requests
const (
	dataDeviceMgrCreateDataSourceOp = 0
	dataDeviceMgrGetDataDeviceOp    = 1
)

// wl_data_device requests
const (
	dataDeviceStartDragOp    = 0
	dataDeviceSetSelectionOp = 1
	dataDeviceReleaseOp      = 2
)

// --- wl_output ---

// wl_output requests
const (
	outputReleaseOp = 0
)

// wl_output events
const (
	outputGeometryEvent = 0
	outputModeEvent     = 1
	outputDoneEvent     = 2
	outputScaleEvent    = 3
	outputNameEvent     = 4
	outputDescEvent     = 5
)

// wl_output.mode flags
const (
	outputModeCurrent   = 0x1
	outputModePreferred = 0x2
)

// wl_shm events
const (
	shmFormatEvent = 0
)

// wl_shm formats
const (
	shmFormatARGB8888 = 0
	shmFormatXRGB8888 = 1
)

// --- wl_shm_pool ---

// wl_shm_pool requests
const (
	shmPoolCreateBufferOp = 0
	shmPoolDestroyOp      = 1
	shmPoolResizeOp       = 2
)

// --- wl_buffer ---

// wl_buffer requests
const (
	bufferDestroyOp = 0
)

// wl_buffer events
const (
	bufferReleaseEvent = 0
)

// --- wl_seat ---

// wl_seat requests
const (
	seatGetPointerOp  = 0
	seatGetKeyboardOp = 1
	seatGetTouchOp    = 2
	seatReleaseOp     = 3
)

// wl_seat events
const (
	seatCapabilitiesEvent = 0
	seatNameEvent         = 1
)

// wl_seat capability bits
const (
	seatCapPointer  = 1
	seatCapKeyboard = 2
	seatCapTouch    = 4
)

// --- wl_pointer ---

// wl_pointer requests
const (
	pointerSetCursorOp = 0
	pointerReleaseOp   = 1
)

// wl_pointer events
const (
	pointerEnterEvent        = 0
	pointerLeaveEvent        = 1
	pointerMotionEvent       = 2
	pointerButtonEvent       = 3
	pointerAxisEvent         = 4
	pointerFrameEvent        = 5
	pointerAxisSourceEvent   = 6
	pointerAxisStopEvent     = 7
	pointerAxisDiscreteEvent = 8
)

// Pointer button states
const (
	pointerButtonReleased = 0
	pointerButtonPressed  = 1
)

// --- wl_keyboard ---

// wl_keyboard events
const (
	keyboardKeymapEvent     = 0
	keyboardEnterEvent      = 1
	keyboardLeaveEvent      = 2
	keyboardKeyEvent        = 3
	keyboardModifiersEvent  = 4
	keyboardRepeatInfoEvent = 5
)

// wl_keyboard requests
const (
	keyboardReleaseOp = 0
)

// wl_keyboard keymap formats
const (
	keyboardKeymapFormatNoKeymap = 0
	keyboardKeymapFormatXKBv1    = 1
)

// wl_keyboard key states
const (
	keyboardKeyReleased = 0
	keyboardKeyPressed  = 1
)

// --- xdg_wm_base ---

// xdg_wm_base requests
const (
	xdgWmBaseDestroyOp       = 0
	xdgWmBaseCreatePosOp     = 1
	xdgWmBaseGetXdgSurfaceOp = 2
	xdgWmBasePongOp          = 3
)

// xdg_wm_base events
const (
	xdgWmBasePingEvent = 0
)

// --- xdg_surface ---

// xdg_surface requests
const (
	xdgSurfaceDestroyOp      = 0
	xdgSurfaceGetToplevelOp  = 1
	xdgSurfaceGetPopupOp     = 2
	xdgSurfaceSetGeometryOp  = 3
	xdgSurfaceAckConfigureOp = 4
)

// xdg_surface events
const (
	xdgSurfaceConfigureEvent = 0
)

// --- xdg_positioner ---

// xdg_positioner requests
const (
	xdgPositionerDestroyOp                 = 0
	xdgPositionerSetSizeOp                 = 1
	xdgPositionerSetAnchorRectOp           = 2
	xdgPositionerSetAnchorOp               = 3
	xdgPositionerSetGravityOp              = 4
	xdgPositionerSetConstraintAdjustmentOp = 5
	xdgPositionerSetOffsetOp               = 6
	xdgPositionerSetReactiveOp             = 7
	xdgPositionerSetParentSizeOp           = 8
	xdgPositionerSetParentConfigureOp      = 9
)

// --- xdg_toplevel ---

// xdg_toplevel requests
const (
	xdgToplevelDestroyOp         = 0
	xdgToplevelSetParentOp       = 1
	xdgToplevelSetTitleOp        = 2
	xdgToplevelSetAppIDOp        = 3
	xdgToplevelShowWindowMenuOp  = 4
	xdgToplevelMoveOp            = 5
	xdgToplevelResizeOp          = 6
	xdgToplevelSetMinSizeOp      = 7
	xdgToplevelSetMaxSizeOp      = 8
	xdgToplevelSetMaximizedOp    = 9
	xdgToplevelUnsetMaximizedOp  = 10
	xdgToplevelSetFullscreenOp   = 11
	xdgToplevelUnsetFullscreenOp = 12
	xdgToplevelSetMinimizedOp    = 13
)

// xdg_toplevel events
const (
	xdgToplevelConfigureEvent = 0
	xdgToplevelCloseEvent     = 1
)

// xdg_toplevel states
const (
	xdgToplevelStateMaximized  = 1
	xdgToplevelStateFullscreen = 2
	xdgToplevelStateResizing   = 3
	xdgToplevelStateActivated  = 4
)

// --- xdg_popup ---

// xdg_popup requests
const (
	xdgPopupDestroyOp    = 0
	xdgPopupGrabOp       = 1
	xdgPopupRepositionOp = 2
)

// xdg_popup events
const (
	xdgPopupConfigureEvent    = 0
	xdgPopupPopupDoneEvent    = 1
	xdgPopupRepositionedEvent = 2
)

// Global interface names
const (
	ifaceWlCompositor = "wl_compositor"
	ifaceWlSubcomp    = "wl_subcompositor"
	ifaceWlDataDevMgr = "wl_data_device_manager"
	ifaceWlOutput     = "wl_output"
	ifaceWlShm        = "wl_shm"
	ifaceWlSeat       = "wl_seat"
	ifaceXdgWmBase    = "xdg_wm_base"
)

// Global interface versions we advertise
const (
	versionWlCompositor = 4
	versionWlSubcomp    = 1
	versionWlDataDevMgr = 3
	versionWlOutput     = 4
	versionWlShm        = 1
	versionWlSeat       = 5
	versionXdgWmBase    = 2
)

// Linux evdev button codes
const (
	evBtnLeft   = 0x110
	evBtnRight  = 0x111
	evBtnMiddle = 0x112
)
