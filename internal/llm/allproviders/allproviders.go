// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package allproviders imports all LLM providers to register them.
// Import this package if you want all providers to be available:
//
//	import _ "github.com/ayatsuri-lab/ayatsuri/internal/llm/allproviders"
package allproviders

import (
	_ "github.com/ayatsuri-lab/ayatsuri/internal/llm/providers/anthropic"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/llm/providers/gemini"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/llm/providers/local"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/llm/providers/openai"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/llm/providers/openaicodex"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/llm/providers/openrouter"
	_ "github.com/ayatsuri-lab/ayatsuri/internal/llm/providers/zai"
)
