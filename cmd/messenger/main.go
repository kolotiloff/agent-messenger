package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/diagnostics"
	"github.com/kolotiloff/agent-messenger/internal/distribution"
	"github.com/kolotiloff/agent-messenger/internal/localui"
	"github.com/kolotiloff/agent-messenger/internal/mcp"
	"github.com/kolotiloff/agent-messenger/internal/platform"
	"github.com/kolotiloff/agent-messenger/internal/supervisor"
)

func main() {
	root, args := splitArgs(os.Args[1:])
	if len(args) == 0 && runtime.GOOS == "windows" && len(os.Args) == 1 {
		// Started with a double click: install the program for this user
		// if this is a download (T-053), open the window and keep any
		// error on screen until the owner has read it.
		if root == "" && selfInstall(os.Stdout) {
			return
		}
		if err := runOpen(root, nil, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, "Нажмите Enter, чтобы закрыть это окно.")
			_, _ = fmt.Scanln()
			os.Exit(1)
		}
		return
	}
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	if args[0] == "task-mcp" {
		if err := mcp.TaskMain(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if args[0] == "update-fetch" {
		if err := runUpdateFetch(root, args[1:]); err != nil {
			fail(err)
		}
		return
	}
	if args[0] == "open" {
		if err := runOpen(root, args[1:], os.Stdout); err != nil {
			fail(err)
		}
		return
	}
	if args[0] == "uninstall" {
		// «Удалить» in Windows «Приложения» (T-053).
		if root == "" {
			var err error
			if root, err = supervisor.DesktopRoot(); err != nil {
				fail(err)
			}
		}
		err := runUninstall(root, os.Stdout)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		if runtime.GOOS == "windows" {
			fmt.Fprintln(os.Stderr, "Нажмите Enter, чтобы закрыть это окно.")
			_, _ = fmt.Scanln()
		}
		if err != nil {
			os.Exit(1)
		}
		return
	}
	if args[0] == "desktop" {
		if err := runDesktop(root, args[1:]); err != nil {
			fail(err)
		}
		return
	}
	if args[0] == "version" {
		if err := run(root, args); err != nil {
			fail(err)
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
	if handled, err := runTaskCommand(root, args); handled {
		return err
	}
	switch args[0] {
	case "version":
		if len(args) != 1 {
			return errors.New("version takes no arguments")
		}
		raw, err := json.Marshal(buildinfo.Current())
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	case "install":
		if runtime.GOOS == "windows" {
			if len(args) != 1 {
				return errors.New("install takes no arguments on Windows")
			}
			return runWinInstall(os.Stdout)
		}
		fs := flag.NewFlagSet("install", flag.ContinueOnError)
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		prefix := fs.String("prefix", filepath.Join(home, ".local"), "absolute user installation directory")
		if err = fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("install takes only --prefix")
		}
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		installed, err := distribution.Install(exe, *prefix, buildinfo.Current())
		if err != nil {
			return err
		}
		fmt.Println("Installed:", installed)
		fmt.Println("No client, model, service or login was started. Add the bin directory to PATH if needed.")
		return nil
	case "ui":
		fs := flag.NewFlagSet("ui", flag.ContinueOnError)
		port := fs.Int("port", 0, "loopback port; 0 chooses a free port")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 || *port < 0 || *port > 65535 {
			return errors.New("ui takes only --port 0..65535")
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		return localui.Run(ctx, root, *port, os.Stdout)
	case "check-cli":
		fs := flag.NewFlagSet("check-cli", flag.ContinueOnError)
		provider := fs.String("provider", "", "claude|codex|cursor")
		cli := fs.String("cli", "", "explicit absolute executable path")
		environment := fs.String("environment", "", "explicit existing profile directory")
		asJSON := fs.Bool("json", false, "machine-readable check report")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("лишние аргументы check-cli")
		}
		report, err := diagnostics.CheckCLI(context.Background(), *provider, *cli, *environment)
		if err != nil {
			return err
		}
		if *asJSON {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(report)
		}
		fmt.Printf("%s: версия=%s (%s), вход=%s (%s).\n", adapters.ProviderTitle(report.Provider), report.Version.Value, report.Version.State, report.Authentication.State, report.Authentication.Reason)
		fmt.Println("Это статус выбранного CLI и профиля с текущим окружением. Готовность сессии и доступность модели не проверялись. Вход и выход из аккаунта не выполнялись.")
		return nil
	case "doctor":
		fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
		asJSON := fs.Bool("json", false, "machine-readable diagnostic report")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("лишние аргументы doctor")
		}
		report := diagnostics.Collect(context.Background(), root)
		if *asJSON {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(report)
		}
		diagnostics.WriteText(os.Stdout, report)
		return nil
	case "start":
		if _, err := serveOptions(args[1:]); err != nil {
			return err
		}
		pid, logPath, err := startBackground(root, args[1:])
		if err != nil {
			return err
		}
		fmt.Printf("background_pid=%d paused=true\nlog=%s\n", pid, logPath)
		fmt.Println("Клиент работает в фоне. Терминал можно закрыть. Проверьте status; чтобы запускать задачи, нужен отдельный resume. shutdown останавливает клиент и сохраняет защитные резервы.")
		return nil
	case "_background-serve":
		return runBackgroundChild(root, args[1:])
	case "_remote-update":
		return runRemoteUpdate(root, args[1:])
	case "recover-missing-root":
		fs := flag.NewFlagSet("recover-missing-root", flag.ContinueOnError)
		confirm := fs.String("confirm", "", "preserve-uncertain")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("лишние аргументы recover-missing-root")
		}
		message, err := supervisor.RecoverMissingRoots(*confirm)
		if errors.Is(err, platform.ErrAlreadyRunning) {
			return errors.New("клиент уже запущен; остановите его перед снятием резерва")
		}
		if err != nil {
			return fmt.Errorf("резерв не снят: %w", err)
		}
		fmt.Println(message)
		return nil
	case "serve":
		options, err := serveOptions(args[1:])
		if err != nil {
			return err
		}
		err = supervisor.Run(context.Background(), root, options)
		if errors.Is(err, platform.ErrAlreadyRunning) {
			return errors.New("клиент уже запущен; команды нужно отправлять ему, а не запускать второй экземпляр")
		}
		if errors.Is(err, platform.ErrUnsupported) {
			return fmt.Errorf("операция этой ОС не поддерживается и не выполнена: %w", err)
		}
		return err
	case "status", "pause", "resume", "shutdown":
		if len(args) != 1 {
			return fmt.Errorf("%s не принимает аргументов", args[0])
		}
		response, err := control.Call(root, control.Request{Cmd: args[0]})
		if err != nil {
			if args[0] == "status" {
				// The registry is readable without the client.
				diagnostics.WriteReservations(os.Stdout, supervisor.ReadReservations())
			}
			return err
		}
		if !response.OK {
			return errors.New(response.Error)
		}
		printResponse(response)
		return nil
	case "mcp", "timeline", "overview", "inspect", "enqueue", "resolve", "stop-job", "register-session",
		"create-claude-session", "create-codex-session", "setup-codex", "recover-stream", "recover-codex", "recover-failed":
		return fmt.Errorf("команда %s относилась к старому режиму постоянных сессий и удалена в этой версии (T-028). Работа идёт через проекты и задачи. Старые резервы и история сохранены; их показывает status", args[0])
	case "help":
		usage()
		return nil
	default:
		usage()
		return errors.New("неизвестная команда")
	}
}

func printResponse(response control.Response) {
	if response.Status != nil {
		fmt.Printf("paused=%t cli_launch=%s broker=%s pid=%d\n",
			response.Status.Paused, response.Status.CLILaunch, response.Status.Broker, response.Status.Supervisor)
		if response.Status.Detail != "" {
			fmt.Println(response.Status.Detail)
		}
		if response.Status.Reservations != nil {
			diagnostics.WriteReservations(os.Stdout, *response.Status.Reservations)
		}
	}
}

// serveOptions reads the serve and start flags. --claude-stream is accepted
// and ignored: the persistent mode it turned on was removed (T-028), and an
// older program may still pass it when it starts this client.
func serveOptions(args []string) (supervisor.AdapterLauncher, error) {
	var options supervisor.AdapterLauncher
	for _, arg := range args {
		switch {
		case arg == "--allow-tasks":
			options.AllowTasks = true
		case arg == "--claude-stream":
		case arg == "--allow-model-calls" || arg == "--only-session" || strings.HasPrefix(arg, "--only-session="):
			return options, fmt.Errorf("флаг %s относился к старому режиму постоянных сессий и удалён (T-028); задачи разрешает --allow-tasks", arg)
		default:
			return options, fmt.Errorf("неизвестный флаг serve: %s", arg)
		}
	}
	return options, nil
}

func splitArgs(args []string) (string, []string) {
	root := ""
	var rest []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--root" && i+1 < len(args) {
			root = args[i+1]
			// A relative root would point elsewhere once a child process or the
			// coordinator runs from another folder.
			if abs, err := filepath.Abs(root); err == nil && root != "" {
				root = abs
			}
			i++
			continue
		}
		rest = append(rest, args[i])
	}
	return root, rest
}

func usage() {
	fmt.Fprintf(os.Stderr, `Agent Messenger — локальный клиент и интерфейс.

--root стоит перед командой. Один пользователь может держать только один serve,
даже с другим каталогом данных.

  messenger version
  messenger [--root КАТАЛОГ] open     (окно в браузере: Linux и Windows; на Windows — двойной щелчок)
  messenger [--root КАТАЛОГ] desktop  (внутренний запуск окна; stdin — канал времени жизни)
  messenger install [--prefix АБСОЛЮТНЫЙ_КАТАЛОГ]
  messenger uninstall                    (Windows: удалить программу, данные остаются)
  messenger --root КАТАЛОГ serve [--allow-tasks]
  messenger --root КАТАЛОГ start [--allow-tasks]
  messenger --root КАТАЛОГ doctor [--json]
  messenger check-cli --provider claude|codex|cursor --cli ФАЙЛ --environment ПРОФИЛЬ [--json]
  messenger --root КАТАЛОГ project-add --name ИМЯ --folder ПАПКА
  messenger --root КАТАЛОГ project-folder --name ИМЯ --folder ПАПКА
  messenger --root КАТАЛОГ projects
  messenger --root КАТАЛОГ agent-set --provider claude|codex|cursor --cli ФАЙЛ [--environment ПРОФИЛЬ] [--profile worker|messenger-only]
  messenger --root КАТАЛОГ tasks [--project ИМЯ]
  messenger --root КАТАЛОГ task-answer --task ID --question ID --op ID --text ТЕКСТ
  messenger --root КАТАЛОГ task-cancel --task ID
  messenger --root КАТАЛОГ coordinator --project ИМЯ
  messenger recover-missing-root --confirm preserve-uncertain
  messenger --root КАТАЛОГ status
  messenger --root КАТАЛОГ ui [--port 0]
  messenger --root КАТАЛОГ pause
  messenger --root КАТАЛОГ resume
  messenger --root КАТАЛОГ shutdown

pause — новые запуски задач не начинаются, работающий исполнитель не останавливается.
doctor — показывает наличие CLI в PATH, статус доступного клиента и резервы; CLI/модели не запускает, вход не проверяет, блокировки не меняет. Код 0 означает сформированный отчёт, а не готовность агентов.
start — запускает клиент в фоне, всегда на паузе; не устанавливает службу или автозапуск. Закрытие терминала не останавливает клиент. Повторный start не заменяет живой экземпляр и не меняет его разрешения.
shutdown — задачи и история остаются, работающий исполнитель не убивается. Его резерв сохраняется.
--allow-tasks: клиент запускает исполнителей задач (модель задачи, D-021). Без флага и на паузе задачи только записываются, ничего не запускается.
status показывает резервы в active-runs.json. Резервы старого режима постоянных сессий (удалён в T-028) сохраняются как есть и только показываются; эта версия их не снимает.
recover-missing-root — только при остановленном клиенте: снимает резерв, чей каталог данных удалён, после доказанной новой загрузки ОС. Модель не вызывается.
`)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
