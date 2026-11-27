package k8s

import "testing"

func TestExtractRemoteAPITarget(t *testing.T) {
	cases := []struct {
		url      string
		wantAddr string
		wantHost string
	}{
		{"https://127.0.0.1:6443", "127.0.0.1:6443", "127.0.0.1"},
		{"https://0.0.0.0:6443", "127.0.0.1:6443", "127.0.0.1"},
		{"https://10.0.10.11:6443", "10.0.10.11:6443", "10.0.10.11"},
		{"https://k8s.internal", "k8s.internal:6443", "k8s.internal"},
	}

	for _, tc := range cases {
		gotAddr, gotHost := extractRemoteAPITarget(tc.url)
		if gotAddr != tc.wantAddr || gotHost != tc.wantHost {
			t.Fatalf("extractRemoteAPITarget(%q) = (%q, %q), want (%q, %q)", tc.url, gotAddr, gotHost, tc.wantAddr, tc.wantHost)
		}
	}
}

func TestIsValidKubeconfigBytes(t *testing.T) {
	validYAML := []byte("apiVersion: v1\nclusters:\n- cluster:\n    server: https://127.0.0.1:6443\n")
	if !isValidKubeconfigBytes(validYAML) {
		t.Fatalf("expected valid kubeconfig YAML to pass")
	}
	if isValidKubeconfigBytes([]byte("not a kubeconfig")) {
		t.Fatalf("expected invalid text to fail kubeconfig check")
	}
}
