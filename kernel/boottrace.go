package main

// Early boot trace hooks called from rt0_amd64.s.
// Keep these tiny and nosplit so they are safe before runtime startup.

//go:nosplit
func traceBootSerialReady() {
	serialPrint("[boot] serial ready\n")
}

//go:nosplit
func traceBootGDTReady() {
	serialPrint("[boot] gdt ready\n")
}

//go:nosplit
func traceBootIDTReady() {
	serialPrint("[boot] idt ready\n")
}

//go:nosplit
func traceBootSyscallReady() {
	serialPrint("[boot] syscall ready\n")
}

//go:nosplit
func traceBootMemReady() {
	serialPrint("[boot] memory ready\n")
}

//go:nosplit
func traceBootPICReady() {
	serialPrint("[boot] pic ready\n")
}
