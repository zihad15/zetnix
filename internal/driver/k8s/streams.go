package k8s

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/zetnix/zetnix/internal/driver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// StreamLogs streams live container logs from a Pod via client-go.
func (d *Driver) StreamLogs(ctx context.Context, namespace, pod, container string, tailLines int64, follow bool) (io.ReadCloser, error) {
	if tailLines <= 0 {
		tailLines = 200
	}
	opts := &corev1.PodLogOptions{
		Container:  container,
		Follow:     follow,
		Timestamps: true,
		TailLines:  &tailLines,
	}
	req := d.clientset.CoreV1().Pods(namespace).GetLogs(pod, opts)
	return req.Stream(ctx)
}

type ptySizeQueue struct {
	ch <-chan driver.TerminalSize
}

func (q *ptySizeQueue) Next() *remotecommand.TerminalSize {
	sz, ok := <-q.ch
	if !ok {
		return nil
	}
	return &remotecommand.TerminalSize{
		Width:  sz.Cols,
		Height: sz.Rows,
	}
}

// Exec attaches an interactive TTY stream to a container inside a Pod.
func (d *Driver) Exec(ctx context.Context, namespace, pod, container string, command []string, streams driver.ExecStreams) error {
	if len(command) == 0 {
		command = []string{"/bin/sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh"}
	}
	req := d.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(d.restConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("create SPDY executor: %w", err)
	}

	var sizeQueue remotecommand.TerminalSizeQueue
	if streams.ResizeCh != nil {
		sizeQueue = &ptySizeQueue{ch: streams.ResizeCh}
	}

	return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             streams.Stdin,
		Stdout:            streams.Stdout,
		Stderr:            streams.Stderr,
		Tty:               true,
		TerminalSizeQueue: sizeQueue,
	})
}

// Watch subscribes to real-time Pod, Deployment, and Event updates via Kubernetes Watch API.
func (d *Driver) Watch(ctx context.Context, namespace string) (<-chan driver.WatchEvent, error) {
	ch := make(chan driver.WatchEvent, 64)
	ns := nsParam(namespace)

	var wg sync.WaitGroup
	watchLoop := func(kind string, openWatch func() (watch.Interface, error)) {
		defer wg.Done()
		for {
			if ctx.Err() != nil {
				return
			}
			w, err := openWatch()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				case <-time.After(3 * time.Second):
					continue
				}
			}
			func() {
				defer w.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case ev, ok := <-w.ResultChan():
						if !ok {
							return
						}
						if ev.Type == "" || ev.Type == watch.Bookmark || ev.Type == watch.Error {
							continue
						}
						name, objNS, status := extractWatchMeta(ev.Object)
						if name == "" {
							continue
						}
						select {
						case ch <- driver.WatchEvent{
							EventType: string(ev.Type),
							Kind:      kind,
							Namespace: objNS,
							Name:      name,
							Status:    status,
							Timestamp: time.Now().UTC(),
						}:
						case <-ctx.Done():
							return
						}
					}
				}
			}()
		}
	}

	wg.Add(3)
	go watchLoop("Pod", func() (watch.Interface, error) {
		rv := ""
		if l, err := d.clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{Limit: 1}); err == nil {
			rv = l.ResourceVersion
		}
		return d.clientset.CoreV1().Pods(ns).Watch(ctx, metav1.ListOptions{ResourceVersion: rv, AllowWatchBookmarks: true})
	})
	go watchLoop("Deployment", func() (watch.Interface, error) {
		rv := ""
		if l, err := d.clientset.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{Limit: 1}); err == nil {
			rv = l.ResourceVersion
		}
		return d.clientset.AppsV1().Deployments(ns).Watch(ctx, metav1.ListOptions{ResourceVersion: rv, AllowWatchBookmarks: true})
	})
	go watchLoop("Event", func() (watch.Interface, error) {
		rv := ""
		if l, err := d.clientset.CoreV1().Events(ns).List(ctx, metav1.ListOptions{Limit: 1}); err == nil {
			rv = l.ResourceVersion
		}
		return d.clientset.CoreV1().Events(ns).Watch(ctx, metav1.ListOptions{ResourceVersion: rv, AllowWatchBookmarks: true})
	})

	go func() {
		wg.Wait()
		close(ch)
	}()

	return ch, nil
}

func extractWatchMeta(obj interface{}) (name, namespace, status string) {
	switch t := obj.(type) {
	case *corev1.Pod:
		return t.Name, t.Namespace, string(t.Status.Phase)
	case interface{ GetName() string }:
		name = t.GetName()
		if nsGetter, ok := obj.(interface{ GetNamespace() string }); ok {
			namespace = nsGetter.GetNamespace()
		}
		return name, namespace, ""
	default:
		return "", "", ""
	}
}
