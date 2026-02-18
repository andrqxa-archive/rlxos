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

package wayland

// xdg_wm_base opcodes (client → server)
const (
	xdgWmBaseDestroyOp       = 0
	xdgWmBaseCreatePosOp     = 1
	xdgWmBaseGetXdgSurfaceOp = 2
	xdgWmBasePongOp          = 3
)

// xdg_wm_base events (server → client)
const (
	xdgWmBasePingEvent = 0
)

// xdg_surface opcodes (client → server)
const (
	xdgSurfaceDestroyOp      = 0
	xdgSurfaceGetToplevelOp  = 1
	xdgSurfaceGetPopupOp     = 2
	xdgSurfaceSetGeometryOp  = 3
	xdgSurfaceAckConfigureOp = 4
)

// xdg_surface events (server → client)
const (
	xdgSurfaceConfigureEvent = 0
)

// xdg_toplevel opcodes (client → server)
const (
	xdgToplevelDestroyOp    = 0
	xdgToplevelSetParentOp  = 1
	xdgToplevelSetTitleOp   = 2
	xdgToplevelSetAppIDOp   = 3
	xdgToplevelSetMinSizeOp = 7
	xdgToplevelSetMaxSizeOp = 8
)

// xdg_toplevel events (server → client)
const (
	xdgToplevelConfigureEvent = 0
	xdgToplevelCloseEvent     = 1
)

// xdgShell manages the XDG shell surface and toplevel.
type xdgShell struct {
	cl          *client
	xdgSurface  uint32
	xdgToplevel uint32
	configured  bool
	width       int
	height      int
	closed      bool
	onConfigure func(width, height int)
	onClose     func()
}

// newXdgShell creates the XDG surface and toplevel for a wl_surface.
func newXdgShell(cl *client) (*xdgShell, error) {
	xdg := &xdgShell{
		cl:     cl,
		width:  800,
		height: 600,
	}

	// xdg_wm_base.get_xdg_surface(id, surface)
	xdg.xdgSurface = cl.allocID()
	cl.setHandler(xdg.xdgSurface, xdg.handleXdgSurface)

	payload := make([]byte, 8)
	putUint32(payload, 0, xdg.xdgSurface)
	putUint32(payload, 4, cl.surface)
	if err := cl.conn.sendMsg(cl.xdgWmBase, xdgWmBaseGetXdgSurfaceOp, payload); err != nil {
		return nil, err
	}

	// xdg_surface.get_toplevel(id)
	xdg.xdgToplevel = cl.allocID()
	cl.setHandler(xdg.xdgToplevel, xdg.handleXdgToplevel)

	payload = make([]byte, 4)
	putUint32(payload, 0, xdg.xdgToplevel)
	if err := cl.conn.sendMsg(xdg.xdgSurface, xdgSurfaceGetToplevelOp, payload); err != nil {
		return nil, err
	}

	return xdg, nil
}

// setTitle sets the window title.
func (xdg *xdgShell) setTitle(title string) error {
	return xdg.cl.conn.sendMsg(xdg.xdgToplevel, xdgToplevelSetTitleOp, encodeString(title))
}

// setAppID sets the app ID.
func (xdg *xdgShell) setAppID(appID string) error {
	return xdg.cl.conn.sendMsg(xdg.xdgToplevel, xdgToplevelSetAppIDOp, encodeString(appID))
}

// initialCommit sends the initial empty commit to trigger the first configure.
func (xdg *xdgShell) initialCommit() error {
	return xdg.cl.surfaceCommit()
}

// waitConfigure dispatches events until the surface is configured.
func (xdg *xdgShell) waitConfigure() error {
	for !xdg.configured {
		if err := xdg.cl.dispatch(); err != nil {
			return err
		}
	}
	return nil
}

func (xdg *xdgShell) handleXdgSurface(_ uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)
	if opcode == xdgSurfaceConfigureEvent && len(payload) >= 4 {
		serial := getUint32(payload, 0)

		// ack_configure
		resp := make([]byte, 4)
		putUint32(resp, 0, serial)
		xdg.cl.conn.sendMsg(xdg.xdgSurface, xdgSurfaceAckConfigureOp, resp)

		xdg.configured = true
	}
}

func (xdg *xdgShell) handleXdgToplevel(_ uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)
	switch opcode {
	case xdgToplevelConfigureEvent:
		if len(payload) >= 8 {
			w := int(getInt32(payload, 0))
			h := int(getInt32(payload, 4))
			// width/height of 0 means the compositor doesn't care, keep current
			if w > 0 && h > 0 {
				xdg.width = w
				xdg.height = h
				if xdg.onConfigure != nil {
					xdg.onConfigure(w, h)
				}
			}
		}
	case xdgToplevelCloseEvent:
		xdg.closed = true
		if xdg.onClose != nil {
			xdg.onClose()
		}
	}
}
