//go:build amd64

package dl

import "fmt"

// validateMachine checks that the ELF e_machine matches amd64.
func validateMachine(machine uint16) error {
	if machine != emX86_64 {
		return fmt.Errorf("%w: expected EM_X86_64 (%d), got %d", ErrWrongArch, emX86_64, machine)
	}
	return nil
}

// archDefaultLibDirs returns the default library search directories for amd64,
// relative to root.
func archDefaultLibDirs() []string {
	return []string{
		"/lib/x86_64-linux-gnu",
		"/usr/lib/x86_64-linux-gnu",
		"/lib64",
		"/usr/lib64",
		"/lib",
		"/usr/lib",
	}
}
