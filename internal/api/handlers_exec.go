package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zetnix/zetnix/internal/driver"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(_ *http.Request) bool { return true },
}

type wsClientMsg struct {
	Type string `json:"type"` // "stdin" or "resize"
	Data string `json:"data,omitempty"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
}

type wsWriter struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func (w *wsWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.conn.WriteMessage(websocket.TextMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *wsWriter) Ping() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second))
}

func (s *Server) handlePodExecWS(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ns := r.PathValue("namespace")
	pod := r.PathValue("pod")
	container := r.URL.Query().Get("container")
	shell := r.URL.Query().Get("shell")

	var cmd []string
	if shell != "" {
		cmd = []string{shell}
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	stdinR, stdinW := io.Pipe()
	resizeCh := make(chan driver.TerminalSize, 8)
	out := &wsWriter{conn: conn}

	// Keep WebSocket alive across reverse proxies (Nginx/Caddy/Traefik) on VPS deployments.
	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := out.Ping(); err != nil {
					cancel()
					return
				}
			}
		}
	}()

	// Read WebSocket input frames and dispatch stdin / resize events.
	go func() {
		defer cancel()
		defer stdinW.Close()
		defer close(resizeCh)

		for {
			_, raw, readErr := conn.ReadMessage()
			if readErr != nil {
				return
			}
			var msg wsClientMsg
			if json.Unmarshal(raw, &msg) == nil && msg.Type != "" {
				switch msg.Type {
				case "stdin":
					if _, err := io.WriteString(stdinW, msg.Data); err != nil {
						return
					}
				case "resize":
					if msg.Cols > 0 && msg.Rows > 0 {
						select {
						case resizeCh <- driver.TerminalSize{Cols: msg.Cols, Rows: msg.Rows}:
						default:
						}
					}
				}
				continue
			}
			// Raw fallback
			if _, err := stdinW.Write(raw); err != nil {
				return
			}
		}
	}()

	execErr := s.active().Exec(ctx, ns, pod, container, cmd, driver.ExecStreams{
		Stdin:    stdinR,
		Stdout:   out,
		Stderr:   out,
		ResizeCh: resizeCh,
	})
	if execErr != nil && ctx.Err() == nil {
		_, _ = out.Write([]byte("\r\n\x1b[31m[exec error] " + execErr.Error() + "\x1b[0m\r\n"))
	}
}
