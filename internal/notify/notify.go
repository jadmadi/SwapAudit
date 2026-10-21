//go:build linux

// Package notify probes external notification transports and sends messages
// through argv-only subprocesses. No shell is ever invoked.
package notify

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Config injects every environment probe and subprocess seam.
type Config struct {
	Mode     string
	Command  []string
	Timeout  time.Duration
	Stderr   io.Writer
	LookPath func(string) (string, error)
	Getenv   func(string) string
	Run      func(ctx context.Context, argv []string) error
	OpenTTY  func() (io.WriteCloser, error)
}

// Message is one notification payload.
type Message struct {
	Title    string
	Body     string
	Severity string
}

// Transport is one delivery attempt; Next is the fallback chain.
type Transport struct {
	Name string
	Send func(Message) error
	Next *Transport
	warn func(format string, args ...any)
}

// Select builds the first available transport and its fallback chain.
func Select(cfg Config) (Transport, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 3 * time.Second
	}
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}
	if cfg.LookPath == nil {
		cfg.LookPath = exec.LookPath
	}
	if cfg.Getenv == nil {
		cfg.Getenv = os.Getenv
	}
	if cfg.Run == nil {
		cfg.Run = runCommand
	}
	if cfg.OpenTTY == nil {
		cfg.OpenTTY = func() (io.WriteCloser, error) {
			return os.OpenFile("/dev/tty", os.O_WRONLY, 0)
		}
	}
	mode := cfg.Mode
	if mode == "" {
		mode = "auto"
	}
	warn := func(format string, args ...any) {
		fmt.Fprintf(cfg.Stderr, format+"\n", args...)
	}

	var chain []Transport
	desktop := func() {
		if t, ok := notifySend(cfg); ok {
			chain = append(chain, t)
		}
		if t, ok := gdbus(cfg); ok {
			chain = append(chain, t)
		}
		if t, ok := dbusSend(cfg); ok {
			chain = append(chain, t)
		}
		if t, ok := kdialog(cfg); ok {
			chain = append(chain, t)
		}
	}

	switch mode {
	case "auto":
		desktop()
		if t, ok := osc(cfg); ok {
			chain = append(chain, t)
		}
		chain = append(chain, stderrTransport(cfg))
	case "desktop":
		desktop()
		if len(chain) == 0 {
			return Transport{}, errors.New("no desktop notification transport available")
		}
		chain = append(chain, stderrTransport(cfg))
	case "osc":
		if t, ok := osc(cfg); ok {
			chain = append(chain, t)
		}
		chain = append(chain, stderrTransport(cfg))
	case "stderr":
		chain = append(chain, stderrTransport(cfg))
	case "command":
		if len(cfg.Command) == 0 {
			return Transport{}, errors.New("no command notification transport available")
		}
		chain = append(chain, commandTransport(cfg))
		chain = append(chain, stderrTransport(cfg))
	default:
		return Transport{}, fmt.Errorf("unknown notification mode %q", mode)
	}

	for i := range chain {
		chain[i].warn = warn
		if i+1 < len(chain) {
			chain[i].Next = &chain[i+1]
		}
	}
	return chain[0], nil
}

// Send tries the chain and returns the name of the transport that succeeded.
func Send(ctx context.Context, t Transport, m Message) (string, error) {
	var firstErr error
	for tr := &t; tr != nil; tr = tr.Next {
		if err := ctx.Err(); err != nil {
			return t.Name, err
		}
		err := tr.Send(m)
		if err == nil {
			return tr.Name, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return t.Name, firstErr
}

// Sender drains messages, falls back on failure, and exits when closed.
func Sender(ctx context.Context, in <-chan Message, t Transport) {
	current := &t
	warned := map[string]bool{}
	for {
		select {
		case <-ctx.Done():
			return
		case m, ok := <-in:
			if !ok {
				return
			}
			for tr := current; tr != nil; tr = tr.Next {
				err := tr.Send(m)
				if err == nil {
					current = tr
					break
				}
				if !warned[tr.Name] {
					warned[tr.Name] = true
					if tr.warn != nil {
						tr.warn("notify: %s failed: %v; falling back", tr.Name, err)
					}
				}
			}
		}
	}
}

func notifySend(cfg Config) (Transport, bool) {
	if _, err := cfg.LookPath("notify-send"); err != nil {
		return Transport{}, false
	}
	return Transport{Name: "notify-send", Send: func(m Message) error {
		urgency := "normal"
		if m.Severity == "critical" {
			urgency = "critical"
		}
		return runTimed(cfg, "notify-send", "--app-name=swapaudit", "--urgency="+urgency, "--expire-time=10000", m.Title, m.Body)
	}}, true
}

func gdbus(cfg Config) (Transport, bool) {
	if _, err := cfg.LookPath("gdbus"); err != nil || cfg.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		return Transport{}, false
	}
	return Transport{Name: "gdbus", Send: func(m Message) error {
		return runTimed(cfg, "gdbus", "call", "--session",
			"--dest", "org.freedesktop.Notifications",
			"--object-path", "/org/freedesktop/Notifications",
			"--method", "org.freedesktop.Notifications.Notify",
			"swapaudit", "0", "", m.Title, m.Body, "[]", "{}", "10000")
	}}, true
}

func dbusSend(cfg Config) (Transport, bool) {
	if _, err := cfg.LookPath("dbus-send"); err != nil || cfg.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		return Transport{}, false
	}
	return Transport{Name: "dbus-send", Send: func(m Message) error {
		return runTimed(cfg, "dbus-send", "--session", "--type=method_call",
			"--dest=org.freedesktop.Notifications", "/org/freedesktop/Notifications",
			"org.freedesktop.Notifications.Notify",
			"string:swapaudit", "uint32:0", `string:""`, "string:"+m.Title, "string:"+m.Body,
			`array:string:""`, `dict:string:string:"",""`, "int32:10000")
	}}, true
}

func kdialog(cfg Config) (Transport, bool) {
	_, kdErr := cfg.LookPath("kdialog")
	display := cfg.Getenv("DISPLAY") != "" || cfg.Getenv("WAYLAND_DISPLAY") != ""
	if kdErr != nil || !display {
		return Transport{}, false
	}
	return Transport{Name: "kdialog", Send: func(m Message) error {
		return runTimed(cfg, "kdialog", "--title", m.Title, "--passivepopup", m.Body, "10")
	}}, true
}

func osc(cfg Config) (Transport, bool) {
	f, err := cfg.OpenTTY()
	if err != nil {
		return Transport{}, false
	}
	f.Close()
	return Transport{Name: "osc", Send: func(m Message) error {
		tty, err := cfg.OpenTTY()
		if err != nil {
			return err
		}
		_, werr := fmt.Fprintf(tty, "\x1b]777;notify;%s;%s\a", oscSafe(m.Title), oscSafe(m.Body))
		cerr := tty.Close()
		if werr != nil {
			return werr
		}
		return cerr
	}}, true
}

func stderrTransport(cfg Config) Transport {
	return Transport{Name: "stderr", Send: func(m Message) error {
		_, err := fmt.Fprintf(cfg.Stderr, "NOTIFY %s | %s\n", stripANSI(m.Title), collapse(m.Body))
		return err
	}}
}

func commandTransport(cfg Config) Transport {
	return Transport{Name: "command", Send: func(m Message) error {
		argv := append(append([]string{}, cfg.Command...), m.Title, m.Body)
		return runTimed(cfg, argv...)
	}}
}

func runTimed(cfg Config, argv ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	return cfg.Run(ctx, argv)
}

func runCommand(ctx context.Context, argv []string) error {
	if len(argv) == 0 {
		return errors.New("empty argv")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func oscSafe(s string) string {
	s = strings.ReplaceAll(s, ";", ",")
	s = strings.ReplaceAll(s, "\a", " ")
	return strings.ReplaceAll(s, "\n", " ")
}

func collapse(s string) string {
	return strings.Join(strings.Fields(stripANSI(s)), " ")
}

func stripANSI(s string) string {
	var b strings.Builder
	esc := false
	for _, r := range s {
		if r == 0x1b {
			esc = true
			continue
		}
		if esc {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				esc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
