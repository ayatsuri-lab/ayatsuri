// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupDoesNotMutatePerTestProcessEnv(t *testing.T) {
	t.Setenv("AYATSURI_HOME", "/original/ayatsuri-home")
	t.Setenv("AYATSURI_CONFIG", "/original/config.yaml")
	t.Setenv("AYATSURI_EXECUTABLE", "/original/ayatsuri")
	t.Setenv("SHELL", "/original/shell")

	helper := Setup(t)

	assert.Equal(t, "/original/ayatsuri-home", os.Getenv("AYATSURI_HOME"))
	assert.Equal(t, "/original/config.yaml", os.Getenv("AYATSURI_CONFIG"))
	assert.Equal(t, "/original/ayatsuri", os.Getenv("AYATSURI_EXECUTABLE"))
	assert.Equal(t, "/original/shell", os.Getenv("SHELL"))

	assert.Contains(t, helper.ChildEnv, "AYATSURI_HOME="+helper.tmpDir)
	assert.Contains(t, helper.ChildEnv, "AYATSURI_CONFIG="+helper.Config.Paths.ConfigFileUsed)
	assert.Contains(t, helper.ChildEnv, "AYATSURI_EXECUTABLE="+helper.Config.Paths.Executable)
	assert.Contains(t, helper.ChildEnv, "SHELL="+helper.Config.Core.DefaultShell)
	assert.Contains(t, helper.ChildEnv, "DEBUG=true")
	assert.Contains(t, helper.ChildEnv, "CI=true")
	assert.Contains(t, helper.ChildEnv, "TZ=UTC")
}
