package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func (s *Server) handleWatchSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	ch, err := s.active().Watch(r.Context(), queryNamespace(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	_, _ = fmt.Fprint(w, "event: ready\ndata: {\"connected\":true}\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			b, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "event: watch\ndata: %s\n\n", b)
			flusher.Flush()
		}
	}
}

func (s *Server) handlePodLogsSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	ns := r.PathValue("namespace")
	pod := r.PathValue("pod")
	container := r.URL.Query().Get("container")
	follow := r.URL.Query().Get("follow") != "false"
	tail := int64(250)
	if rawTail := r.URL.Query().Get("tail"); rawTail != "" {
		if n, err := strconv.ParseInt(rawTail, 10, 64); err == nil && n > 0 {
			tail = n
		}
	}

	stream, err := s.active().StreamLogs(r.Context(), ns, pod, container, tail, follow)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		if r.Context().Err() != nil {
			return
		}
		payload, _ := json.Marshal(map[string]string{"line": scanner.Text()})
		_, _ = fmt.Fprintf(w, "event: log\ndata: %s\n\n", payload)
		flusher.Flush()
	}

	_, _ = fmt.Fprint(w, "event: done\ndata: {\"eof\":true}\n\n")
	flusher.Flush()
}
