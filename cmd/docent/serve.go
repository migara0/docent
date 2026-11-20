package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/migara0/docent/internal/config"
	"github.com/migara0/docent/internal/devnotes"
	"github.com/migara0/docent/internal/help"
	"github.com/migara0/docent/internal/library"
	"github.com/migara0/docent/internal/server"
)

const defaultAddr = "127.0.0.1:8787"

type serveOptions struct {
	addr   string
	lan    bool
	noOpen bool
}

func (o *serveOptions) bind(f *pflag.FlagSet) {
	f.StringVar(&o.addr, "addr", "", "address to listen on (default "+defaultAddr+"; the next free port is used if it is busy)")
	f.BoolVar(&o.lan, "lan", false, "listen on all network interfaces so a phone or another computer can read (read-only; the folder picker is disabled)")
	f.BoolVar(&o.noOpen, "no-open", false, "do not open the browser")
}

func newServe() *cobra.Command {
	var o serveOptions
	c := &cobra.Command{
		Use:   "serve [folder-or-file...]",
		Short: "Start the app (opens the browser)",
		Long:  "Starts Docent. Any folders or files given are opened for this run only; your registered libraries are always loaded.",
		RunE:  func(cmd *cobra.Command, args []string) error { return runServe(o, args) },
	}
	o.bind(c.Flags())
	return c
}

// serverInfo is written while a server runs so other commands (docent open) can find it.
type serverInfo struct {
	URL     string `json:"url"`
	PID     int    `json:"pid"`
	Version string `json:"version"`
	LAN     bool   `json:"lan"`
}

func runServe(o serveOptions, open []string) error {
	paths, err := config.ResolvePaths()
	if err != nil {
		return err
	}
	store := config.NewStore(paths)
	if running, ok := findRunning(paths); ok {
		fmt.Printf("Docent is already running at %s\n", running.URL)
		for _, p := range open {
			if _, err := postOpen(running.URL, p); err != nil {
				return err
			}
		}
		if !o.noOpen {
			openBrowser(running.URL)
		}
		return nil
	}

	_ = os.RemoveAll(filepath.Join(paths.CacheDir, "drops")) // copies of dropped files left by a crash; no server is running, so none are in use
	debug.SetGCPercent(40)                                   // keep the heap close to what is live: this is a background app that should stay small
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	mgr := library.NewManager(ctx, store, library.WithHooks(devnotes.NewHooks()))
	if err := mgr.Sync(); err != nil {
		return err
	}
	if err := help.Write(filepath.Join(paths.CacheDir, "help")); err != nil {
		slog.Warn("could not write the built-in guide", "err", err)
	} else {
		mgr.OpenHelp(filepath.Join(paths.CacheDir, "help"))
	}
	var first *library.Source
	for _, p := range open {
		s, err := mgr.Open(p)
		if err != nil {
			return err
		}
		if first == nil {
			first = s
		}
	}

	addr, explicit := o.addr, o.addr != ""
	if !explicit {
		addr = defaultAddr
		if o.lan {
			addr = "0.0.0.0:8787"
		}
	}
	ln, err := listen(addr, explicit)
	if err != nil {
		return err
	}
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	url := "http://127.0.0.1:" + port

	app := server.New(mgr, store, server.Options{Version: version, LAN: o.lan, DropDir: filepath.Join(paths.CacheDir, "drops")})
	defer app.Cleanup()
	srv := &http.Server{Handler: app.Handler(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second, MaxHeaderBytes: 32 << 10}
	if err := writeInfo(paths, serverInfo{URL: url, PID: os.Getpid(), Version: version, LAN: o.lan}); err != nil {
		slog.Warn("could not write the server info file", "err", err)
	}
	defer removeInfo(paths) // only if it is still ours: a newer Docent may have replaced it while this one was shutting down
	go watchConfig(ctx, paths, func() {
		if err := mgr.Sync(); err != nil {
			slog.Warn("could not reload libraries", "err", err)
		}
	})

	fmt.Printf("Docent %s is running at %s\n", version, url)
	if o.lan {
		fmt.Println("LAN mode: anyone on your network can read what is open. Read-only; adding folders is disabled. Reach it from another device at:")
		for _, ip := range server.LANAddrs() {
			fmt.Printf("  http://%s:%s\n", ip, port)
		}
	}
	fmt.Println(stopHint())
	if !o.noOpen {
		target := url
		if first != nil {
			target += "/" + first.ID + "/"
		}
		openBrowser(target)
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
		fmt.Println("\nStopped.")
	}
	return nil
}

// listen binds addr. When the address was not chosen by the user, a busy port moves on to the next one.
func listen(addr string, explicit bool) (net.Listener, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("bad address %q: %w", addr, err)
	}
	ln, err := net.Listen("tcp", addr)
	if err == nil || explicit {
		return ln, err
	}
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	for p := port + 1; p <= port+20; p++ {
		if ln, e := net.Listen("tcp", net.JoinHostPort(host, fmt.Sprint(p))); e == nil {
			return ln, nil
		}
	}
	return nil, fmt.Errorf("ports %d to %d are all busy: %w", port, port+20, err)
}

func writeInfo(p config.Paths, info serverInfo) error {
	if err := os.MkdirAll(p.CacheDir, 0o755); err != nil {
		return err
	}
	data, _ := json.Marshal(info)
	return os.WriteFile(p.ServerInfo, data, 0o644)
}

// removeInfo deletes the server info file only when it still names this process.
func removeInfo(p config.Paths) {
	data, err := os.ReadFile(p.ServerInfo)
	if err != nil {
		return
	}
	var info serverInfo
	if json.Unmarshal(data, &info) == nil && info.PID == os.Getpid() {
		_ = os.Remove(p.ServerInfo)
	}
}

// findRunning reports a live server, ignoring (and removing) a stale info file left by a crash.
func findRunning(p config.Paths) (serverInfo, bool) {
	data, err := os.ReadFile(p.ServerInfo)
	if err != nil {
		return serverInfo{}, false
	}
	var info serverInfo
	if json.Unmarshal(data, &info) != nil || info.URL == "" {
		return serverInfo{}, false
	}
	client := &http.Client{Timeout: 700 * time.Millisecond}
	res, err := client.Get(info.URL + "/_api/health")
	if err != nil {
		_ = os.Remove(p.ServerInfo)
		return serverInfo{}, false
	}
	res.Body.Close()
	return info, res.StatusCode == http.StatusOK
}

// postOpen asks a running server to open a path, and returns the new source's URL.
func postOpen(base, path string) (string, error) {
	abs, err := absPath(path)
	if err != nil {
		return "", err
	}
	body, _ := json.Marshal(map[string]string{"path": abs})
	res, err := http.Post(base+"/_api/open", "application/json", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var out struct {
		library.Info
		Error string `json:"error"`
	}
	_ = json.NewDecoder(res.Body).Decode(&out)
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s", out.Error)
	}
	return base + "/" + out.ID + "/", nil
}

// watchConfig calls onChange whenever config.json changes on disk (for example after `docent add` in another terminal).
func watchConfig(ctx context.Context, p config.Paths, onChange func()) {
	_ = os.MkdirAll(p.ConfigDir, 0o755)
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer w.Close()
	if w.Add(p.ConfigDir) != nil {
		return
	}
	var timer *time.Timer
	var fire <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-w.Events:
			if strings.HasSuffix(ev.Name, "config.json") {
				if timer == nil {
					timer = time.NewTimer(150 * time.Millisecond)
				} else {
					timer.Reset(150 * time.Millisecond)
				}
				fire = timer.C
			}
		case <-fire:
			fire = nil
			onChange()
		}
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Printf("Open %s in your browser.\n", url)
		return
	}
	go cmd.Wait()
}
