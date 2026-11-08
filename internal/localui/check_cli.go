package localui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/diagnostics"
)

type cliChecker func(context.Context, string, string, string) (diagnostics.CLICheck, error)

// defaultProfileFunc is the agent's standard profile folder; replaced in tests.
var defaultProfileFunc = diagnostics.DefaultProfile

func serveCLICheck(w http.ResponseWriter, r *http.Request, decoder *json.Decoder, checker cliChecker, busy chan struct{}) {
	var input struct {
		Provider    string `json:"provider"`
		CLI         string `json:"cli"`
		Environment string `json:"environment"`
	}
	err := decoder.Decode(&input)
	// An agent connected with its standard profile has no profile path: the
	// check uses the agent's standard profile folder (T-056).
	if err == nil && input.Environment == "" && (input.Provider == "claude" || input.Provider == "codex" || input.Provider == "cursor") {
		input.Environment, err = defaultProfileFunc(input.Provider)
	}
	if err != nil || decoder.Decode(new(any)) != io.EOF ||
		(input.Provider != "claude" && input.Provider != "codex" && input.Provider != "cursor") ||
		!filepath.IsAbs(input.CLI) || !filepath.IsAbs(input.Environment) ||
		len(input.CLI) > 4096 || len(input.Environment) > 4096 || strings.ContainsRune(input.CLI+input.Environment, 0) {
		http.Error(w, "invalid diagnostic selection", http.StatusBadRequest)
		return
	}
	select {
	case busy <- struct{}{}:
		defer func() { <-busy }()
	default:
		http.Error(w, "a diagnostic is already running", http.StatusConflict)
		return
	}
	report, err := checker(r.Context(), input.Provider, input.CLI, input.Environment)
	if err != nil {
		// Raw provider errors and account information must not enter the UI.
		http.Error(w, "check selected executable and profile paths", http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "check": report})
}
