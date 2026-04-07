// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"github.com/ayatsuri-lab/ayatsuri/internal/cmn/config"
	"github.com/ayatsuri-lab/ayatsuri/internal/core/exec"
	"github.com/ayatsuri-lab/ayatsuri/internal/runtime"
)

// TestHooks exposes selected internal scheduler hooks to external tests only.
type TestHooks struct {
	OnLockWait func()
}

func NewWithHooksForTest(
	cfg *config.Config,
	er EntryReader,
	drm runtime.Manager,
	dagRunStore exec.DAGRunStore,
	queueStore exec.QueueStore,
	procStore exec.ProcStore,
	reg exec.ServiceRegistry,
	coordinatorCli exec.Dispatcher,
	watermarkStore WatermarkStore,
	hooks TestHooks,
) (*Scheduler, error) {
	return newScheduler(
		cfg,
		er,
		drm,
		dagRunStore,
		queueStore,
		procStore,
		reg,
		coordinatorCli,
		watermarkStore,
		schedulerHooks{onLockWait: hooks.OnLockWait},
	)
}
