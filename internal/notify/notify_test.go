//go:build linux

package notify

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func lookPath(available ...string) func(string) (string, error) {
	set := map[string]bool{}
	for _, name := range available {
		set[name] = true
	}
	return func(name string) (string, error) {
		if set[name] {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
}

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func noTTY() (io.WriteCloser, error) { return nil, errors.New("no tty") }

type bufferCloser struct{ *bytes.Buffer }

func (bufferCloser) Close() error { return nil }

func tty(buffer *bytes.Buffer) func() (io.WriteCloser, error) {
	return func() (io.WriteCloser, error) { return bufferCloser{buffer}, nil }
}

func chainNames(t Transport) []string {
	var out []string
	for p := &t; p != nil; p = p.Next {
		out = append(out, p.Name)
	}
	return out
}

func TestSelectAutoChain(t *testing.T) {
	var ttyBuf bytes.Buffer
	tr, err := Select(Config{
		Mode: "auto", LookPath: lookPath("notify-send", "gdbus", "dbus-send", "kdialog"),
		Getenv:  env(map[string]string{"DBUS_SESSION_BUS_ADDRESS": "unix:path=/run/user/1000/bus", "DISPLAY": ":1"}),
		OpenTTY: tty(&ttyBuf), Stderr: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "notify-send,gdbus,dbus-send,kdialog,osc,stderr"
	if got := strings.Join(chainNames(tr), ","); got != want {
		t.Errorf("chain = %s, want %s", got, want)
	}
}

func TestSelectFallbackToStderr(t *testing.T) {
	tr, err := Select(Config{Mode: "auto", LookPath: lookPath(), Getenv: env(nil), OpenTTY: noTTY, Stderr: &bytes.Buffer{}})
	if err != nil {
		t.Fatal(err)
	}
	if tr.Name != "stderr" {
		t.Errorf("primary = %q, want stderr", tr.Name)
	}
}

func TestSelectDesktopUnavailable(t *testing.T) {
	if _, err := Select(Config{Mode: "desktop", LookPath: lookPath(), Getenv: env(nil), OpenTTY: noTTY, Stderr: &bytes.Buffer{}}); err == nil {
		t.Fatal("desktop mode must fail when no desktop transport exists")
	}
}

func TestSelectCommandRequiresArgv(t *testing.T) {
	if _, err := Select(Config{Mode: "command", Stderr: &bytes.Buffer{}}); err == nil {
		t.Fatal("command mode must require --notify-cmd")
	}
}

func TestUnknownMode(t *testing.T) {
	if _, err := Select(Config{Mode: "carrier-pigeon", Stderr: &bytes.Buffer{}}); err == nil {
		t.Fatal("unknown mode must fail")
	}
}

func captureRun(captured *[]string, deadline bool, err error) func(context.Context, []string) error {
	return func(ctx context.Context, argv []string) error {
		*captured = append([]string{}, argv...)
		if deadline {
			if _, ok := ctx.Deadline(); !ok {
				return errors.New("missing deadline")
			}
		}
		return err
	}
}

func TestNotifySendArgv(t *testing.T) {
	var argv []string
	tr, err := Select(Config{
		Mode: "auto", LookPath: lookPath("notify-send"), Getenv: env(nil), OpenTTY: noTTY,
		Stderr: &bytes.Buffer{}, Timeout: time.Second, Run: captureRun(&argv, true, nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	name, err := Send(context.Background(), tr, Message{Title: "T", Body: "B", Severity: "critical"})
	if err != nil || name != "notify-send" {
		t.Fatalf("Send = %q, %v", name, err)
	}
	want := "notify-send --app-name=swapaudit --urgency=critical --expire-time=10000 T B"
	if got := strings.Join(argv, " "); got != want {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

func TestGDBusArgv(t *testing.T) {
	var argv []string
	tr, err := Select(Config{
		Mode: "auto", LookPath: lookPath("gdbus"),
		Getenv:  env(map[string]string{"DBUS_SESSION_BUS_ADDRESS": "bus"}),
		OpenTTY: noTTY, Stderr: &bytes.Buffer{}, Run: captureRun(&argv, false, nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	if name, err := Send(context.Background(), tr, Message{Title: "T", Body: "B"}); err != nil || name != "gdbus" {
		t.Fatalf("Send = %q, %v", name, err)
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"gdbus call --session", "org.freedesktop.Notifications.Notify", "swapaudit 0  T B [] {} 10000"} {
		if !strings.Contains(joined, want) {
			t.Errorf("argv missing %q: %s", want, joined)
		}
	}
}

func TestDBusSendArgv(t *testing.T) {
	var argv []string
	tr, err := Select(Config{
		Mode: "auto", LookPath: lookPath("dbus-send"),
		Getenv:  env(map[string]string{"DBUS_SESSION_BUS_ADDRESS": "bus"}),
		OpenTTY: noTTY, Stderr: &bytes.Buffer{}, Run: captureRun(&argv, false, nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	if name, err := Send(context.Background(), tr, Message{Title: "T", Body: "B"}); err != nil || name != "dbus-send" {
		t.Fatalf("Send = %q, %v", name, err)
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"dbus-send --session --type=method_call", "string:swapaudit", "uint32:0", "int32:10000"} {
		if !strings.Contains(joined, want) {
			t.Errorf("argv missing %q: %s", want, joined)
		}
	}
}

func TestKDialogArgv(t *testing.T) {
	var argv []string
	tr, err := Select(Config{
		Mode: "auto", LookPath: lookPath("kdialog"), Getenv: env(map[string]string{"WAYLAND_DISPLAY": "wayland-0"}),
		OpenTTY: noTTY, Stderr: &bytes.Buffer{}, Run: captureRun(&argv, false, nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	if name, err := Send(context.Background(), tr, Message{Title: "T", Body: "B"}); err != nil || name != "kdialog" {
		t.Fatalf("Send = %q, %v", name, err)
	}
	if got := strings.Join(argv, " "); got != "kdialog --title T --passivepopup B 10" {
		t.Errorf("argv = %q", got)
	}
}

func TestCommandLiteralArgv(t *testing.T) {
	var argv []string
	tr, err := Select(Config{
		Mode: "command", Command: []string{"/bin/echo", "; rm -rf /"},
		Stderr: &bytes.Buffer{}, Run: captureRun(&argv, false, nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Send(context.Background(), tr, Message{Title: "T", Body: "B"}); err != nil {
		t.Fatal(err)
	}
	if len(argv) != 4 || argv[0] != "/bin/echo" || argv[1] != "; rm -rf /" || argv[2] != "T" || argv[3] != "B" {
		t.Errorf("argv = %#v", argv)
	}
}

func TestSendFallsBackToStderr(t *testing.T) {
	var stderr bytes.Buffer
	tr, err := Select(Config{
		Mode: "auto", LookPath: lookPath("notify-send"), Getenv: env(nil), OpenTTY: noTTY,
		Stderr: &stderr, Run: captureRun(new([]string), false, errors.New("boom")),
	})
	if err != nil {
		t.Fatal(err)
	}
	name, err := Send(context.Background(), tr, Message{Title: "T", Body: "B"})
	if err != nil || name != "stderr" {
		t.Fatalf("Send = %q, %v", name, err)
	}
	if !strings.Contains(stderr.String(), "NOTIFY T | B") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestSenderWarnsOnceAndDrains(t *testing.T) {
	var stderr bytes.Buffer
	tr, err := Select(Config{
		Mode: "auto", LookPath: lookPath("notify-send"), Getenv: env(nil), OpenTTY: noTTY,
		Stderr: &stderr, Run: captureRun(new([]string), false, errors.New("boom")),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := make(chan Message, 4)
	done := make(chan struct{})
	go func() { defer close(done); Sender(ctx, ch, tr) }()
	ch <- Message{Title: "T1", Body: "B"}
	ch <- Message{Title: "T2", Body: "B"}
	close(ch)
	<-done
	if got := strings.Count(stderr.String(), "notify: notify-send failed:"); got != 1 {
		t.Errorf("warnings = %d, want 1\n%s", got, stderr.String())
	}
	if got := strings.Count(stderr.String(), "NOTIFY"); got != 2 {
		t.Errorf("stderr deliveries = %d, want 2\n%s", got, stderr.String())
	}
}

func TestOSCPayload(t *testing.T) {
	var ttyBuf bytes.Buffer
	tr, err := Select(Config{Mode: "osc", LookPath: lookPath(), Getenv: env(nil), OpenTTY: tty(&ttyBuf), Stderr: &bytes.Buffer{}})
	if err != nil {
		t.Fatal(err)
	}
	if name, err := Send(context.Background(), tr, Message{Title: "a;b", Body: "line1\nline2"}); err != nil || name != "osc" {
		t.Fatalf("Send = %q, %v", name, err)
	}
	got := ttyBuf.String()
	if !strings.HasPrefix(got, "\x1b]777;notify;") || !strings.HasSuffix(got, "\a") {
		t.Errorf("osc payload = %q", got)
	}
	if strings.Contains(got, "a;b") {
		t.Error("OSC title was not sanitized")
	}
}

func TestStderrCollapseAndStripANSI(t *testing.T) {
	var stderr bytes.Buffer
	tr, err := Select(Config{Mode: "stderr", Stderr: &stderr})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Send(context.Background(), tr, Message{Title: "\x1b[31mT\x1b[0m", Body: "a\n  b\tc"}); err != nil {
		t.Fatal(err)
	}
	if got := stderr.String(); got != "NOTIFY T | a b c\n" {
		t.Errorf("stderr = %q", got)
	}
}
