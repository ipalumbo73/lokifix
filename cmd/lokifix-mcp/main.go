package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ivanpalumbo/lokifix/internal/audit"
	"github.com/ivanpalumbo/lokifix/internal/auth"
	"github.com/ivanpalumbo/lokifix/internal/mcp"
	"github.com/ivanpalumbo/lokifix/internal/transport"
	"github.com/ivanpalumbo/lokifix/internal/tunnel"
)

const connectionCodeFile = "lokifix-connection.txt"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	noTunnel := hasFlag("--no-tunnel")
	standalone := hasFlag("--standalone")

	authMgr := auth.NewManager()
	wsServer := transport.NewServer(authMgr)

	port, err := wsServer.Start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
		os.Exit(1)
	}

	// Determine server URL
	serverURL := fmt.Sprintf("ws://localhost:%d", port)

	// Start tunnel unless disabled
	var tun *tunnel.Tunnel
	if !noTunnel {
		if envURL := os.Getenv("LOKIFIX_TUNNEL_URL"); envURL != "" {
			serverURL = envURL
			fmt.Fprintf(os.Stderr, "  Usando tunnel URL da env: %s\n", envURL)
		} else {
			tun, err = startTunnel(ctx, port)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  ⚠ Tunnel non disponibile: %v\n", err)
				fmt.Fprintf(os.Stderr, "  Usando connessione locale (ws://localhost:%d)\n", port)
			} else {
				serverURL = tun.PublicURL()
				defer tun.Stop()
				fmt.Fprintf(os.Stderr, "  ✓ Tunnel attivo: %s\n", serverURL)
			}
		}
	}

	// Generate connection code
	cc, err := authMgr.GenerateCode()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate code: %v\n", err)
		os.Exit(1)
	}

	sessionID := cc.Code
	connectionStr := auth.EncodeConnectionInfo(serverURL, cc.Token)

	// Set up operator-side audit logger
	logDir := getOperatorLogDir()
	logger, logErr := audit.NewLogger("OPERATOR", logDir, sessionID)
	if logErr != nil {
		fmt.Fprintf(os.Stderr, "  ⚠ Audit log non disponibile: %v\n", logErr)
	} else {
		fmt.Fprintf(os.Stderr, "  📋 Audit log operatore: %s\n", logger.FilePath())
	}

	// Write connection code to file
	codeFilePath := writeConnectionFile(connectionStr, cc.Code, serverURL)

	// Print connection info to stderr
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "╔══════════════════════════════════════════════════╗")
	fmt.Fprintln(os.Stderr, "║          LokiFix MCP Server v1.1.0              ║")
	fmt.Fprintln(os.Stderr, "╠══════════════════════════════════════════════════╣")
	fmt.Fprintf(os.Stderr, "║  Porta locale:  %-32d ║\n", port)
	if tun != nil {
		fmt.Fprintf(os.Stderr, "║  Tunnel:        %-32s ║\n", truncate(serverURL, 32))
	}
	fmt.Fprintln(os.Stderr, "║                                                  ║")
	fmt.Fprintln(os.Stderr, "║  CODICE DI CONNESSIONE:                          ║")
	for i := 0; i < len(connectionStr); i += 48 {
		end := i + 48
		if end > len(connectionStr) {
			end = len(connectionStr)
		}
		fmt.Fprintf(os.Stderr, "║  %-48s ║\n", connectionStr[i:end])
	}
	fmt.Fprintln(os.Stderr, "║                                                  ║")
	fmt.Fprintf(os.Stderr, "║  Sessione: %-37s ║\n", cc.Code)
	fmt.Fprintf(os.Stderr, "║  Scade: %-40s ║\n", time.Now().Add(15*time.Minute).Format("15:04:05"))
	fmt.Fprintf(os.Stderr, "║  File: %-41s ║\n", truncate(codeFilePath, 41))
	fmt.Fprintln(os.Stderr, "╠══════════════════════════════════════════════════╣")
	fmt.Fprintln(os.Stderr, "║  ⏳ In attesa dell'agente remoto...              ║")
	fmt.Fprintln(os.Stderr, "╚══════════════════════════════════════════════════╝")

	// Connection callbacks
	wsServer.OnAgentConnected = func(hostname string) {
		fmt.Fprintf(os.Stderr, "\n  ✓ Agente connesso: %s\n", hostname)
		fmt.Fprintln(os.Stderr, "    I tool remoti sono ora disponibili in Claude Code.")
		os.Remove(codeFilePath)
		if logger != nil {
			logger.Log(audit.ActionConnect, "Agent connesso: "+hostname, "OK")
		}
	}

	wsServer.OnAgentDisconnected = func() {
		fmt.Fprintln(os.Stderr, "\n  ✗ Agente disconnesso.")
		if logger != nil {
			logger.Log(audit.ActionDisconnect, "Agent disconnesso", "OK")
			reportPath, err := logger.WriteReport()
			if err == nil {
				fmt.Fprintf(os.Stderr, "  📄 Report sessione: %s\n", reportPath)
			}
		}
		// Generate new code for reconnection
		newCC, err := authMgr.GenerateCode()
		if err == nil {
			newConnStr := auth.EncodeConnectionInfo(serverURL, newCC.Token)
			writeConnectionFile(newConnStr, newCC.Code, serverURL)
			fmt.Fprintf(os.Stderr, "  Nuovo codice generato: %s\n", newCC.Code)
		}
	}

	if standalone {
		fmt.Fprintln(os.Stderr, "\n  Modalità standalone — in attesa di connessioni...")
		<-ctx.Done()
		cleanup(wsServer, logger)
		return
	}

	// Run MCP server on stdio
	mcpServer := mcp.NewMCPServer(wsServer, logger)
	if err := mcpServer.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
	}

	cleanup(wsServer, logger)
}

func cleanup(wsServer *transport.Server, logger *audit.Logger) {
	wsServer.Stop()
	if logger != nil {
		logger.Log(audit.ActionDisconnect, "MCP server terminato", "OK")
		logger.WriteReport()
		logger.Close()
	}
}

func startTunnel(ctx context.Context, localPort int) (*tunnel.Tunnel, error) {
	exeDir, _ := os.Executable()
	baseDir := filepath.Dir(exeDir)
	if baseDir == "" || baseDir == "." {
		baseDir, _ = os.Getwd()
	}

	cloudflaredPath, err := tunnel.EnsureCloudflared(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cloudflared setup: %w", err)
	}

	fmt.Fprintf(os.Stderr, "  Avvio tunnel cloudflare...\n")
	return tunnel.Start(ctx, cloudflaredPath, localPort)
}

func writeConnectionFile(connectionStr, sessionCode, serverURL string) string {
	homeDir, _ := os.UserHomeDir()
	filePath := filepath.Join(homeDir, connectionCodeFile)

	content := fmt.Sprintf(`LokiFix Remote Agent - Codice di Connessione
=============================================

Codice completo (da incollare nell'agent remoto):
%s

Sessione: %s
Server:   %s
Generato: %s

ISTRUZIONI:
1. Copia lokifix-agent.exe sul PC remoto
2. Esegui lokifix-agent.exe
3. Incolla il codice completo qui sopra
`, connectionStr, sessionCode, serverURL, time.Now().Format("2006-01-02 15:04:05"))

	os.WriteFile(filePath, []byte(content), 0600)
	return filePath
}

func getOperatorLogDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "lokifix-logs")
}

func hasFlag(flag string) bool {
	for _, arg := range os.Args[1:] {
		if arg == flag {
			return true
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
