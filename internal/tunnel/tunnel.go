package tunnel

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	cloudflaredURL    = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe"
	cloudflaredURLArm = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-arm64.exe"
)

// Tunnel manages a cloudflared quick tunnel.
type Tunnel struct {
	cmd       *exec.Cmd
	publicURL string
	cancel    context.CancelFunc
}

// EnsureCloudflared downloads cloudflared if not already present.
// Returns the path to the cloudflared binary.
func EnsureCloudflared(baseDir string) (string, error) {
	binaryName := "cloudflared.exe"
	binaryPath := filepath.Join(baseDir, binaryName)

	// Check if already exists
	if _, err := os.Stat(binaryPath); err == nil {
		return binaryPath, nil
	}

	// Check if in PATH
	if path, err := exec.LookPath("cloudflared"); err == nil {
		return path, nil
	}

	fmt.Fprintf(os.Stderr, "  Scaricamento cloudflared...\n")

	dlURL := cloudflaredURL
	if runtime.GOARCH == "arm64" {
		dlURL = cloudflaredURLArm
	}

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", baseDir, err)
	}

	if err := downloadFile(binaryPath, dlURL); err != nil {
		return "", fmt.Errorf("download cloudflared: %w", err)
	}

	fmt.Fprintf(os.Stderr, "  ✓ cloudflared scaricato in %s\n", binaryPath)
	return binaryPath, nil
}

// Start launches a cloudflared quick tunnel pointing to the given local port.
// It returns the public URL once the tunnel is ready.
func Start(ctx context.Context, cloudflaredPath string, localPort int) (*Tunnel, error) {
	tunnelCtx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(tunnelCtx, cloudflaredPath,
		"tunnel", "--url", fmt.Sprintf("http://localhost:%d", localPort),
		"--no-autoupdate",
	)

	// Capture stderr where cloudflared outputs the URL
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start cloudflared: %w", err)
	}

	t := &Tunnel{
		cmd:    cmd,
		cancel: cancel,
	}

	// Wait for the tunnel URL (with timeout)
	urlCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		urlRegex := regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)

		for scanner.Scan() {
			line := scanner.Text()
			if matches := urlRegex.FindString(line); matches != "" {
				urlCh <- matches
				break
			}
		}
		// Keep reading to prevent pipe block
		io.Copy(io.Discard, stderrPipe)
	}()

	go func() {
		if err := cmd.Wait(); err != nil && tunnelCtx.Err() == nil {
			errCh <- fmt.Errorf("cloudflared exited: %w", err)
		}
	}()

	select {
	case url := <-urlCh:
		// Convert https to wss for WebSocket
		t.publicURL = strings.Replace(url, "https://", "wss://", 1)
		return t, nil
	case err := <-errCh:
		cancel()
		return nil, err
	case <-time.After(30 * time.Second):
		cancel()
		return nil, fmt.Errorf("timeout waiting for tunnel URL (30s)")
	}
}

// PublicURL returns the public WebSocket URL of the tunnel.
func (t *Tunnel) PublicURL() string {
	return t.publicURL
}

// Stop shuts down the tunnel.
func (t *Tunnel) Stop() {
	if t.cancel != nil {
		t.cancel()
	}
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
	}
}

func downloadFile(dst, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	tmpPath := dst + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	written, err := io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		return err
	}

	fmt.Fprintf(os.Stderr, "  ✓ Scaricati %.1f MB\n", float64(written)/(1024*1024))

	return os.Rename(tmpPath, dst)
}
