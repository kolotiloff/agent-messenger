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
	"time"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
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
	if args[0] == "mcp" {
		if err := mcp.Main(args[1:]); err != nil {
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
		install := distribution.Install
		if link, _ := installedLink(*prefix); link != "" {
			// An update of an existing installation goes next to the
			// running version, as from the window (D-106): it needs no
			// stopped client (owner, 2026-11-12: install refused while the
			// client ran).
			install = distribution.InstallUpdate
		}
		installed, err := install(exe, *prefix, buildinfo.Current())
		if err != nil {
			return err
		}
		fmt.Println("Installed:", installed)
		menuEntryAfterInstall(installed)
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
	case "timeline":
		fs := flag.NewFlagSet("timeline", flag.ContinueOnError)
		alias := fs.String("alias", "", "session mailbox alias")
		limit := fs.Int("limit", 50, "page size 1..100")
		before := fs.String("before", "", "older than this Messenger request ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 || strings.TrimSpace(*alias) == "" || *limit < 1 || *limit > 100 {
			return errors.New("timeline requires --alias and --limit 1..100; no positional arguments")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		response, err := control.CallContext(ctx, root, control.Request{Cmd: "timeline", Alias: *alias, Limit: *limit, Before: *before})
		if err != nil {
			return err
		}
		if !response.OK {
			return errors.New(response.Error)
		}
		fmt.Println(string(response.Timeline))
		return nil
	case "overview":
		if len(args) != 1 {
			return errors.New("overview takes no arguments")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		response, err := control.CallContext(ctx, root, control.Request{Cmd: "overview"})
		if err != nil {
			return err
		}
		if !response.OK {
			return errors.New(response.Error)
		}
		fmt.Println(string(response.Overview))
		return nil
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
		if _, err := serveAllowsModels(args[1:]); err != nil {
			return err
		}
		pid, logPath, err := startBackground(root, args[1:])
		if err != nil {
			return err
		}
		fmt.Printf("background_pid=%d paused=true\nlog=%s\n", pid, logPath)
		fmt.Println("Клиент работает в фоне. Терминал можно закрыть. Проверьте status; для доставки нужен отдельный resume. shutdown останавливает клиент и сохраняет защитные резервы.")
		return nil
	case "_background-serve":
		return runBackgroundChild(root, args[1:])
	case "_remote-update":
		return runRemoteUpdate(root, args[1:])
	case "recover-stream":
		fs := flag.NewFlagSet("recover-stream", flag.ContinueOnError)
		alias := fs.String("alias", "", "exact session alias")
		runID := fs.String("run", "", "exact StreamRuns id from inspect")
		confirm := fs.String("confirm", "", "preserve-uncertain")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("лишние аргументы recover-stream")
		}
		message, err := supervisor.RecoverStream(root, *alias, *runID, *confirm)
		if err != nil {
			return fmt.Errorf("восстановление канала не завершено: %w", err)
		}
		fmt.Println(message)
		return nil
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
	case "recover-codex":
		fs := flag.NewFlagSet("recover-codex", flag.ContinueOnError)
		alias := fs.String("alias", "", "exact session alias")
		confirm := fs.String("confirm", "", "preserve-uncertain")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("лишние аргументы recover-codex")
		}
		message, err := supervisor.RecoverCodex(root, *alias, *confirm)
		if err != nil {
			return fmt.Errorf("восстановление сессии Codex не завершено: %w", err)
		}
		fmt.Println(message)
		return nil
	case "recover-failed":
		message, err := supervisor.RecoverFailed(request(args).Confirmation)
		if errors.Is(err, platform.ErrAlreadyRunning) {
			return errors.New("клиент уже запущен; восстановление не меняет резерв, пока serve работает")
		}
		if errors.Is(err, core.ErrBadConfirmation) {
			return errors.New("нужна точная фраза --confirm actions-may-repeat; резерв не изменён")
		}
		if err != nil {
			return fmt.Errorf("восстановление не завершено, файл резерва не заменён: %w", err)
		}
		fmt.Println(message)
		return nil
	case "serve":
		options, err := serveOptions(args[1:])
		if err != nil {
			return err
		}
		refreshMenuEntry(root)
		err = supervisor.Run(context.Background(), root, options)
		if errors.Is(err, platform.ErrAlreadyRunning) {
			return errors.New("клиент уже запущен; команды нужно отправлять ему, а не запускать второй экземпляр")
		}
		if errors.Is(err, platform.ErrUnsupported) {
			return fmt.Errorf("операция этой ОС не поддерживается и не выполнена: %w", err)
		}
		return err
	case "status", "pause", "resume", "stop-job", "shutdown", "enqueue", "inspect", "resolve", "register-session", "create-claude-session", "create-codex-session":
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
		if response.Status.StreamState != "" {
			fmt.Printf("stream_state=%s\n", response.Status.StreamState)
		}
		if response.Status.LaunchSession != "" {
			fmt.Printf("launch_session=%s\n", response.Status.LaunchSession)
		}
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
	options, err := serveOptions(args)
	return options.AllowModels, err
}

func serveOptions(args []string) (supervisor.AdapterLauncher, error) {
	var options supervisor.AdapterLauncher
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--claude-stream" {
			options.ClaudeStream = true
			continue
		}
		if arg == "--allow-model-calls" {
			options.AllowModels = true
			continue
		}
		if arg == "--allow-tasks" {
			options.AllowTasks = true
			continue
		}
		if arg == "--only-session" || strings.HasPrefix(arg, "--only-session=") {
			value := strings.TrimPrefix(arg, "--only-session=")
			if arg == "--only-session" {
				i++
				if i >= len(args) {
					return options, errors.New("--only-session requires an alias")
				}
				value = args[i]
			}
			if options.OnlySession != "" || strings.TrimSpace(value) != value || value == "" || len(value) > 128 || strings.HasPrefix(value, "--") || strings.ContainsAny(value, "\r\n\x00") {
				return options, errors.New("invalid or repeated --only-session")
			}
			options.OnlySession = value
			continue
		}
		return options, fmt.Errorf("неизвестный флаг serve: %s", arg)
	}
	if options.OnlySession != "" && !options.ClaudeStream {
		return options, errors.New("--only-session currently requires --claude-stream")
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
даже с другим каталогом данных. Реальный CLI запускается только после register-session.

  messenger version
  messenger [--root КАТАЛОГ] open     (окно в браузере: Linux и Windows; на Windows — двойной щелчок)
  messenger [--root КАТАЛОГ] desktop  (внутренний запуск окна; stdin — канал времени жизни)
  messenger install [--prefix АБСОЛЮТНЫЙ_КАТАЛОГ]
  messenger uninstall                    (Windows: удалить программу, данные остаются)
  messenger --root КАТАЛОГ serve [--allow-model-calls] [--claude-stream] [--allow-tasks]
  messenger --root КАТАЛОГ start [--allow-model-calls] [--claude-stream] [--allow-tasks]
  messenger --root КАТАЛОГ doctor [--json]
  messenger check-cli --provider claude|codex|cursor --cli ФАЙЛ --environment ПРОФИЛЬ [--json]
  messenger --root КАТАЛОГ create-claude-session --environment КАТАЛОГ --alias ИМЯ --cli ФАЙЛ --workspace КАТАЛОГ
  messenger --root КАТАЛОГ create-codex-session --environment УПРАВЛЯЕМЫЙ_ПРОФИЛЬ --alias ИМЯ --cli ФАЙЛ
  messenger recover-failed --confirm actions-may-repeat
  messenger --root КАТАЛОГ recover-stream --alias ИМЯ --run ID_ЗАПУСКА --confirm preserve-uncertain
  messenger --root КАТАЛОГ project-add --name ИМЯ --folder ПАПКА
  messenger --root КАТАЛОГ project-folder --name ИМЯ --folder ПАПКА
  messenger --root КАТАЛОГ projects
  messenger --root КАТАЛОГ agent-set --provider claude|codex|cursor --cli ФАЙЛ [--environment ПРОФИЛЬ] [--profile worker|messenger-only]
  messenger --root КАТАЛОГ tasks [--project ИМЯ]
  messenger --root КАТАЛОГ task-answer --task ID --question ID --op ID --text ТЕКСТ
  messenger --root КАТАЛОГ task-cancel --task ID
  messenger --root КАТАЛОГ coordinator --project ИМЯ
  messenger --root КАТАЛОГ coordinator-watch --project ИМЯ --after КУРСОР [--minutes 30]
  messenger --root КАТАЛОГ recover-codex --alias ИМЯ --confirm preserve-uncertain
  messenger recover-missing-root --confirm preserve-uncertain
  messenger --root КАТАЛОГ register-session --provider claude|codex|cursor --environment КАТАЛОГ --conversation-id ID --alias ИМЯ --cli ФАЙЛ [--workspace КАТАЛОГ]
  messenger --root КАТАЛОГ status
  messenger --root КАТАЛОГ overview
  messenger --root КАТАЛОГ ui [--port 0]
  messenger --root КАТАЛОГ timeline --alias ИМЯ [--limit 50] [--before ID_ПИСЬМА]
  messenger --root КАТАЛОГ pause
  messenger --root КАТАЛОГ resume
  messenger --root КАТАЛОГ stop-job
  messenger --root КАТАЛОГ shutdown
  messenger --root КАТАЛОГ enqueue --alias ИМЯ --id ID --from ОТ --text ТЕКСТ
  messenger --root КАТАЛОГ inspect
  messenger --root КАТАЛОГ resolve fail|accept|retry --alias ИМЯ [--confirm actions-may-repeat]

coordinator-watch — фоновое ожидание координатора: выходит, когда у задач проекта появились события после курсора, или через 30 минут без них.
pause — новые задания не резервируются, активный процесс не останавливается.
doctor — показывает наличие CLI в PATH и статус доступного клиента; CLI/модели не запускает, вход не проверяет, блокировки не меняет. Код 0 означает сформированный отчёт, а не готовность агентов.
start — запускает клиент в фоне, всегда на паузе; не устанавливает службу или автозапуск. Закрытие терминала не останавливает клиент. Повторный start не заменяет живой экземпляр и не меняет его разрешения.
stop-job — сначала отзывает MCP, затем сигналит группе только если pid, время старта и группа всё ещё принадлежат той же попытке.
shutdown — очередь остаётся, активный процесс не убивается. Следующий serve сразу отзывает старый MCP и не снимает блокировку сессии.
recover-failed снимает только неудачную попытку после явного подтверждения, если записанный процесс и его группа уже не работают. История остаётся. Модель при этом не запускается. Пустой, повреждённый или неполный резерв эта команда не удаляет.
Без --allow-model-calls serve не запускает CLI и не тратит токены.
С --allow-model-calls запускается только CLI, заранее записанный через register-session, и только точный ID.
Пустая группа процессов не снимает блокировку. В обычном режиме второе письмо той же сессии не стартует.
--claude-stream: один постоянный Claude, письма по очереди в том же процессе; другие провайдеры остаются в очереди.
--only-session ИМЯ: в режиме --claude-stream обрабатывается только эта сессия. Остальные очереди не выбираются. Сам флаг вызовы модели не разрешает.
--allow-tasks: клиент запускает исполнителей задач (модель задачи, D-021). Без флага и на паузе задачи только записываются, ничего не запускается.
create-claude-session создаёт новый точный ID без модели, для последующего запуска с --claude-stream.
Постоянный процесс сохраняет блокировку после stop/shutdown/restart. Незавершённый ход не повторяется автоматически.
inspect показывает StreamRuns и Verifications: успешный ход не является выходом процесса.
recover-stream: только при остановленном serve, после доказанной новой загрузки того же Mac и хотя бы одного проверенного ответа. Старые резервы без заранее записанной идентичности загрузки не подходят. История и неопределённые письма сохраняются, автоматического повтора нет; очередь остаётся на паузе. Модель не вызывается.
`)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
