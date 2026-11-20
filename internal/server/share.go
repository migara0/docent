package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"rsc.io/qr"
)

// Sharing with a phone: while it is on, a second listener on the local network serves the same libraries read-only (the same rules as `docent serve --lan`:
// no folder picker, nothing can be added or changed). It is off until you turn it on, stops by itself after a time limit, and always stops when Docent exits.
// The page shows a QR code for the address of the page you are reading, so the phone opens the same page.

const (
	shareFirstPort = 8788
	defaultShare   = 60 * time.Minute
	minShare       = 5 * time.Minute
	maxShare       = 8 * time.Hour
)

type sharer struct {
	mu    sync.Mutex
	srv   *http.Server
	port  int
	until time.Time
	timer *time.Timer
}

// virtualInterface is true for the networks containers and virtual machines create on this computer: a phone can never reach those.
func virtualInterface(name string) bool {
	for _, p := range []string{"docker", "br-", "veth", "virbr", "lxc", "lxd", "cni", "flannel", "podman", "vboxnet", "vmnet", "tun", "tap"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// LANAddrs lists this computer's private IPv4 addresses on real networks (the ones a phone on the same network can reach).
func LANAddrs() []string {
	var out []string
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 || virtualInterface(ifc.Name) {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && n.IP.IsPrivate() {
				out = append(out, n.IP.String())
			}
		}
	}
	return out
}

func (sh *sharer) stop() {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.stopLocked()
}

func (sh *sharer) stopLocked() {
	if sh.timer != nil {
		sh.timer.Stop()
		sh.timer = nil
	}
	if sh.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = sh.srv.Shutdown(ctx)
		cancel()
		_ = sh.srv.Close()
		sh.srv = nil
	}
}

type shareInfo struct {
	Available bool      `json:"available"` // false in LAN mode itself: only the computer's own browser can start sharing
	Active    bool      `json:"active"`
	URLs      []string  `json:"urls"`
	Until     time.Time `json:"until,omitzero"`
	NoNetwork bool      `json:"noNetwork"`
}

func (s *Server) shareInfo() shareInfo {
	info := shareInfo{Available: !s.opt.LAN, URLs: []string{}}
	s.share.mu.Lock()
	defer s.share.mu.Unlock()
	if s.share.srv != nil {
		info.Active, info.Until = true, s.share.until
		for _, ip := range LANAddrs() {
			info.URLs = append(info.URLs, fmt.Sprintf("http://%s:%d", ip, s.share.port))
		}
	} else {
		info.NoNetwork = len(LANAddrs()) == 0
	}
	return info
}

func (s *Server) shareGet(w http.ResponseWriter, r *http.Request) error { return ok(w, s.shareInfo()) }

type shareRequest struct {
	On      bool `json:"on"`
	Minutes int  `json:"minutes"`
}

func (s *Server) shareSet(w http.ResponseWriter, r *http.Request) error {
	if err := s.mutationsAllowed(); err != nil {
		return err
	}
	var req shareRequest
	if err := readJSON(w, r, &req); err != nil {
		return err
	}
	if !req.On {
		s.share.stop()
		return ok(w, s.shareInfo())
	}
	if len(LANAddrs()) == 0 {
		return fail(http.StatusConflict, "this computer has no address on a local network (connect to Wi-Fi or a cable network first)")
	}
	d := time.Duration(req.Minutes) * time.Minute
	if d <= 0 {
		d = defaultShare
	}
	d = min(max(d, minShare), maxShare)

	s.share.mu.Lock()
	if s.share.srv == nil {
		var ln net.Listener
		var err error
		for p := shareFirstPort; p < shareFirstPort+20; p++ {
			if ln, err = net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", p)); err == nil {
				s.share.port = p
				break
			}
		}
		if err != nil {
			s.share.mu.Unlock()
			return fail(http.StatusConflict, "no free port to share on: %v", err)
		}
		handler := New(s.mgr, s.store, Options{Version: s.opt.Version, LAN: true}).Handler()
		srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
		go srv.Serve(ln)
		s.share.srv = srv
	}
	// a new request restarts the time limit
	if s.share.timer != nil {
		s.share.timer.Stop()
	}
	s.share.until = time.Now().Add(d)
	s.share.timer = time.AfterFunc(d, s.share.stop)
	s.share.mu.Unlock()
	return ok(w, s.shareInfo())
}

// shareQR draws the QR code for an address of the running share (nothing else: the endpoint is not a general QR generator).
func (s *Server) shareQR(w http.ResponseWriter, r *http.Request) error {
	info := s.shareInfo()
	raw := r.URL.Query().Get("url")
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 600 || u.Scheme != "http" || !info.Active {
		return fail(http.StatusBadRequest, "not a shared address")
	}
	base := false
	for _, b := range info.URLs {
		if strings.TrimPrefix(b, "http://") == u.Host {
			base = true
		}
	}
	if !base {
		return fail(http.StatusBadRequest, "not a shared address")
	}
	code, err := qr.Encode(u.String(), qr.M) // re-escaped, so the code only ever holds ASCII
	if err != nil {
		return fail(http.StatusBadRequest, "cannot encode that address: %v", err)
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(qrSVG(code)))
	return nil
}

// qrSVG draws the code in black on white with the 4-module quiet zone scanners need, whatever the page's theme.
func qrSVG(c *qr.Code) string {
	const quiet = 4
	var b strings.Builder
	n := c.Size + 2*quiet
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges"><rect width="%d" height="%d" fill="#fff"/><path fill="#000" d="`, n, n, n, n)
	for y := 0; y < c.Size; y++ {
		for x := 0; x < c.Size; {
			if !c.Black(x, y) {
				x++
				continue
			}
			run := 0
			for x+run < c.Size && c.Black(x+run, y) {
				run++
			}
			fmt.Fprintf(&b, "M%d %dh%dv1h-%dz", x+quiet, y+quiet, run, run)
			x += run
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String()
}
