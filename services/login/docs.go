/*
`login` provides login screen and user session orchestration.

Responsibilities:
- Presents graphical login UI.
- Authenticates users and starts desktop sessions.
- Tracks active sessions and handles lock/logout transitions.

Operational note:
  - This is a core desktop service and should run continuously on graphical
    systems.
*/
package main
