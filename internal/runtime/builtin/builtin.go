// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package builtin

import (
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/agentstep"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/archive"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/chat"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/command"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/dag"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/docker"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/gha"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/http"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/jq"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/kubernetes"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/mail"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/redis"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/router"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/s3"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/sql"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/sql/drivers/postgres"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/sql/drivers/sqlite"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/ssh"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/runtime/builtin/template"
)
