// Package localui serves a read-only local front end. It cannot launch agents,
// mutate the queue or expose arbitrary control commands to the browser.
package localui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/diagnostics"
)

//go:embed assets/*
var assets embed.FS

type caller func(context.Context, control.Request) (control.Response, error)

// Run binds only IPv4 loopback, with a fresh capability for each invocation.
// The URL is private. The caller must not publish or log it to a shared service.
func Run(ctx context.Context, root string, port int, out io.Writer) error {
	if port < 0 || port > 65535 {
		return errors.New("ui port must be 0..65535")
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	token := hex.EncodeToString(secret)
	ln, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return err
	}
	defer ln.Close()
	host := ln.Addr().String()
	call := func(ctx context.Context, r control.Request) (control.Response, error) {
		return control.CallContext(ctx, root, r)
	}
	h := newHandler(host, token, call, func(ctx context.Context) any {
		r := diagnostics.Collect(ctx, root)
		// Avoid copying private root and profile paths into UI discovery.
		type candidate struct {
			Provider string `json:"provider"`
			Name     string `json:"name"`
			State    string `json:"state"`
		}
		list := []candidate{}
		for _, a := range r.Agents {
			list = append(list, candidate{a.Provider, a.Name, a.CLI.State})
		}
		return struct {
			OS        string      `json:"os"`
			CheckedAt string      `json:"checked_at"`
			Agents    []candidate `json:"agents"`
		}{r.OS, r.CheckedAt, list}
	})
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 7 * time.Second, IdleTimeout: 20 * time.Second, MaxHeaderBytes: 8192, ErrorLog: log.New(io.Discard, "", 0)}
	stop := context.AfterFunc(ctx, func() { _ = srv.Close() })
	defer stop()
	if _, err := fmt.Fprintf(out, "http://%s/#%s\n", host, token); err != nil {
		return err
	}
	err = srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func newHandler(host, token string, call caller, discover func(context.Context) any) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		if r.Host != host {
			http.Error(w, "invalid host", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+host {
			http.Error(w, "invalid origin", http.StatusForbidden)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			http.Error(w, "cross-site request refused", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "read-only interface", http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			files := map[string]string{"/": "index.html", "/app.js": "app.js", "/app.css": "app.css", "/design-v1.css": "design-v1.css"}
			name, ok := files[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			data, err := assets.ReadFile("assets/" + name)
			if err != nil {
				http.Error(w, "asset unavailable", 500)
				return
			}
			switch {
			case strings.HasSuffix(name, ".css"):
				w.Header().Set("Content-Type", "text/css; charset=utf-8")
			case strings.HasSuffix(name, ".js"):
				w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			default:
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
			}
			_, _ = w.Write(data)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			http.Error(w, "access denied", http.StatusUnauthorized)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if r.URL.Path == "/api/discovery" {
			writeJSON(w, discover(ctx))
			return
		}
		req := control.Request{}
		switch r.URL.Path {
		case "/api/status":
			req.Cmd = "status"
		case "/api/overview":
			req.Cmd = "overview"
		case "/api/timeline":
			req.Cmd = "timeline"
			req.Alias = r.URL.Query().Get("alias")
			req.Before = r.URL.Query().Get("before")
			req.Limit = 50
			if v := r.URL.Query().Get("limit"); v != "" {
				n, err := strconv.Atoi(v)
				if err != nil {
					http.Error(w, "invalid limit", 400)
					return
				}
				req.Limit = n
			}
			if strings.TrimSpace(req.Alias) == "" || len(req.Alias) > 4096 || len(req.Before) > 4096 || req.Limit < 1 || req.Limit > 100 {
				http.Error(w, "invalid page", 400)
				return
			}
		default:
			http.NotFound(w, r)
			return
		}
		response, err := call(ctx, req)
		if err != nil || !response.OK {
			http.Error(w, "local client unavailable or request rejected", http.StatusServiceUnavailable)
			return
		}
		switch req.Cmd {
		case "status":
			if response.Status == nil {
				http.Error(w, "missing status", 502)
				return
			}
			status := *response.Status
			status.Detail = ""
			writeJSON(w, status)
		case "overview":
			writeRaw(w, response.Overview)
		case "timeline":
			writeRaw(w, response.Timeline)
		}
	})
}

func writeRaw(w http.ResponseWriter, raw json.RawMessage) {
	if len(raw) == 0 || !json.Valid(raw) {
		http.Error(w, "invalid client response", 502)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(raw)
}

func writeJSON(w http.ResponseWriter, value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "invalid response", 500)
		return
	}
	writeRaw(w, raw)
}
