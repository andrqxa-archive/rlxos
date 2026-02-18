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

// Syscall globals — written by assembly, read by Go dispatcher.
// Safe for single-core (interrupts disabled during SYSCALL).
var (
	sysNum      uintptr
	sysArgs     [6]uintptr
	sysRet      uintptr
	sysRetAddr  uintptr
	nextFakeTID uintptr = 2
	fakeMonoNS  uint64  = 1
)

// Linux syscall numbers (amd64)
const (
	sysRead             = 0
	sysWrite            = 1
	sysClose            = 3
	sysMmap             = 9
	sysMprotect         = 10
	sysMunmap           = 11
	sysBrk              = 12
	sysRtSigaction      = 13
	sysRtSigprocmask    = 14
	sysSchedYield       = 24
	sysMincore          = 27
	sysMadvise          = 28
	sysNanosleep        = 35
	sysGetpid           = 39
	sysClone            = 56
	sysExit             = 60
	sysKill             = 62
	sysFcntl            = 72
	sysGetrlimit        = 97
	sysSigaltstack      = 131
	sysArchPrctl        = 158
	sysGettid           = 186
	sysTkill            = 200
	sysFutex            = 202
	sysSchedGetaffinity = 204
	sysExitGroup        = 231
	sysOpenat           = 257
	sysEpollCreate1     = 291
	sysPipe2            = 293
	sysClockGettime     = 228
)

// Linux arch_prctl operations
const (
	archSetGS = 0x1001
	archSetFS = 0x1002
	archGetFS = 0x1003
	archGetGS = 0x1004
)

// Linux mmap flags
const (
	mapAnonymous = 0x20
	mapPrivate   = 0x02
	mapFixed     = 0x10
)

// Linux errno values (returned as negative)
const (
	eNOSYS = 38
	eNOMEM = 12
	eAGAIN = 11
	eINVAL = 22
	eNOENT = 2
	eBADF  = 9
)

// handleSyscall dispatches syscalls from the assembly handler.
// CRITICAL: This must NOT allocate heap memory or use Go runtime features
// that would trigger recursive syscalls. All functions called must be //go:nosplit.
//
//go:nosplit
//go:noinline
func handleSyscall() {
	num := sysNum
	a1, a2, a3 := sysArgs[0], sysArgs[1], sysArgs[2]
	a4, a5, a6 := sysArgs[3], sysArgs[4], sysArgs[5]

	var ret uintptr

	switch num {
	case sysMmap:
		ret = doMmap(a1, a2, a3, a4, a5, a6)
	case sysMunmap:
		ret = doMunmap(a1, a2)
	case sysMprotect:
		ret = 0 // no-op, all memory is RWX in ring 0
	case sysMadvise:
		ret = 0 // no-op
	case sysBrk:
		ret = doBrk(a1)
	case sysWrite:
		ret = doWrite(a1, a2, a3)
	case sysRead:
		ret = doRead(a1, a2, a3)
	case sysFutex:
		ret = doFutex(a1, a2, a3, a4, a5, a6)
	case sysClone:
		ret = doClone(a1, a2, a3, a4, a5)
	case sysArchPrctl:
		ret = doArchPrctl(a1, a2)
	case sysRtSigaction:
		ret = 0 // accept all signal actions
	case sysRtSigprocmask:
		ret = 0 // accept all signal masks
	case sysSigaltstack:
		ret = 0 // no-op
	case sysSchedGetaffinity:
		ret = doSchedGetaffinity(a1, a2, a3)
	case sysGetpid:
		ret = 1
	case sysGettid:
		ret = 1
	case sysGetrlimit:
		ret = doGetrlimit(a1, a2)
	case sysExit:
		halt()
	case sysExitGroup:
		halt()
	case sysClose:
		ret = 0
	case sysFcntl:
		ret = 0
	case sysSchedYield:
		ret = 0
	case sysNanosleep:
		ret = doNanosleep(a1, a2)
	case sysMincore:
		ret = doMincore(a1, a2, a3)
	case sysTkill:
		ret = 0
	case sysKill:
		ret = 0
	case sysEpollCreate1:
		ret = negErrno(eNOSYS)
	case sysPipe2:
		ret = negErrno(eNOSYS)
	case sysClockGettime:
		ret = doClockGettime(a1, a2)
	case sysOpenat:
		ret = negErrno(eNOENT)
	default:
		// Unknown syscall — log and return ENOSYS
		serialPrint("SYSCALL unknown: ")
		serialPrintDec(uint64(num))
		serialPrint("\n")
		ret = negErrno(eNOSYS)
	}

	sysRet = ret
}

//go:nosplit
func negErrno(e uintptr) uintptr {
	return ^e + 1 // -e in two's complement
}

//go:nosplit
func doWrite(fd, buf, count uintptr) uintptr {
	if fd == 1 || fd == 2 { // stdout or stderr
		p := (*[1 << 30]byte)(ptrOf(buf))
		for i := uintptr(0); i < count; i++ {
			serialWriteByte(p[i])
		}
		return count
	}
	return negErrno(eBADF)
}

//go:nosplit
func doRead(fd, buf, count uintptr) uintptr {
	// No file system — return 0 (EOF) for any reads
	return 0
}

//go:nosplit
func doArchPrctl(code, addr uintptr) uintptr {
	switch code {
	case archSetFS:
		wrmsr(msrFSBASE, uint64(addr))
		return 0
	case archGetFS:
		v := rdmsr(msrFSBASE)
		*(*uint64)(ptrOf(addr)) = v
		return 0
	case archSetGS:
		wrmsr(msrGSBASE, uint64(addr))
		return 0
	case archGetGS:
		v := rdmsr(msrGSBASE)
		*(*uint64)(ptrOf(addr)) = v
		return 0
	}
	return negErrno(eINVAL)
}

//go:nosplit
func doSchedGetaffinity(pid, cpusetsize, mask uintptr) uintptr {
	if cpusetsize < 8 {
		return negErrno(eINVAL)
	}
	// Report 1 CPU
	p := (*[1]uint64)(ptrOf(mask))
	p[0] = 1 // CPU 0 only
	return 8 // bytes written
}

//go:nosplit
func doGetrlimit(resource, rlim uintptr) uintptr {
	// Return generous limits
	type rlimit struct {
		cur, max uint64
	}
	r := (*rlimit)(ptrOf(rlim))
	r.cur = 1 << 30 // 1 GB
	r.max = 1 << 30
	return 0
}

//go:nosplit
func doNanosleep(req, rem uintptr) uintptr {
	// Busy-wait approximation (no timer yet)
	// Just return immediately for MVP
	return 0
}

//go:nosplit
func doClockGettime(clockid, tp uintptr) uintptr {
	// Return a synthetic monotonic time for runtime bootstrap.
	type timespec struct {
		sec, nsec int64
	}
	fakeMonoNS += 1_000_000 // +1ms per call
	t := (*timespec)(ptrOf(tp))
	t.sec = int64(fakeMonoNS / 1_000_000_000)
	t.nsec = int64(fakeMonoNS % 1_000_000_000)
	return 0
}

//go:nosplit
func doMincore(addr, length, vec uintptr) uintptr {
	// Linux mincore requires addr to be page-aligned and length > 0.
	if addr == 0 || (addr&(pageSize-1)) != 0 || length == 0 || vec == 0 {
		return negErrno(eINVAL)
	}
	// Report "resident" for the first queried page; enough for runtime probe.
	*(*byte)(ptrOf(vec)) = 1
	return 0
}

//go:nosplit
func doClone(flags, stack, parentTid, childTid, tls uintptr) uintptr {
	// Minimal single-thread emulation:
	// Report parent-side success and synthetic TID.
	tid := nextFakeTID
	nextFakeTID++
	if parentTid != 0 {
		*(*uint32)(ptrOf(parentTid)) = uint32(tid)
	}
	if childTid != 0 {
		*(*uint32)(ptrOf(childTid)) = uint32(tid)
	}
	return tid
}
