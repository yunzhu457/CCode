package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunStartsAndExits(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(configPath, []byte(`
protocol: openai
model: test-model
base_url: https://example.test
api_key: test-key
`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	out := new(strings.Builder)
	err = run([]string{"-config", configPath}, strings.NewReader("/exit\n"), out)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !strings.Contains(out.String(), "C Code (openai)") {
		t.Fatalf("output = %q", out.String())
	}
	if !strings.Contains(out.String(), "bye") {
		t.Fatalf("output missing bye: %q", out.String())
	}
}

func TestRunReturnsConfigError(t *testing.T) {
	err := run([]string{"-config", filepath.Join(t.TempDir(), "missing.yaml")}, strings.NewReader(""), new(strings.Builder))
	if err == nil {
		t.Fatal("run() error = nil, want config error")
	}
}

func TestRunReturnsFlagError(t *testing.T) {
	err := run([]string{"-unknown"}, strings.NewReader(""), new(strings.Builder))
	if err == nil {
		t.Fatal("run() error = nil, want flag error")
	}
}

func TestRunStartsAnthropicAndExits(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(configPath, []byte(`
protocol: anthropic
model: test-model
base_url: https://example.test
api_key: test-key
`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	out := new(strings.Builder)
	err = run([]string{"-config", configPath}, strings.NewReader("/exit\n"), out)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !strings.Contains(out.String(), "C Code (anthropic)") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunRejectsUnknownProtocol(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(configPath, []byte(`
protocol: unknown
model: test-model
base_url: https://example.test
api_key: test-key
`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	err = run([]string{"-config", configPath}, strings.NewReader("/exit\n"), new(strings.Builder))
	if err == nil {
		t.Fatal("run() error = nil, want protocol error")
	}
}

func TestMainSuccessPath(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(configPath, []byte(`
protocol: openai
model: test-model
base_url: https://example.test
api_key: test-key
`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldArgs := os.Args
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	t.Cleanup(func() {
		os.Args = oldArgs
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	})

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	_, _ = stdinW.WriteString("/exit\n")
	_ = stdinW.Close()
	t.Cleanup(func() { _ = stdinR.Close() })

	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("create stdout: %v", err)
	}
	t.Cleanup(func() { _ = stdout.Close() })

	os.Args = []string{"ccode", "-config", configPath}
	os.Stdin = stdinR
	os.Stdout = stdout

	main()
}
