// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package intg_test

import (
	"testing"

	"github.com/ayatsuri-lab/ayatsuri/internal/core"
	"github.com/ayatsuri-lab/ayatsuri/internal/test"
)

func TestDollarEscape(t *testing.T) {
	t.Parallel()

	t.Run("BackslashDollarLiteralInShell", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `
env:
  - PRICE: '\$9.99'
steps:
  - name: shell-price
    command: echo "${PRICE}"
    output: PRICE_OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"PRICE_OUT": "$9.99",
		})
	})
}
