package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/mcp"
)

// Task-model commands. They open the data directory's database directly, so
// they work whether or not the client is running; the client launches the
// recorded runs. They never call a model.
func runTaskCommand(root string, args []string) (bool, error) {
	open := func() (*core.Store, error) {
		if err := os.MkdirAll(root, 0o700); err != nil {
			return nil, err
		}
		return core.Open(filepath.Join(root, "queue.sqlite"))
	}
	printJSON := func(v any) {
		raw, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(raw))
	}
	switch args[0] {
	case "coordinator":
		// stdio MCP server for one project's coordinator chat; stdout carries
		// only JSON-RPC.
		if err := os.MkdirAll(root, 0o700); err != nil {
			return true, err
		}
		return true, mcp.CoordinatorMain(filepath.Join(root, "queue.sqlite"), args[1:])
	case "project-add", "project-folder":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		name := fs.String("name", "", "project name")
		folder := fs.String("folder", "", "absolute project folder on this computer")
		if err := fs.Parse(args[1:]); err != nil {
			return true, err
		}
		st, err := open()
		if err != nil {
			return true, err
		}
		defer st.Close()
		if args[0] == "project-add" {
			err = st.CreateProject(*name, *folder)
		} else {
			err = st.SetProjectFolder(*name, *folder)
		}
		if err == nil {
			fmt.Println("готово; модель не вызывалась")
		}
		return true, err
	case "projects":
		st, err := open()
		if err != nil {
			return true, err
		}
		defer st.Close()
		projects, err := st.Projects()
		if err == nil {
			printJSON(projects)
		}
		return true, err
	case "agent-set":
		fs := flag.NewFlagSet("agent-set", flag.ContinueOnError)
		provider := fs.String("provider", "", "claude|codex")
		cli := fs.String("cli", "", "absolute path of the agent program")
		environment := fs.String("environment", "", "profile directory; empty means the standard one")
		profile := fs.String("profile", "worker", "worker|messenger-only")
		if err := fs.Parse(args[1:]); err != nil {
			return true, err
		}
		st, err := open()
		if err != nil {
			return true, err
		}
		defer st.Close()
		err = st.SetComputerAgent(*provider, *cli, *environment, *profile)
		if err == nil {
			fmt.Println("готово; модель не вызывалась")
		}
		return true, err
	case "tasks":
		fs := flag.NewFlagSet("tasks", flag.ContinueOnError)
		project := fs.String("project", "", "project name; empty lists all")
		if err := fs.Parse(args[1:]); err != nil {
			return true, err
		}
		st, err := open()
		if err != nil {
			return true, err
		}
		defer st.Close()
		tasks, err := st.Tasks(*project, 200)
		if err == nil {
			printJSON(tasks)
		}
		return true, err
	case "task-answer":
		fs := flag.NewFlagSet("task-answer", flag.ContinueOnError)
		task := fs.String("task", "", "task id")
		question := fs.String("question", "", "question id")
		op := fs.String("op", "", "operation id; repeat it to retry safely")
		text := fs.String("text", "", "answer")
		if err := fs.Parse(args[1:]); err != nil {
			return true, err
		}
		if *op == "" {
			return true, errors.New("--op is required, so a repeated command cannot answer twice")
		}
		st, err := open()
		if err != nil {
			return true, err
		}
		defer st.Close()
		err = st.AnswerTask(*task, *question, *op, *text, core.AudienceOwner)
		if err == nil {
			fmt.Println("ответ записан; задача продолжится в том же разговоре")
		}
		return true, err
	case "task-cancel":
		fs := flag.NewFlagSet("task-cancel", flag.ContinueOnError)
		task := fs.String("task", "", "task id")
		if err := fs.Parse(args[1:]); err != nil {
			return true, err
		}
		st, err := open()
		if err != nil {
			return true, err
		}
		defer st.Close()
		state, err := st.CancelTask(*task)
		if err == nil {
			fmt.Println("state=" + state)
		}
		return true, err
	}
	return false, nil
}
