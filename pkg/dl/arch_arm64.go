//go:build arm64

package dl

import "fmt"

// validateMachine checks that the ELF e_machine matches arm64.
func validateMachine(machine uint16) error {
	if machine != emAARCH64 {
		return fmt.Errorf("%w: expected EM_AARCH64 (%d), got %d", ErrWrongArch, emAARCH64, machine)
	}
	return nil
}

// archDefaultLibDirs returns the default library search directories for arm64,
// relative to root.
func archDefaultLibDirs() []string {
	return []string{
		"/lib/aarch64-linux-gnu",
		"/usr/lib/aarch64-linux-gnu",
		"/lib64",
		"/usr/lib64",
		"/lib",
		"/usr/lib",
	}
}
