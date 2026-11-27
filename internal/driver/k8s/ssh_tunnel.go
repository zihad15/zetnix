package k8s

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/zetnix/zetnix/internal/driver"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"k8s.io/client-go/tools/clientcmd"
)

// SSHConnectRequest configures an in-process SSH tunnel + remote kubeconfig bootstrap.
type SSHConnectRequest struct {
	Name                  string `json:"name"`
	Host                  string `json:"host"`
	Port                  string `json:"port"`
	Username              string `json:"username"`
	Password              string `json:"password"`
	PrivateKey            string `json:"privateKey"`
	Passphrase            string `json:"passphrase"`
	RemoteKubeconfigPath  string `json:"remoteKubeconfigPath"`
	KubeconfigYAML        string `json:"kubeconfigYaml"`
	Context               string `json:"context"`
	UseSudo               bool   `json:"useSudo"`
	InsecureSkipTLSVerify bool   `json:"insecureSkipTlsVerify"`
	TrustHostKey          bool   `json:"trustHostKey"`
}

type sshTunnel struct {
	client   *ssh.Client
	listener net.Listener
	closeMu  sync.Mutex
	closed   bool
}

func (t *sshTunnel) Close() error {
	t.closeMu.Lock()
	defer t.closeMu.Unlock()
	if t.closed {
		return nil
	}
	t.closed = true
	var errs []error
	if t.listener != nil {
		if err := t.listener.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if t.client != nil {
		if err := t.client.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (t *sshTunnel) serve(remoteTarget string) {
	for {
		localConn, err := t.listener.Accept()
		if err != nil {
			return
		}
		go func(lc net.Conn) {
			defer lc.Close()
			remoteConn, dialErr := t.client.Dial("tcp", remoteTarget)
			if dialErr != nil {
				return
			}
			defer remoteConn.Close()

			done := make(chan struct{}, 2)
			go func() {
				_, _ = io.Copy(remoteConn, lc)
				done <- struct{}{}
			}()
			go func() {
				_, _ = io.Copy(lc, remoteConn)
				done <- struct{}{}
			}()
			<-done
		}(localConn)
	}
}

// NewFromSSH dials the target host over SSH, optionally fetches the remote kubeconfig,
// opens a local loopback tunnel to the remote Kubernetes API server, and verifies connectivity.
func NewFromSSH(ctx context.Context, req SSHConnectRequest) (*Driver, string, error) {
	host := strings.TrimSpace(req.Host)
	port := strings.TrimSpace(req.Port)
	user := strings.TrimSpace(req.Username)
	if host == "" {
		return nil, "", fmt.Errorf("VPS host / IP is required")
	}
	if port == "" {
		port = "22"
	}
	if user == "" {
		user = "root"
	}

	var authMethods []ssh.AuthMethod
	if strings.TrimSpace(req.PrivateKey) != "" {
		var signer ssh.Signer
		var err error
		if req.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(req.PrivateKey), []byte(req.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(req.PrivateKey))
		}
		if err != nil {
			return nil, "", fmt.Errorf("parse SSH private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if req.Password != "" {
		pw := req.Password
		authMethods = append(authMethods,
			ssh.Password(pw),
			ssh.KeyboardInteractive(func(_, _ string, questions []string, _ []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = pw
				}
				return answers, nil
			}),
		)
	}
	if len(authMethods) == 0 {
		return nil, "", fmt.Errorf("provide an SSH password or private key")
	}

	var fingerprint string
	hostKeyCB := buildHostKeyCallback(req.TrustHostKey, &fingerprint)

	sshCfg := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCB,
		Timeout:         10 * time.Second,
	}

	sshAddr := net.JoinHostPort(host, port)
	sshClient, err := dialSSHWithContext(ctx, sshAddr, sshCfg)
	if err != nil {
		return nil, "", fmt.Errorf("SSH connection to %s failed: %w", sshAddr, err)
	}

	cleanupOnErr := true
	defer func() {
		if cleanupOnErr {
			_ = sshClient.Close()
		}
	}()

	kubeBytes := []byte(strings.TrimSpace(req.KubeconfigYAML))
	if len(kubeBytes) == 0 {
		fetched, fetchErr := fetchRemoteKubeconfig(ctx, sshClient, user, strings.TrimSpace(req.RemoteKubeconfigPath), req.UseSudo, req.Password)
		if fetchErr != nil {
			return nil, "", fetchErr
		}
		kubeBytes = fetched
	}

	rawCfg, err := clientcmd.Load(kubeBytes)
	if err != nil {
		return nil, "", fmt.Errorf("parse remote kubeconfig: %w", err)
	}

	ctxName := req.Context
	if ctxName == "" {
		ctxName = rawCfg.CurrentContext
	}
	if ctxName == "" {
		for k := range rawCfg.Contexts {
			ctxName = k
			break
		}
	}

	overrides := &clientcmd.ConfigOverrides{CurrentContext: ctxName}
	clientCfg := clientcmd.NewNonInteractiveClientConfig(*rawCfg, ctxName, overrides, nil)
	restCfg, err := clientCfg.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("build Kubernetes client config: %w", err)
	}

	remoteTarget, originalHost := extractRemoteAPITarget(restCfg.Host)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", fmt.Errorf("open local tunnel listener: %w", err)
	}

	tunnel := &sshTunnel{client: sshClient, listener: ln}
	go tunnel.serve(remoteTarget)

	restCfg.Host = "https://" + ln.Addr().String()
	restCfg.Timeout = 15 * time.Second
	if req.InsecureSkipTLSVerify {
		restCfg.TLSClientConfig.Insecure = true
		restCfg.TLSClientConfig.CAData = nil
		restCfg.TLSClientConfig.CAFile = ""
	} else if originalHost != "" && originalHost != "127.0.0.1" && originalHost != "localhost" {
		restCfg.TLSClientConfig.ServerName = originalHost
	}

	label := strings.TrimSpace(req.Name)
	if label == "" {
		label = fmt.Sprintf("%s (ssh)", host)
	}

	drv, err := newWithRESTConfig(restCfg, driver.ModeSSHTunnel, label, ctxName, "", tunnel)
	if err != nil {
		_ = tunnel.Close()
		return nil, "", err
	}

	info := drv.Info(ctx)
	if !info.Connected {
		_ = tunnel.Close()
		return nil, "", fmt.Errorf("SSH tunnel opened (%s), but Kubernetes API unreachable at %s: %s", ln.Addr().String(), remoteTarget, info.Error)
	}

	cleanupOnErr = false
	return drv, fingerprint, nil
}

func dialSSHWithContext(ctx context.Context, addr string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
	d := net.Dialer{Timeout: cfg.Timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

func buildHostKeyCallback(trustAny bool, fingerprintOut *string) ssh.HostKeyCallback {
	var knownCB ssh.HostKeyCallback
	if !trustAny {
		if home, err := os.UserHomeDir(); err == nil {
			khPath := filepath.Join(home, ".ssh", "known_hosts")
			if cb, khErr := knownhosts.New(khPath); khErr == nil {
				knownCB = cb
			}
		}
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		*fingerprintOut = ssh.FingerprintSHA256(key)
		if trustAny || knownCB == nil {
			return nil
		}
		return knownCB(hostname, remote, key)
	}
}

func fetchRemoteKubeconfig(ctx context.Context, client *ssh.Client, username, customPath string, useSudo bool, sudoPass string) ([]byte, error) {
	if customPath != "" {
		quoted := quoteRemotePath(customPath)
		if data, ok := runRemoteShell(ctx, client, "cat "+quoted+" 2>/dev/null", ""); ok && isValidKubeconfigBytes(data) {
			return data, nil
		}
		if data, ok := runRemoteShell(ctx, client, "sudo -n cat "+quoted+" 2>/dev/null", ""); ok && isValidKubeconfigBytes(data) {
			return data, nil
		}
		if useSudo && sudoPass != "" {
			if data, ok := runRemoteShell(ctx, client, "sudo -p '' -S cat "+quoted+" 2>/dev/null", sudoPass+"\n"); ok && isValidKubeconfigBytes(data) {
				return data, nil
			}
		}
		return nil, fmt.Errorf("connected over SSH as %q, but could not read kubeconfig at %q (check path/permissions)", username, customPath)
	}

	// Single-roundtrip discovery script (~200ms) for standard kubeconfigs and CLI configs.
	probeScript := `sh -lc '
for p in "$KUBECONFIG" "$HOME/.kube/config" "/etc/rancher/k3s/k3s.yaml" "/etc/rancher/rke2/rke2.yaml" "/etc/kubernetes/admin.conf" "/var/lib/k0s/pki/admin.conf" "/var/snap/microk8s/current/credentials/client.config" "/root/.kube/config"; do
  if [ -n "$p" ] && [ -r "$p" ] && grep -q "server:" "$p" 2>/dev/null; then
    cat "$p"
    exit 0
  fi
done
if command -v kubectl >/dev/null 2>&1; then
  out=$(kubectl config view --raw --flatten 2>/dev/null)
  if echo "$out" | grep -q "server:"; then
    echo "$out"
    exit 0
  fi
fi
if command -v microk8s >/dev/null 2>&1; then
  out=$(microk8s config 2>/dev/null)
  if echo "$out" | grep -q "server:"; then
    echo "$out"
    exit 0
  fi
fi
for p in "$HOME/.kube/config" "/etc/rancher/k3s/k3s.yaml" "/etc/rancher/rke2/rke2.yaml" "/etc/kubernetes/admin.conf" "/var/lib/k0s/pki/admin.conf" "/root/.kube/config"; do
  if [ -e "$p" ]; then
    echo "__ZETNIX_LOCKED__:$p"
  fi
done
exit 2
'`

	out, _ := runRemoteShell(ctx, client, probeScript, "")
	if isValidKubeconfigBytes(out) {
		return out, nil
	}

	// If sudo is enabled, run ONE single sudo probe covering root-owned distros.
	if useSudo {
		sudoCmd := `sudo -n sh -c '
for p in "/etc/rancher/k3s/k3s.yaml" "/etc/rancher/rke2/rke2.yaml" "/etc/kubernetes/admin.conf" "/var/lib/k0s/pki/admin.conf" "/root/.kube/config"; do
  if [ -r "$p" ] && grep -q "server:" "$p" 2>/dev/null; then cat "$p"; exit 0; fi
done
if command -v k0s >/dev/null 2>&1; then k0s kubeconfig admin 2>/dev/null && exit 0; fi
if command -v microk8s >/dev/null 2>&1; then microk8s config 2>/dev/null && exit 0; fi
exit 1
' 2>/dev/null`
		if data, ok := runRemoteShell(ctx, client, sudoCmd, ""); ok && isValidKubeconfigBytes(data) {
			return data, nil
		}
		if sudoPass != "" {
			sudoPassCmd := strings.Replace(sudoCmd, "sudo -n ", "sudo -p '' -S ", 1)
			if data, ok := runRemoteShell(ctx, client, sudoPassCmd, sudoPass+"\n"); ok && isValidKubeconfigBytes(data) {
				return data, nil
			}
		}
	}

	var locked []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "__ZETNIX_LOCKED__:") {
			locked = append(locked, strings.TrimPrefix(line, "__ZETNIX_LOCKED__:"))
		}
	}
	if len(locked) > 0 {
		return nil, fmt.Errorf(
			"connected over SSH as %q and found kubeconfig at %s, but permission was denied (no sudo access). Run on VPS: sudo cp %s ~/.kube/config && sudo chown %s:%s ~/.kube/config",
			username, strings.Join(locked, ", "), locked[0], username, username,
		)
	}

	return nil, fmt.Errorf(
		"connected over SSH as %q, but no kubeconfig was found on the server (checked ~/.kube/config, k3s, rke2, kubeadm, k0s, microk8s, and /root/.kube/config). Verify with 'kubectl get nodes' on the VPS or paste your kubeconfig in the Kubeconfig File / YAML tab",
		username,
	)
}

func isValidKubeconfigBytes(out []byte) bool {
	return len(out) > 0 && (bytes.Contains(out, []byte("clusters:")) || bytes.Contains(out, []byte(`"clusters"`))) &&
		(bytes.Contains(out, []byte("server:")) || bytes.Contains(out, []byte(`"server"`)))
}

func runRemoteShell(ctx context.Context, client *ssh.Client, cmd string, stdinData string) ([]byte, bool) {
	sess, err := client.NewSession()
	if err != nil {
		return nil, false
	}
	defer sess.Close()

	var stdout bytes.Buffer
	sess.Stdout = &stdout
	if stdinData != "" {
		sess.Stdin = strings.NewReader(stdinData)
	}

	done := make(chan error, 1)
	go func() {
		done <- sess.Run(cmd)
	}()

	timer := time.NewTimer(7 * time.Second)
	defer timer.Stop()

	select {
	case err := <-done:
		return bytes.TrimSpace(stdout.Bytes()), err == nil
	case <-timer.C:
		_ = sess.Close()
		return bytes.TrimSpace(stdout.Bytes()), false
	case <-ctx.Done():
		_ = sess.Close()
		return nil, false
	}
}

func quoteRemotePath(remotePath string) string {
	if strings.HasPrefix(remotePath, "$HOME/") {
		return `"$HOME/` + strings.ReplaceAll(strings.TrimPrefix(remotePath, "$HOME/"), `"`, `\"`) + `"`
	}
	return "'" + strings.ReplaceAll(remotePath, "'", "'\\''") + "'"
}

func extractRemoteAPITarget(serverURL string) (targetAddr string, hostOnly string) {
	u, err := url.Parse(serverURL)
	if err != nil || u.Host == "" {
		return "127.0.0.1:6443", "127.0.0.1"
	}
	h, p, splitErr := net.SplitHostPort(u.Host)
	if splitErr != nil {
		h = u.Host
		if u.Scheme == "http" {
			p = "80"
		} else {
			p = "6443"
		}
	}
	if h == "0.0.0.0" || h == "" {
		h = "127.0.0.1"
	}
	return net.JoinHostPort(h, p), h
}
