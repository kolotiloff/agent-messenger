package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/mcp"
	"github.com/kolotiloff/agent-messenger/internal/platform"
	"github.com/kolotiloff/agent-messenger/internal/supervisor"
)

func main() {
	root, args := splitArgs(os.Args[1:])
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	if args[0] == "mcp" {
		if err := mcp.Main(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if root == "" {
		var err error
		root, err = platform.DefaultRoot()
		if err != nil {
			fail(err)
		}
	}
	if err := run(root, args); err != nil {
		fail(err)
	}
}

func run(root string, args []string) error {
	switch args[0] {
	case "serve":
		allow, err := serveAllowsModels(args[1:])
		if err != nil {
			return err
		}
		err = supervisor.Run(context.Background(), root, supervisor.AdapterLauncher{AllowModels: allow})
		if errors.Is(err, platform.ErrAlreadyRunning) {
			return errors.New("клиент уже запущен; команды нужно отправлять ему, а не запускать второй экземпляр")
		}
		if errors.Is(err, platform.ErrUnsupported) {
			return fmt.Errorf("операция этой ОС не поддерживается и не выполнена: %w", err)
		}
		return err
	case "status", "pause", "resume", "stop-job", "shutdown", "enqueue", "inspect", "resolve", "register-session":
		response, err := control.Call(root, request(args))
		if err != nil {
			return err
		}
		if !response.OK {
			return errors.New(response.Error)
		}
		printResponse(response)
		return nil
	case "help":
		usage()
		return nil
	default:
		usage()
		return errors.New("неизвестная команда")
	}
}

func request(args []string) control.Request {
	flags := map[string]string{}
	for i := 1; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") && i+1 < len(args) {
			flags[strings.TrimPrefix(args[i], "--")] = args[i+1]
			i++
		}
	}
	req := control.Request{
		Cmd: args[0], Alias: flags["alias"], MessageID: flags["id"], Sender: flags["from"], Text: flags["text"],
		Confirmation: flags["confirm"], Provider: flags["provider"], Environment: flags["environment"],
		ConversationID: flags["conversation-id"], CLI: flags["cli"], Workspace: flags["workspace"],
	}
	if args[0] == "resolve" && len(args) > 1 && !strings.HasPrefix(args[1], "--") {
		req.Action = args[1]
	}
	return req
}

func printResponse(response control.Response) {
	if response.Status != nil {
		fmt.Printf("paused=%t cli_launch=%s broker=%s held=%d queued=%d pid=%d\n",
			response.Status.Paused, response.Status.CLILaunch, response.Status.Broker,
			response.Status.Held, response.Status.Queued, response.Status.Supervisor)
		if response.Status.Detail != "" {
			fmt.Println(response.Status.Detail)
		}
	}
	if response.Stop != nil {
		fmt.Printf("state=%s signal=%s\n%s\n", response.Stop.State, response.Stop.Signal, response.Stop.Note)
	}
	if response.Created != nil {
		fmt.Printf("created=%t\n", *response.Created)
	}
	if len(response.Inspect) > 0 {
		fmt.Println(string(response.Inspect))
	}
}

func serveAllowsModels(args []string) (bool, error) {
	allow := false
	for _, arg := range args {
		if arg == "--allow-model-calls" {
			allow = true
			continue
		}
		return false, fmt.Errorf("неизвестный флаг serve: %s", arg)
	}
	return allow, nil
}

func splitArgs(args []string) (string, []string) {
	root := ""
	var rest []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--root" && i+1 < len(args) {
			root = args[i+1]
			i++
			continue
		}
		rest = append(rest, args[i])
	}
	return root, rest
}

func usage() {
	fmt.Fprintf(os.Stderr, `Agent Messenger — локальный клиент без окна.

--root стоит перед командой. Один пользователь может держать только один serve,
даже с другим каталогом данных. Реальный CLI запускается только после register-session.

  messenger --root КАТАЛОГ serve [--allow-model-calls]
  messenger --root КАТАЛОГ register-session --provider claude|codex|cursor --environment КАТАЛОГ --conversation-id ID --alias ИМЯ --cli ФАЙЛ [--workspace КАТАЛОГ]
  messenger --root КАТАЛОГ status
  messenger --root КАТАЛОГ pause
  messenger --root КАТАЛОГ resume
  messenger --root КАТАЛОГ stop-job
  messenger --root КАТАЛОГ shutdown
  messenger --root КАТАЛОГ enqueue --alias ИМЯ --id ID --from ОТ --text ТЕКСТ
  messenger --root КАТАЛОГ inspect
  messenger --root КАТАЛОГ resolve fail|accept|retry --alias ИМЯ [--confirm actions-may-repeat]

pause — новые задания не резервируются, активный процесс не останавливается.
stop-job — сначала отзывает MCP, затем сигналит группе только если pid, время старта и группа всё ещё принадлежат той же попытке.
shutdown — очередь остаётся, активный процесс не убивается. Следующий serve сразу отзывает старый MCP и не снимает блокировку сессии.
Без --allow-model-calls serve не запускает CLI и не тратит токены.
С --allow-model-calls запускается только CLI, заранее записанный через register-session, и только точный ID.
Пустая группа процессов не завершает попытку и не снимает блокировку. Второе письмо той же сессии само не стартует.
`)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
