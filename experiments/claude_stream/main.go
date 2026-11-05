//go:build streamprobe

// A bounded, opt-in protocol experiment. Not the Messenger serve loop.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kolotiloff/agent-messenger/internal/supervisor"
)

func main() {
	root := flag.String("root", "", "existing Messenger data root")
	cli := flag.String("cli", "", "absolute Claude executable")
	private := flag.String("private", "", "new private directory, outside source control")
	allow := flag.Bool("allow-model-calls", false, "authorize exactly two prompts in a fresh Claude test session")
	flag.Parse()
	if !*allow || *root == "" || *cli == "" || *private == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "requires --root, --cli, --private and --allow-model-calls; no model started")
		os.Exit(2)
	}
	result, err := supervisor.RunClaudeStreamProbe(context.Background(), *root, *cli, *private)
	_ = json.NewEncoder(os.Stdout).Encode(result)
	if err != nil {
		fmt.Fprintln(os.Stderr, "probe failed; see private result.json; no automatic retry")
		os.Exit(1)
	}
}
