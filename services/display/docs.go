/*
`display` is the core display/compositor service.

Responsibilities:
- Starts graphics backend (`auto`, `drm`, or `framebuffer`).
- Handles input events and desktop rendering pipeline.
- Hosts display APIs used by desktop apps and session components.

Operational note:
- This service is required for graphical desktop sessions.
*/
package main
