// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build unix

package signal

import "syscall"

// See https://pubs.opengroup.org/onlinepubs/9699919799/

var signalMap = map[syscall.Signal]signalInfo{
	syscall.SIGABRT:   {"SIGABRT", true},   // A - Process abort signal
	syscall.SIGALRM:   {"SIGALRM", true},   // T - Alarm clock
	syscall.SIGBUS:    {"SIGBUS", true},    // A - Access to undefined portion of memory object
	syscall.SIGCHLD:   {"SIGCHLD", false},  // I - Child process terminated, stopped, or continued
	syscall.SIGCONT:   {"SIGCONT", false},  // C - Continue executing, if stopped
	syscall.SIGFPE:    {"SIGFPE", true},    // A - Erroneous arithmetic operation
	syscall.SIGHUP:    {"SIGHUP", true},    // T - Hangup
	syscall.SIGILL:    {"SIGILL", true},    // A - Illegal instruction
	syscall.SIGINT:    {"SIGINT", true},    // T - Terminal interrupt signal
	syscall.SIGIO:     {"SIGIO", true},     // T - I/O possible (similar to SIGPOLL)
	syscall.SIGKILL:   {"SIGKILL", true},   // T - Kill (cannot be caught or ignored)
	syscall.SIGPIPE:   {"SIGPIPE", true},   // T - Write on pipe with no one to read it
	syscall.SIGPROF:   {"SIGPROF", true},   // T - Profiling timer expired
	syscall.SIGQUIT:   {"SIGQUIT", true},   // A - Terminal quit signal
	syscall.SIGSEGV:   {"SIGSEGV", true},   // A - Invalid memory reference
	syscall.SIGSTOP:   {"SIGSTOP", false},  // S - Stop executing (cannot be caught or ignored)
	syscall.SIGSYS:    {"SIGSYS", true},    // A - Bad system call
	syscall.SIGTERM:   {"SIGTERM", true},   // T - Termination signal
	syscall.SIGTRAP:   {"SIGTRAP", true},   // A - Trace/breakpoint trap
	syscall.SIGTSTP:   {"SIGTSTP", false},  // S - Terminal stop signal
	syscall.SIGTTIN:   {"SIGTTIN", false},  // S - Background process attempting read
	syscall.SIGTTOU:   {"SIGTTOU", false},  // S - Background process attempting write
	syscall.SIGURG:    {"SIGURG", false},   // I - High bandwidth data available at socket
	syscall.SIGUSR1:   {"SIGUSR1", true},   // T - User-defined signal 1
	syscall.SIGUSR2:   {"SIGUSR2", true},   // T - User-defined signal 2
	syscall.SIGVTALRM: {"SIGVTALRM", true}, // T - Virtual timer expired
	syscall.SIGWINCH:  {"SIGWINCH", false}, // I - Window size change (not in POSIX table)
	syscall.SIGXCPU:   {"SIGXCPU", true},   // A - CPU time limit exceeded
	syscall.SIGXFSZ:   {"SIGXFSZ", true},   // A - File size limit exceeded
}

func isTerminationSignalInternal(sig syscall.Signal) bool {
	if sigInfo, ok := signalMap[sig]; ok {
		return sigInfo.isTermination
	}
	return false
}
