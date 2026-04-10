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
	"github.com/ivanpalumbo/lokifix/internal/ui"
)

const maxRetries = 5

func main() {
	// Initialize themed UI (detect ANSI support)
	ui.Init()

	ui.PrintBanner()

	connectionCode := readConnectionCode()
	if connectionCode == "" {
		fmt.Printf("\n  %s Codice di connessione vuoto\n", ui.Red("✗"))
		os.Exit(1)
	}

	serverURL, token, err := auth.DecodeConnectionInfo(connectionCode)
	if err != nil {
		fmt.Printf("\n  %s Codice non valido: %v\n", ui.Red("✗"), err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		ui.PrintDisconnectMessage()
		cancel()
	}()

	// Generate session ID from the connection code (first 8 chars)
	sessionID := connectionCode[:min(8, len(connectionCode))]

	// Set up audit logger
	logDir := getLogDir()
	logger, err := audit.NewLogger("REMOTE", logDir, sessionID)
	if err != nil {
		fmt.Printf("  %s Impossibile creare audit log: %v\n", ui.Gold("⚠"), err)
		fmt.Println("  Le azioni NON verranno registrate.")
	} else {
		ui.PrintInfo("📋 Audit log:", logger.FilePath())

		// Show actions in real-time on console with themed formatting
		logger.OnEntry = func(entry audit.Entry) {
			ts := entry.Timestamp.Format("15:04:05")
			line := ui.FormatLogEntry(ts, entry.Result, entry.Action, entry.Detail)
			fmt.Println(line)
		}
	}

	// Create handler with logging and themed confirmation
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
			ui.PrintInfo("📄 Report sessione:", reportPath)
		}
		ui.PrintSessionReport(logger.SessionSummary())
		logger.Close()
	}

	fmt.Println()
	fmt.Println("  Premi Invio per chiudere...")
	bufio.NewReader(os.Stdin).ReadByte()
}

func connectAndRun(ctx context.Context, serverURL, token string, handler transport.RequestHandler, logger *audit.Logger, sessionID string) {
	retries := 0
	sessionToken := ""

	for {
		if ctx.Err() != nil {
			return
		}

		fmt.Printf("\n  %s Connessione a %s...\n", ui.Cyan("↻"), maskURL(serverURL))

		client := transport.NewClient(serverURL, token, handler)
		if sessionToken != "" {
			client.SetSessionToken(sessionToken)
		}

		if err := client.Connect(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			retries++
			if retries > maxRetries {
				ui.PrintFatalRetry(maxRetries, err)
				return
			}
			delay := retries * 2
			ui.PrintRetry(retries, maxRetries, err, delay)
			select {
			case <-time.After(time.Duration(delay) * time.Second):
				continue
			case <-ctx.Done():
				return
			}
		}

		retries = 0

		hostname, _ := os.Hostname()
		ui.PrintConnectionHeader(hostname)

		if logger != nil {
			logger.Log(audit.ActionConnect, fmt.Sprintf("Connesso a %s come %s", maskURL(serverURL), hostname), "OK")
		}

		ui.PrintInstructions()

		err := client.Run(ctx)
		// Save session token for reconnection before closing
		if st := client.SessionToken(); st != "" {
			sessionToken = st
		}
		client.Close()

		if ctx.Err() != nil {
			ui.PrintDisconnectMessage()
			return
		}

		if err != nil {
			fmt.Printf("\n  %s Connessione persa: %v\n", ui.Gold("⚠"), err)
			fmt.Printf("  %s Tentativo di riconnessione...\n", ui.Cyan("↻"))
			time.Sleep(2 * time.Second)
			continue
		}

		return
	}
}

func askConfirmation(action, detail, reason string) bool {
	ui.PrintConfirmDialog(action, detail, reason)
	ui.PrintPrompt()

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	approved := input == "s" || input == "si" || input == "sì" || input == "y" || input == "yes"

	if approved {
		ui.PrintApproved()
	} else {
		ui.PrintDenied()
	}

	return approved
}

func readConnectionCode() string {
	if len(os.Args) > 1 {
		return strings.TrimSpace(strings.Join(os.Args[1:], ""))
	}

	reader := bufio.NewReader(os.Stdin)
	if ui.IsColor() {
		fmt.Printf("  %s Inserisci il codice di connessione:\n  %s ", ui.Gold("ᛚ"), ui.Green(">"))
	} else {
		fmt.Print("\n  Inserisci il codice di connessione:\n  > ")
	}
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
	exePath, err := os.Executable()
	if err == nil {
		logDir := filepath.Join(filepath.Dir(exePath), "lokifix-logs")
		if err := os.MkdirAll(logDir, 0700); err == nil {
			return logDir
		}
	}
	return filepath.Join(os.TempDir(), "lokifix-logs")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
