package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ivanpalumbo/lokifix/internal/agent"
	"github.com/ivanpalumbo/lokifix/internal/audit"
	"github.com/ivanpalumbo/lokifix/internal/auth"
	"github.com/ivanpalumbo/lokifix/internal/transport"
)

const banner = `
 ╦  ╔═╗╦╔═╦╔═╗╦═╗ ╦
 ║  ║ ║╠╩╗║╠╣ ║╔╩╦╝
 ╩═╝╚═╝╩ ╩╩╚  ╩╩ ╚═
  Remote Agent v1.1.0
`

const maxRetries = 5

func main() {
	fmt.Print(banner)
	fmt.Println("  Agente remoto per LokiFix")
	fmt.Println("  Ogni azione dell'operatore viene registrata.")
	fmt.Println("  Le operazioni pericolose richiedono la tua approvazione.")
	fmt.Println()
	fmt.Println("  ─────────────────────────────────────")

	connectionCode := readConnectionCode()
	if connectionCode == "" {
		fmt.Println("\n  ✗ Codice di connessione vuoto")
		os.Exit(1)
	}

	serverURL, token, err := auth.DecodeConnectionInfo(connectionCode)
	if err != nil {
		fmt.Printf("\n  ✗ Codice non valido: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n\n  Disconnessione in corso...")
		cancel()
	}()

	// Generate session ID from the connection code (first 8 chars)
	sessionID := connectionCode[:min(8, len(connectionCode))]

	// Set up audit logger
	logDir := getLogDir()
	logger, err := audit.NewLogger("REMOTE", logDir, sessionID)
	if err != nil {
		fmt.Printf("  ⚠ Impossibile creare audit log: %v\n", err)
		fmt.Println("  Le azioni NON verranno registrate.")
	} else {
		fmt.Printf("  📋 Audit log: %s\n", logger.FilePath())

		// Show actions in real-time on console
		logger.OnEntry = func(entry audit.Entry) {
			ts := entry.Timestamp.Format("15:04:05")
			symbol := "✓"
			if entry.Result == "ERROR" {
				symbol = "✗"
			} else if entry.Result == "DENIED" {
				symbol = "⊘"
			}

			detail := entry.Detail
			if len(detail) > 80 {
				detail = detail[:80] + "..."
			}

			fmt.Printf("  %s [%s] %s %-15s %s\n", symbol, ts, entry.Result, entry.Action, detail)
		}
	}

	// Create handler with logging and confirmation
	handlerCfg := agent.HandlerConfig{
		Logger: logger,
		ConfirmFunc: func(action, detail, reason string) bool {
			return askConfirmation(action, detail, reason)
		},
	}
	handler := agent.NewHandler(handlerCfg)

	connectAndRun(ctx, serverURL, token, handler, logger, sessionID)

	// Session summary
	if logger != nil {
		logger.Log(audit.ActionDisconnect, "Sessione terminata", "OK")
		reportPath, err := logger.WriteReport()
		if err == nil {
			fmt.Printf("\n  📄 Report sessione: %s\n", reportPath)
		}
		fmt.Println(logger.SessionSummary())
		logger.Close()
	}

	fmt.Println("  Premi Invio per chiudere...")
	bufio.NewReader(os.Stdin).ReadByte()
}

func connectAndRun(ctx context.Context, serverURL, token string, handler transport.RequestHandler, logger *audit.Logger, sessionID string) {
	retries := 0

	for {
		if ctx.Err() != nil {
			return
		}

		fmt.Printf("\n  Connessione a %s...\n", maskURL(serverURL))

		client := transport.NewClient(serverURL, token, handler)

		if err := client.Connect(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			retries++
			if retries > maxRetries {
				fmt.Printf("  ✗ Connessione fallita dopo %d tentativi: %v\n", maxRetries, err)
				return
			}
			delay := time.Duration(retries*2) * time.Second
			fmt.Printf("  ✗ Connessione fallita: %v\n", err)
			fmt.Printf("  Nuovo tentativo tra %v... (%d/%d)\n", delay, retries, maxRetries)
			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				return
			}
		}

		retries = 0

		hostname, _ := os.Hostname()
		fmt.Printf("  ✓ Connesso! Hostname: %s\n", hostname)

		if logger != nil {
			logger.Log(audit.ActionConnect, fmt.Sprintf("Connesso a %s come %s", maskURL(serverURL), hostname), "OK")
		}

		fmt.Println()
		fmt.Println("  ─────────────────────────────────────")
		fmt.Println("  L'operatore può ora gestire questa macchina.")
		fmt.Println("  Le azioni appariranno qui sotto in tempo reale.")
		fmt.Println("  Operazioni pericolose richiedono la tua conferma.")
		fmt.Println("  Premi Ctrl+C per disconnetterti.")
		fmt.Println("  ─────────────────────────────────────")
		fmt.Println()

		err := client.Run(ctx)
		client.Close()

		if ctx.Err() != nil {
			fmt.Println("\n  Disconnesso dall'utente.")
			return
		}

		if err != nil {
			fmt.Printf("\n  ⚠ Connessione persa: %v\n", err)
			fmt.Println("  Tentativo di riconnessione...")
			time.Sleep(2 * time.Second)
			continue
		}

		return
	}
}

func askConfirmation(action, detail, reason string) bool {
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════════════╗")
	fmt.Println("  ║  ⚠  OPERAZIONE PERICOLOSA - CONFERMA RICHIESTA ║")
	fmt.Println("  ╠══════════════════════════════════════════════════╣")
	fmt.Printf("  ║  Azione:  %s\n", action)

	// Print detail with word wrap
	if len(detail) > 45 {
		fmt.Printf("  ║  Dettaglio: %s\n", detail[:45])
		for i := 45; i < len(detail); i += 45 {
			end := i + 45
			if end > len(detail) {
				end = len(detail)
			}
			fmt.Printf("  ║             %s\n", detail[i:end])
		}
	} else {
		fmt.Printf("  ║  Dettaglio: %s\n", detail)
	}

	fmt.Printf("  ║  Motivo:  %s\n", reason)
	fmt.Println("  ╚══════════════════════════════════════════════════╝")
	fmt.Print("  Approvi? [s/N]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	approved := input == "s" || input == "si" || input == "sì" || input == "y" || input == "yes"

	if approved {
		fmt.Println("  → Approvato")
	} else {
		fmt.Println("  → Negato")
	}
	fmt.Println()

	return approved
}

func readConnectionCode() string {
	if len(os.Args) > 1 {
		return strings.TrimSpace(strings.Join(os.Args[1:], ""))
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\n  Inserisci il codice di connessione:\n  > ")
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func maskURL(url string) string {
	parts := strings.Split(url, "//")
	if len(parts) < 2 {
		return "***"
	}
	host := strings.Split(parts[1], "/")[0]
	hostParts := strings.Split(host, ".")
	if len(hostParts) > 2 && len(hostParts[0]) > 3 {
		return hostParts[0][:3] + "***." + strings.Join(hostParts[len(hostParts)-2:], ".")
	}
	return host
}

func getLogDir() string {
	// Try to use a lokifix-logs directory next to the executable
	exePath, err := os.Executable()
	if err == nil {
		logDir := filepath.Join(filepath.Dir(exePath), "lokifix-logs")
		if err := os.MkdirAll(logDir, 0700); err == nil {
			return logDir
		}
	}
	// Fallback to temp
	return filepath.Join(os.TempDir(), "lokifix-logs")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
