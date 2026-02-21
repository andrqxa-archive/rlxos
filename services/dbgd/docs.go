/*
`dbgd` is the AvyOS remote debugging daemon.

What it provides:
- TCP debug API endpoint (default `:5037`).
- Remote process/command debugging workflows.
- Internal helper modes for chunked file read/write operations.

Operational note:
- Usually managed as a background service.
*/
package main
