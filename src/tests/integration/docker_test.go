package integration_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	"clipshare/src/internal/certs"
)

const (
	daemonImage     = "clipshare:integration"
	listenerContext = "src/tests/integration/listener"
)

// tempDir creates a temporary directory under /tmp that is traversable by
// container users and is cleaned up when the test ends.
func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "clipshare-int-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("chmod temp dir: %v", err)
	}
	return dir
}

// copyFixture copies a fixture config file into a temp directory and returns
// the host path to the copy.
func copyFixture(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join("fixtures", name)
	dir := tempDir(t)
	dst := filepath.Join(dir, name)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
	return dst
}

// waitForLog polls a container's logs until the given substring appears or the
// timeout expires.
func waitForLog(ctx context.Context, c testcontainers.Container, want string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		logs, err := c.Logs(ctx)
		if err != nil {
			return err
		}
		data, err := io.ReadAll(logs)
		logs.Close()
		if err != nil {
			return err
		}
		if strings.Contains(string(data), want) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil // timeout, caller should fail
}

// daemonReadyLog returns the log line to wait for depending on whether TLS is
// enabled.
func daemonReadyLog(tls bool) string {
	if tls {
		return "wss server listening"
	}
	return "ws server listening"
}

// startNetwork creates an isolated Docker bridge network for one test.
// It is removed automatically when the test ends.
func startNetwork(ctx context.Context, t *testing.T) *testcontainers.DockerNetwork {
	t.Helper()
	net, err := network.New(ctx)
	if err != nil {
		t.Fatalf("create network: %v", err)
	}
	t.Cleanup(func() {
		if err := net.Remove(ctx); err != nil {
			t.Logf("remove network: %v", err)
		}
	})
	return net
}



// startDaemon starts the clipshare daemon container on the given network.
// If certDir is non-empty, the CA/server certificate files are copied into
// /app/certs.
func startDaemon(ctx context.Context, t *testing.T, net *testcontainers.DockerNetwork, cfgPath, certDir string) testcontainers.Container {
	t.Helper()
	return startDaemonWithAlias(ctx, t, net, cfgPath, certDir, "daemon")
}

// startDaemonWithAlias is like startDaemon but lets the caller choose the
// container's network alias.
func startDaemonWithAlias(ctx context.Context, t *testing.T, net *testcontainers.DockerNetwork, cfgPath, certDir, alias string) testcontainers.Container {
	t.Helper()
	files := []testcontainers.ContainerFile{
		{
			HostFilePath:      cfgPath,
			ContainerFilePath: "/app/config.toml",
			FileMode:          0o644,
		},
	}
	if certDir != "" {
		for _, name := range []string{"ca.pem", "server.pem", "server.key"} {
			files = append(files, testcontainers.ContainerFile{
				HostFilePath:      filepath.Join(certDir, name),
				ContainerFilePath: filepath.Join("/app/certs", name),
				FileMode:          0o644,
			})
		}
	}

	req := testcontainers.ContainerRequest{
		Image:          daemonImage,
		Files:          files,
		Networks:       []string{net.Name},
		NetworkAliases: map[string][]string{net.Name: {alias}},
		ExposedPorts:   []string{"40403/tcp", "40405/tcp", "40404/udp"},
		Cmd:            []string{"./clipshare", "daemon", "--no-watch"},
		WaitingFor:     wait.ForLog(daemonReadyLog(certDir != "")).WithStartupTimeout(30 * time.Second),
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	t.Cleanup(func() {
		if err := c.Terminate(ctx); err != nil {
			t.Logf("terminate daemon: %v", err)
		}
	})
	return c
}

// daemonHostPort returns the host:port to reach a daemon container port from
// the test runner.
func daemonHostPort(ctx context.Context, t *testing.T, c testcontainers.Container, port string) string {
	t.Helper()
	host, err := c.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	mapped, err := c.MappedPort(ctx, nat.Port(port))
	if err != nil {
		t.Fatalf("mapped port %s: %v", port, err)
	}
	return fmt.Sprintf("%s:%s", host, mapped.Port())
}

// startListener builds and starts the mDNS/beacon listener container on the
// given network.
func startListener(ctx context.Context, t *testing.T, net *testcontainers.DockerNetwork, beaconPort int) testcontainers.Container {
	t.Helper()
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:       "listener",
			PrintBuildLog: true,
		},
		Networks: []string{net.Name},
		Cmd:      []string{"-beacon-port", fmt.Sprintf("%d", beaconPort), "-timeout", "10s"},
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start listener: %v", err)
	}
	t.Cleanup(func() {
		if err := c.Terminate(ctx); err != nil {
			t.Logf("terminate listener: %v", err)
		}
	})
	return c
}

// readListenerOutput waits for the listener container to exit and returns its
// stdout/stderr lines.
func readListenerOutput(ctx context.Context, t *testing.T, c testcontainers.Container) []string {
	t.Helper()
	state, err := c.State(ctx)
	if err != nil {
		t.Fatalf("container state: %v", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for state.Running && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		state, err = c.State(ctx)
		if err != nil {
			t.Fatalf("container state: %v", err)
		}
	}

	logs, err := c.Logs(ctx)
	if err != nil {
		t.Fatalf("listener logs: %v", err)
	}
	defer logs.Close()
	data, err := io.ReadAll(logs)
	if err != nil {
		t.Fatalf("read logs: %v", err)
	}

	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// containerClipboard reads the container's X11 clipboard via xclip and copies
// the result out of the container to avoid Docker exec stream multiplexing
// headers.
func containerClipboard(ctx context.Context, t *testing.T, c testcontainers.Container) string {
	t.Helper()
	return readContainerFile(ctx, t, c, "/tmp/clipboard.txt", func() error {
		code, _, err := c.Exec(ctx, []string{"sh", "-c", "xclip -selection clipboard -t UTF8_STRING -o > /tmp/clipboard.txt"})
		if err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("xclip exit code %d", code)
		}
		return nil
	})
}

// watchClipboard reads the clipboard file produced by the watch container's
// command wrapper.
func watchClipboard(ctx context.Context, t *testing.T, c testcontainers.Container) string {
	t.Helper()
	return readContainerFile(ctx, t, c, "/tmp/watch-clipboard.txt", nil)
}

// readContainerFile copies a file from the container to the host and returns
// its contents. If setup is non-nil it is called first to create the file.
func readContainerFile(ctx context.Context, t *testing.T, c testcontainers.Container, containerPath string, setup func() error) string {
	t.Helper()
	if setup != nil {
		if err := setup(); err != nil {
			t.Fatalf("setup for %s: %v", containerPath, err)
		}
	}
	rc, err := c.CopyFileFromContainer(ctx, containerPath)
	if err != nil {
		t.Fatalf("copy %s from container: %v", containerPath, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read %s: %v", containerPath, err)
	}
	return strings.TrimSpace(string(data))
}

// execSend runs `clipshare send <text>` inside the daemon container via the
// local API.
func execSend(ctx context.Context, t *testing.T, c testcontainers.Container, text string) {
	t.Helper()
	code, reader, err := c.Exec(ctx, []string{"./clipshare", "send", text})
	if err != nil {
		t.Fatalf("exec send: %v", err)
	}
	data, _ := io.ReadAll(reader)
	if code != 0 {
		t.Fatalf("send failed (code %d): %s", code, string(data))
	}
}

// startOneShot starts a transient `clipshare send --oneshot <text>` container
// on the given network and exposes its WebSocket port to the host.
func startOneShot(ctx context.Context, t *testing.T, net *testcontainers.DockerNetwork, cfgPath, text string) testcontainers.Container {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image: daemonImage,
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      cfgPath,
				ContainerFilePath: "/app/config.toml",
				FileMode:          0o644,
			},
		},
		Networks:     []string{net.Name},
		ExposedPorts: []string{"40403/tcp"},
		Cmd:          []string{"./clipshare", "send", "--oneshot", text},
		WaitingFor:   wait.ForLog("one-shot: waiting for a client").WithStartupTimeout(30 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start one-shot: %v", err)
	}
	t.Cleanup(func() {
		if err := c.Terminate(ctx); err != nil {
			t.Logf("terminate one-shot: %v", err)
		}
	})
	return c
}

// waitForContainerExit waits until the container is no longer running or the
// timeout expires. It returns the final exit code if available.
func waitForContainerExit(ctx context.Context, t *testing.T, c testcontainers.Container, timeout time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		state, err := c.State(ctx)
		if err != nil {
			t.Fatalf("container state: %v", err)
		}
		if !state.Running {
			return state.ExitCode
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for container to exit")
	return -1
}

// startWatch starts a `clipshare watch --timeout <dur>` container on the given
// network and exposes its WebSocket port to the host.
func startWatch(ctx context.Context, t *testing.T, net *testcontainers.DockerNetwork, cfgPath string, timeout time.Duration) testcontainers.Container {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image: daemonImage,
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      cfgPath,
				ContainerFilePath: "/app/config.toml",
				FileMode:          0o644,
			},
		},
		Networks:     []string{net.Name},
		ExposedPorts: []string{"40403/tcp"},
		Cmd: []string{
			"sh", "-c",
			fmt.Sprintf("./clipshare watch --timeout %s && xclip -selection clipboard -t UTF8_STRING -o > /tmp/watch-clipboard.txt && sleep 1", timeout.String()),
		},
		WaitingFor: wait.ForLog("watch: listening").WithStartupTimeout(30 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start watch: %v", err)
	}
	t.Cleanup(func() {
		if err := c.Terminate(ctx); err != nil {
			t.Logf("terminate watch: %v", err)
		}
	})
	return c
}

// generateCerts creates a temporary CA, server cert, and client cert for mTLS
// tests. It returns the directory containing all PEM files.
func generateCerts(t *testing.T) string {
	t.Helper()
	dir := tempDir(t)
	if err := certs.Init(dir); err != nil {
		t.Fatalf("init CA: %v", err)
	}
	if err := certs.Issue(dir, "daemon-tls", "server", []string{"127.0.0.1", "localhost"}); err != nil {
		t.Fatalf("issue server cert: %v", err)
	}
	if err := certs.Issue(dir, "android", "client", nil); err != nil {
		t.Fatalf("issue client cert: %v", err)
	}
	// Make keys readable by the container user for test mounts.
	for _, name := range []string{"ca.key", "server.key", "android-client.key"} {
		if err := os.Chmod(filepath.Join(dir, name), 0o644); err != nil {
			t.Fatalf("chmod %s: %v", name, err)
		}
	}
	return dir
}

// clientTLSConfig returns a tls.Config for the Android simulator using the
// client certificate issued by generateCerts.
func clientTLSConfig(t *testing.T, certDir string) *tls.Config {
	t.Helper()
	pool, err := certs.LoadPool(filepath.Join(certDir, "ca.pem"))
	if err != nil {
		t.Fatalf("load CA pool: %v", err)
	}
	cert, err := certs.LoadKeyPair(
		filepath.Join(certDir, "android-client.pem"),
		filepath.Join(certDir, "android-client.key"),
	)
	if err != nil {
		t.Fatalf("load client keypair: %v", err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}
}

// serverTLSConfig returns a tls.Config that trusts the test CA. It is used to
// dial a daemon that requires mTLS from the test runner.
func serverTLSConfig(t *testing.T, certDir string) *tls.Config {
	t.Helper()
	pool, err := certs.LoadPool(filepath.Join(certDir, "ca.pem"))
	if err != nil {
		t.Fatalf("load CA pool: %v", err)
	}
	return &tls.Config{RootCAs: pool}
}
