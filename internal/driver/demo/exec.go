package demo

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/zetnix/zetnix/internal/driver"
)

// Exec provides an interactive ANSI PTY shell simulator for Demo Mode pods so users can
// explore container filesystems, environment variables, processes, and health probes inside xterm.js.
func (d *Driver) Exec(ctx context.Context, namespace, pod, container string, _ []string, streams driver.ExecStreams) error {
	if container == "" {
		container = "app"
	}
	write := func(s string) {
		_, _ = io.WriteString(streams.Stdout, s)
	}

	banner := fmt.Sprintf(
		"\r\n\x1b[38;5;51m⚡ Zetnix Interactive Container Shell\x1b[0m \x1b[90m(Demo Sandbox Mode)\x1b[0m\r\n"+
			"\x1b[90mTarget:\x1b[0m \x1b[37m%s/%s\x1b[0m  \x1b[90mContainer:\x1b[0m \x1b[36m%s\x1b[0m\r\n"+
			"\x1b[90mType \x1b[33mhelp\x1b[90m for built-in commands (ls, ps, env, top, df, cat, curl, clear, exit).\x1b[0m\r\n\r\n",
		namespace, pod, container,
	)
	write(banner)

	prompt := fmt.Sprintf("\x1b[1;32mroot@%s\x1b[0m:\x1b[1;34m/app\x1b[0m# ", pod)
	write(prompt)

	// Drain resize events cleanly
	if streams.ResizeCh != nil {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case _, ok := <-streams.ResizeCh:
					if !ok {
						return
					}
				}
			}
		}()
	}

	buf := make([]byte, 1024)
	var line strings.Builder

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		n, err := streams.Stdin.Read(buf)
		if err != nil {
			return nil
		}
		for i := 0; i < n; i++ {
			b := buf[i]
			switch b {
			case '\r', '\n':
				write("\r\n")
				cmd := strings.TrimSpace(line.String())
				line.Reset()
				if cmd == "exit" || cmd == "quit" {
					write("\x1b[90mSession closed.\x1b[0m\r\n")
					return nil
				}
				if out := runDemoCommand(cmd, namespace, pod, container); out != "" {
					write(out)
				}
				write(prompt)
			case 0x7f, 0x08: // Backspace
				cur := line.String()
				if len(cur) > 0 {
					line.Reset()
					line.WriteString(cur[:len(cur)-1])
					write("\b \b")
				}
			case 0x03: // Ctrl+C
				line.Reset()
				write("^C\r\n" + prompt)
			case 0x0c: // Ctrl+L
				write("\x1b[2J\x1b[H" + prompt + line.String())
			default:
				if b >= 32 && b < 127 {
					line.WriteByte(b)
					_, _ = streams.Stdout.Write([]byte{b})
				}
			}
		}
	}
}

func runDemoCommand(cmd, ns, pod, container string) string {
	if cmd == "" {
		return ""
	}
	parts := strings.Fields(cmd)
	switch parts[0] {
	case "help":
		return "\x1b[36mAvailable Demo Commands:\x1b[0m\r\n" +
			"  ls [-la]        List files in /app\r\n" +
			"  pwd             Print working directory\r\n" +
			"  whoami          Print current user\r\n" +
			"  uname [-a]      Print container kernel info\r\n" +
			"  ps              Show running container processes\r\n" +
			"  env             Inspect pod environment variables\r\n" +
			"  top             Snapshot CPU & memory usage\r\n" +
			"  df              Filesystem disk usage\r\n" +
			"  cat <file>      Inspect /etc/os-release or /app/config.yaml\r\n" +
			"  curl <url>      Test local /healthz endpoint\r\n" +
			"  clear           Clear terminal screen\r\n" +
			"  exit            Close session\r\n"
	case "clear":
		return "\x1b[2J\x1b[H"
	case "pwd":
		return "/app\r\n"
	case "whoami":
		return "root\r\n"
	case "id":
		return "uid=0(root) gid=0(root) groups=0(root)\r\n"
	case "uname":
		return "Linux " + pod + " 6.8.0-45-generic #45-Ubuntu SMP PREEMPT_DYNAMIC x86_64 GNU/Linux\r\n"
	case "ls":
		return "\x1b[1;34mbin\x1b[0m  \x1b[1;34mconfig\x1b[0m  \x1b[1;32mserver\x1b[0m  config.yaml  healthcheck.sh\r\n"
	case "ps":
		return "PID   USER     TIME  COMMAND\r\n" +
			"  1   root     0:42  /app/server --port=8080 --metrics=9090\r\n" +
			" 19   root     0:00  /bin/sh\r\n" +
			" 24   root     0:00  ps aux\r\n"
	case "env":
		return fmt.Sprintf(
			"HOSTNAME=%s\r\nPOD_NAMESPACE=%s\r\nCONTAINER_NAME=%s\r\nKUBERNETES_SERVICE_HOST=10.43.0.1\r\nKUBERNETES_SERVICE_PORT=443\r\nLOG_LEVEL=info\r\nOTEL_EXPORTER_OTLP_ENDPOINT=http://prometheus-server.observability.svc:9090\r\n",
			pod, ns, container,
		)
	case "top":
		return "Mem: 142M used, 370M free | CPU: 4.2% usr, 1.1% sys, 94.7% idle\r\n" +
			"PID 1  /app/server   CPU 3.8%   RSS 118Mi\r\n"
	case "df":
		return "Filesystem      Size  Used Avail Use% Mounted on\r\n" +
			"overlay          48G   14G   34G  30% /\r\n" +
			"tmpfs            64M   12K   64M   1% /var/run/secrets/kubernetes.io/serviceaccount\r\n"
	case "cat":
		if len(parts) > 1 && strings.Contains(parts[1], "os-release") {
			return "NAME=\"Alpine Linux\"\r\nID=alpine\r\nVERSION_ID=3.20.3\r\nPRETTY_NAME=\"Alpine Linux v3.20\"\r\n"
		}
		return "service:\n  name: " + pod + "\n  namespace: " + ns + "\n  telemetry: enabled\r\n"
	case "curl":
		return "{\"status\":\"ok\",\"pod\":\"" + pod + "\",\"namespace\":\"" + ns + "\",\"ready\":true}\r\n"
	default:
		return fmt.Sprintf("sh: %s: simulated sandbox shell — try 'help' or connect a live cluster for full Linux binary exec\r\n", parts[0])
	}
}
