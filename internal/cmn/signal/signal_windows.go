// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package signal

import (
	"syscall"
)

var signalMap = map[syscall.Signal]signalInfo{
	syscall.SIGABRT:    {"SIGABRT", true},
	syscall.SIGFPE:     {"SIGFPE", true},
	syscall.SIGILL:     {"SIGILL", true},
	syscall.SIGKILL:    {"SIGKILL", true},
	syscall.SIGHUP:     {"SIGHUP", true},
	syscall.SIGINT:     {"SIGINT", true},
	syscall.SIGSEGV:    {"SIGSEGV", true},
	syscall.SIGTERM:    {"SIGTERM", true},
	syscall.Signal(10): {"SIGUSR1", true}, // Map to Windows equivalent
	syscall.Signal(12): {"SIGUSR2", true}, // Map to Windows equivalent
}

func isTerminationSignalInternal(sig syscall.Signal) bool {
	if info, ok := signalMap[sig]; ok {
		return info.isTermination
	}
	return false
}
