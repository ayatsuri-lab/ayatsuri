// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package signal

import (
	"os"
	"syscall"
)

var nameToSignalInfo = map[string]syscall.Signal{}

func init() {
	for sig, info := range signalMap {
		nameToSignalInfo[info.name] = sig
	}
}

// IsTerminationSignalOS checks if the given os.Signal is a termination signal
func IsTerminationSignalOS(sis os.Signal) bool {
	sig, ok := sis.(syscall.Signal)
	if !ok {
		return false
	}
	return isTerminationSignalInternal(sig)
}

// IsTerminationSignal checks if the given signal is a termination signal
func IsTerminationSignal(sig syscall.Signal) bool {
	return isTerminationSignalInternal(sig)
}

// GetSignalNum returns the signal number for the given signal name
func GetSignalNum(sig string, fallback ...syscall.Signal) int {
	if s, ok := nameToSignalInfo[sig]; ok {
		return int(s)
	}
	if len(fallback) > 0 {
		return int(fallback[0])
	}
	return int(syscall.SIGTERM)
}

type signalInfo struct {
	name          string
	isTermination bool
}
