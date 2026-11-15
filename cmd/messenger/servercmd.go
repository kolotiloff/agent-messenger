package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/broker"
	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/platform"
	"github.com/kolotiloff/agent-messenger/internal/supervisor"
)

// T-036, D-138: a server on a computer without a screen, such as a VPS, is
// run and managed from the terminal. `server run` is the client for a
// system service; the other commands talk to it like the window does.

// serverCallTimeout is above the client's own bound for a server action
// (18 s), so a slow answer is still read.
const serverCallTimeout = 25 * time.Second

func runServerCommand(root string, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("укажите действие: run, enable, invite, status, disable или revoke")
	}
	switch args[0] {
	case "run":
		if len(args) != 1 {
			return errors.New("server run не принимает аргументов")
		}
		return runServerService(root)
	case "enable":
		fs := flag.NewFlagSet("server enable", flag.ContinueOnError)
		fs.SetOutput(out)
		internet := fs.Bool("internet", false, "доступ из интернета")
		lan := fs.Bool("lan", false, "доступ из локальной сети")
		port := fs.Int("port", broker.DefaultPort, "порт TCP")
		public := fs.String("public", "", "внешний адрес этого компьютера")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("лишние аргументы: %s", strings.Join(fs.Args(), " "))
		}
		if *internet && *lan {
			return errors.New("выберите одно: --internet или --lan")
		}
		scope := broker.ScopeLocal
		switch {
		case *internet:
			scope = broker.ScopeInternet
		case *lan:
			scope = broker.ScopeLAN
		}
		address := strings.TrimSpace(*public)
		if scope == broker.ScopeInternet && address == "" {
			ip, err := outboundPublicIP()
			if err != nil {
				return err
			}
			address = ip
		}
		if scope != broker.ScopeInternet && address != "" {
			return errors.New("--public нужен только вместе с --internet")
		}
		status, err := serverCall(root, "enable", map[string]any{"scope": scope, "port": *port, "public_address": address})
		if err != nil {
			return err
		}
		fmt.Fprintln(out, "Сервер запущен.")
		printServerStatus(out, status)
		return nil
	case "invite":
		fs := flag.NewFlagSet("server invite", flag.ContinueOnError)
		fs.SetOutput(out)
		address := fs.String("address", "", "адрес сервера в приглашении (по умолчанию первый)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("лишние аргументы: %s", strings.Join(fs.Args(), " "))
		}
		if *address == "" {
			status, err := serverCall(root, "status", nil)
			if err != nil {
				return err
			}
			if !status.Host.Running {
				return errors.New("сервер не запущен; сначала server enable")
			}
			if len(status.Host.Addresses) == 0 {
				return errors.New("у сервера нет адреса для приглашения")
			}
			*address = status.Host.Addresses[0]
		}
		raw, err := serverRaw(root, "invite", map[string]any{"address": *address})
		if err != nil {
			return err
		}
		var code broker.InviteCode
		if err := json.Unmarshal(raw, &code); err != nil || code.Code == "" {
			return errors.New("клиент вернул непонятное приглашение")
		}
		fmt.Fprintln(out, "Приглашение (одно устройство, действует до "+localTime(code.ExpiresAt)+"):")
		fmt.Fprintln(out)
		fmt.Fprintln(out, code.Code)
		fmt.Fprintln(out)
		fmt.Fprintln(out, "На компьютере: окно Messenger → «Сервер» → «Подключиться», вставьте код целиком.")
		return nil
	case "status":
		if len(args) != 1 {
			return errors.New("server status не принимает аргументов")
		}
		status, err := serverCall(root, "status", nil)
		if err != nil {
			return err
		}
		printServerStatus(out, status)
		return nil
	case "disable":
		if len(args) != 1 {
			return errors.New("server disable не принимает аргументов")
		}
		status, err := serverCall(root, "disable", nil)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, "Сервер остановлен.")
		printServerStatus(out, status)
		return nil
	case "revoke":
		fs := flag.NewFlagSet("server revoke", flag.ContinueOnError)
		fs.SetOutput(out)
		device := fs.String("device", "", "ID устройства из server status")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *device == "" || fs.NArg() != 0 {
			return errors.New("укажите --device ID из server status")
		}
		status, err := serverCall(root, "revoke", map[string]any{"device": *device})
		if err != nil {
			return err
		}
		fmt.Fprintln(out, "Устройство отключено.")
		printServerStatus(out, status)
		return nil
	}
	return fmt.Errorf("неизвестное действие server: %s", args[0])
}

// runServerService runs the client in the foreground until SIGTERM or
// Ctrl+C, for a system service. It runs no tasks and no models, and it
// refuses update requests from the network: the service is updated by
// running the installer again.
func runServerService(root string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	err := supervisor.RunWithOptions(ctx, root, supervisor.AdapterLauncher{}, supervisor.RunOptions{StartPaused: true})
	if errors.Is(err, platform.ErrAlreadyRunning) {
		return errors.New("клиент Messenger уже запущен у этого пользователя; остановите его командой shutdown с его --root и повторите")
	}
	return err
}

type serverStatus struct {
	broker.Status
}

func serverRaw(root, action string, payload map[string]any) (json.RawMessage, error) {
	req := control.Request{Cmd: "server", Action: action}
	if payload != nil {
		raw, _ := json.Marshal(payload)
		req.Text = string(raw)
	}
	ctx, cancel := context.WithTimeout(context.Background(), serverCallTimeout)
	defer cancel()
	response, err := control.CallContext(ctx, root, req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, errors.New("клиент не ответил вовремя; результат неизвестен. Проверьте server status, прежде чем повторять")
		}
		return nil, fmt.Errorf("клиент Messenger не запущен или не отвечает (%v). На сервере: systemctl status agent-messenger", err)
	}
	if !response.OK {
		return nil, errors.New(response.Error)
	}
	return response.Overview, nil
}

func serverCall(root, action string, payload map[string]any) (serverStatus, error) {
	var status serverStatus
	raw, err := serverRaw(root, action, payload)
	if err != nil {
		return status, err
	}
	if json.Unmarshal(raw, &status) != nil {
		return status, errors.New("клиент вернул непонятное состояние сервера")
	}
	return status, nil
}

func printServerStatus(out io.Writer, s serverStatus) {
	h := s.Host
	switch {
	case h.Running:
		fmt.Fprintf(out, "Сервер: работает, доступ %s, порт %d\n", scopeName(h.Scope), h.Port)
	case h.Enabled:
		fmt.Fprintln(out, "Сервер: включён, но не работает")
	default:
		fmt.Fprintln(out, "Сервер: выключен")
	}
	if h.Error != "" {
		fmt.Fprintln(out, "Ошибка: "+h.Error)
	}
	if h.Running {
		if len(h.Addresses) > 0 {
			fmt.Fprintln(out, "Адреса: "+strings.Join(h.Addresses, ", "))
		}
		fmt.Fprintln(out, "Отпечаток: "+h.Fingerprint)
		active := 0
		for _, d := range h.Devices {
			if d.RevokedAt == "" {
				active++
			}
		}
		fmt.Fprintf(out, "Устройства: %d\n", active)
		for _, d := range h.Devices {
			if d.RevokedAt != "" {
				continue
			}
			seen := "ещё не выходил на связь"
			if d.LastSeen != "" {
				seen = "на связи " + localTime(d.LastSeen)
			}
			fmt.Fprintf(out, "  %s  %s  (%s)\n", d.ID, d.Name, seen)
		}
	}
	if s.Member.Connected {
		fmt.Fprintln(out, "Этот компьютер подключён к серверу "+s.Member.Address)
	}
}

func scopeName(scope string) string {
	switch scope {
	case broker.ScopeInternet:
		return "из интернета"
	case broker.ScopeLAN:
		return "из локальной сети"
	}
	return "только с этого компьютера"
}

func localTime(stamp string) string {
	t, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		return stamp
	}
	return t.Local().Format("2006-01-02 15:04")
}

// outboundPublicIP is the address of the interface this computer reaches
// the internet through, when that address is public, as on most VPS. No
// packet is sent: a UDP "connection" only picks the route.
func outboundPublicIP() (string, error) {
	for _, target := range []string{"192.0.2.1:9", "[2001:db8::1]:9"} {
		conn, err := net.Dial("udp", target)
		if err != nil {
			continue
		}
		ip := conn.LocalAddr().(*net.UDPAddr).IP
		conn.Close()
		if publicIP(ip) {
			return ip.String(), nil
		}
	}
	return "", errors.New("не удалось узнать внешний адрес этого компьютера: у сетевой карты нет публичного адреса (компьютер за роутером или NAT). Укажите его сами: --public АДРЕС")
}

func publicIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return false
	}
	// Carrier-grade NAT (100.64.0.0/10) is not reachable from outside.
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1]&0xc0 == 64 {
		return false
	}
	// Documentation ranges never belong to a real interface.
	for _, block := range []string{"192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/32"} {
		_, n, _ := net.ParseCIDR(block)
		if n.Contains(ip) {
			return false
		}
	}
	return true
}
