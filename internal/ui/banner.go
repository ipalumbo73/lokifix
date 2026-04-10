package ui

import "fmt"

const version = "1.2.0"

// PrintBanner prints the themed LokiFix banner.
func PrintBanner() {
	if activeTheme.active {
		printColorBanner()
	} else {
		printPlainBanner()
	}
}

func printColorBanner() {
	t := activeTheme
	w := 42 // inner width

	title := "   <  L . O . K . I . F . I . X  >   "
	sep := "   ----------------------------------   "
	ver := fmt.Sprintf("       Remote Agent  v%s", version)
	powered := "    Powered by Claude - Anthropic"

	// Helper: prints a banner row with green borders and colored content
	row := func(color, text string) {
		fmt.Printf("  %s|%s%s%s%s|%s\n",
			t.Green,                  // 1: left | in green
			color,                    // 2: content color
			centerPad(text, w),       // 3: padded content
			t.Reset,                  // 4: reset content color
			t.Green,                  // 5: right | in green
			t.Reset)                  // 6: final reset
	}

	fmt.Println()
	fmt.Printf("  %s+%s+%s\n", t.Green, pad('-', w), t.Reset)
	row("", "")
	row(t.Bold+t.Gold, title)
	row(t.Gray, sep)
	row(t.White, ver)
	row(t.Cyan, powered)
	row("", "")
	fmt.Printf("  %s+%s+%s\n", t.Green, pad('-', w), t.Reset)
	fmt.Println()
}

func printPlainBanner() {
	w := 42

	title := "   <  L . O . K . I . F . I . X  >   "
	sep := "   ----------------------------------   "
	ver := fmt.Sprintf("       Remote Agent  v%s", version)
	powered := "    Powered by Claude - Anthropic"

	fmt.Println()
	fmt.Printf("  +%s+\n", pad('-', w))
	fmt.Printf("  |%s|\n", pad(' ', w))
	fmt.Printf("  |%s|\n", centerPad(title, w))
	fmt.Printf("  |%s|\n", centerPad(sep, w))
	fmt.Printf("  |%s|\n", centerPad(ver, w))
	fmt.Printf("  |%s|\n", centerPad(powered, w))
	fmt.Printf("  |%s|\n", pad(' ', w))
	fmt.Printf("  +%s+\n", pad('-', w))
	fmt.Println()
}

// pad returns a string of n copies of character c.
func pad(c byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return string(b)
}

// padRight pads a string with spaces to exactly width characters.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + pad(' ', width-len(s))
}

// centerPad centers a string within the given width, padding both sides with spaces.
func centerPad(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	total := width - len(s)
	left := total / 2
	right := total - left
	return pad(' ', left) + s + pad(' ', right)
}

// PrintConnectionHeader prints the status header after successful connection.
func PrintConnectionHeader(hostname string) {
	if activeTheme.active {
		t := activeTheme
		fmt.Printf("  %sᛚ%s %sConnesso%s %s|%s %s%s%s %s|%s Operatore attivo\n",
			t.Gold, t.Reset,
			t.Bold+t.Green, t.Reset,
			t.Gray, t.Reset,
			t.White, hostname, t.Reset,
			t.Gray, t.Reset)
	} else {
		fmt.Printf("  ᛚ Connesso | %s | Operatore attivo\n", hostname)
	}
	fmt.Printf("  %s\n", Separator(42))
	fmt.Println()
}

// PrintDisconnectMessage prints a themed disconnect message.
func PrintDisconnectMessage() {
	fmt.Println()
	if activeTheme.active {
		t := activeTheme
		fmt.Printf("  %sᛚ%s %sDisconnesso%s\n", t.Gold, t.Reset, t.Gray, t.Reset)
	} else {
		fmt.Println("  ᛚ Disconnesso")
	}
}

// PrintRetry prints a retry attempt message.
func PrintRetry(attempt, max int, err error, delaySec int) {
	if activeTheme.active {
		t := activeTheme
		fmt.Printf("  %s✗%s Connessione fallita: %s%v%s\n", t.Red, t.Reset, t.Gray, err, t.Reset)
		fmt.Printf("  %s↻%s Nuovo tentativo tra %ds... %s(%d/%d)%s\n",
			t.Cyan, t.Reset, delaySec, t.Gray, attempt, max, t.Reset)
	} else {
		fmt.Printf("  ✗ Connessione fallita: %v\n", err)
		fmt.Printf("  ↻ Nuovo tentativo tra %ds... (%d/%d)\n", delaySec, attempt, max)
	}
}

// PrintFatalRetry prints a final retry failure message.
func PrintFatalRetry(max int, err error) {
	if activeTheme.active {
		t := activeTheme
		fmt.Printf("  %s✗ Connessione fallita dopo %d tentativi: %v%s\n", t.Red, max, err, t.Reset)
	} else {
		fmt.Printf("  ✗ Connessione fallita dopo %d tentativi: %v\n", max, err)
	}
}

// PrintInfo prints a themed informational message.
func PrintInfo(label, value string) {
	if activeTheme.active {
		t := activeTheme
		fmt.Printf("  %s%s%s %s\n", t.Cyan, label, t.Reset, value)
	} else {
		fmt.Printf("  %s %s\n", label, value)
	}
}

// PrintInstructions prints the post-connection instructions.
func PrintInstructions() {
	fmt.Println()
	if activeTheme.active {
		t := activeTheme
		fmt.Printf("  %s┌─────────────────────────────────────────┐%s\n", t.Gray, t.Reset)
		fmt.Printf("  %s│%s L'operatore puo ora gestire questa      %s│%s\n", t.Gray, t.Reset, t.Gray, t.Reset)
		fmt.Printf("  %s│%s macchina. Le azioni appariranno qui     %s│%s\n", t.Gray, t.Reset, t.Gray, t.Reset)
		fmt.Printf("  %s│%s sotto in tempo reale.                   %s│%s\n", t.Gray, t.Reset, t.Gray, t.Reset)
		fmt.Printf("  %s│%s                                         %s│%s\n", t.Gray, t.Reset, t.Gray, t.Reset)
		fmt.Printf("  %s│%s %s⚠%s  Operazioni pericolose richiedono    %s│%s\n", t.Gray, t.Reset, t.Gold, t.Reset, t.Gray, t.Reset)
		fmt.Printf("  %s│%s    la tua conferma.                     %s│%s\n", t.Gray, t.Reset, t.Gray, t.Reset)
		fmt.Printf("  %s│%s %s⏎%s  Premi Ctrl+C per disconnetterti.   %s│%s\n", t.Gray, t.Reset, t.Cyan, t.Reset, t.Gray, t.Reset)
		fmt.Printf("  %s└─────────────────────────────────────────┘%s\n", t.Gray, t.Reset)
	} else {
		fmt.Println("  ┌─────────────────────────────────────────┐")
		fmt.Println("  │ L'operatore puo ora gestire questa      │")
		fmt.Println("  │ macchina. Le azioni appariranno qui     │")
		fmt.Println("  │ sotto in tempo reale.                   │")
		fmt.Println("  │                                         │")
		fmt.Println("  │ ⚠  Operazioni pericolose richiedono    │")
		fmt.Println("  │    la tua conferma.                     │")
		fmt.Println("  │ ⏎  Premi Ctrl+C per disconnetterti.   │")
		fmt.Println("  └─────────────────────────────────────────┘")
	}
	fmt.Println()
}

// PrintConfirmDialog prints a themed dangerous operation confirmation dialog.
func PrintConfirmDialog(action, detail, reason string) {
	fmt.Println()
	if activeTheme.active {
		t := activeTheme
		fmt.Printf("  %s╔══════════════════════════════════════════════════╗%s\n", t.Red, t.Reset)
		fmt.Printf("  %s║%s  %s⚠  OPERAZIONE PERICOLOSA%s                        %s║%s\n",
			t.Red, t.Reset, t.Bold+t.Red, t.Reset, t.Red, t.Reset)
		fmt.Printf("  %s╠══════════════════════════════════════════════════╣%s\n", t.Red, t.Reset)
		fmt.Printf("  %s║%s  %sAzione:%s   %-40s%s║%s\n",
			t.Red, t.Reset, t.Gold, t.Reset, action, t.Red, t.Reset)

		// Detail with word wrap
		printWrappedDetail(detail, t)

		fmt.Printf("  %s║%s  %sMotivo:%s   %-40s%s║%s\n",
			t.Red, t.Reset, t.Gray, t.Reset, reason, t.Red, t.Reset)
		fmt.Printf("  %s╚══════════════════════════════════════════════════╝%s\n", t.Red, t.Reset)
	} else {
		fmt.Println("  ╔══════════════════════════════════════════════════╗")
		fmt.Println("  ║  ⚠  OPERAZIONE PERICOLOSA                       ║")
		fmt.Println("  ╠══════════════════════════════════════════════════╣")
		fmt.Printf("  ║  Azione:   %-40s║\n", action)
		if len(detail) > 40 {
			fmt.Printf("  ║  Dettaglio: %-38s║\n", detail[:38])
			for i := 38; i < len(detail); i += 38 {
				end := i + 38
				if end > len(detail) {
					end = len(detail)
				}
				fmt.Printf("  ║             %-38s║\n", detail[i:end])
			}
		} else {
			fmt.Printf("  ║  Dettaglio: %-38s║\n", detail)
		}
		fmt.Printf("  ║  Motivo:   %-40s║\n", reason)
		fmt.Println("  ╚══════════════════════════════════════════════════╝")
	}
}

func printWrappedDetail(detail string, t Theme) {
	if len(detail) > 38 {
		fmt.Printf("  %s║%s  %sDettaglio:%s %-38s%s║%s\n",
			t.Red, t.Reset, t.White, t.Reset, detail[:38], t.Red, t.Reset)
		for i := 38; i < len(detail); i += 38 {
			end := i + 38
			if end > len(detail) {
				end = len(detail)
			}
			fmt.Printf("  %s║%s             %-38s%s║%s\n",
				t.Red, t.Reset, detail[i:end], t.Red, t.Reset)
		}
	} else {
		fmt.Printf("  %s║%s  %sDettaglio:%s %-38s%s║%s\n",
			t.Red, t.Reset, t.White, t.Reset, detail, t.Red, t.Reset)
	}
}

// FormatLogEntry formats a single audit log entry for console display.
func FormatLogEntry(timestamp, result, action, detail string) string {
	// Truncate detail
	if len(detail) > 60 {
		detail = detail[:57] + "..."
	}

	symbol := StatusSymbol(result)

	if activeTheme.active {
		t := activeTheme
		return fmt.Sprintf("  %s %s%s%s %-17s %s%s%s",
			symbol,
			t.Gray, timestamp, t.Reset,
			Gold(action),
			t.White, detail, t.Reset)
	}

	// Plain fallback
	sym := "[OK]"
	switch result {
	case "ERROR":
		sym = "[!!]"
	case "DENIED":
		sym = "[NO]"
	}
	return fmt.Sprintf("  %s %s %-17s %s", sym, timestamp, action, detail)
}

// PrintPrompt prints the themed confirmation prompt.
func PrintPrompt() {
	if activeTheme.active {
		t := activeTheme
		fmt.Printf("  %sApprovi?%s %s[s/N]%s: ", t.Bold+t.White, t.Reset, t.Gray, t.Reset)
	} else {
		fmt.Print("  Approvi? [s/N]: ")
	}
}

// PrintApproved prints the themed approval message.
func PrintApproved() {
	fmt.Printf("  %s\n\n", Green("→ Approvato"))
}

// PrintDenied prints the themed denial message.
func PrintDenied() {
	fmt.Printf("  %s\n\n", Red("→ Negato"))
}

// PrintSessionReport prints a themed session summary to console.
func PrintSessionReport(summary string) {
	fmt.Println()
	if activeTheme.active {
		t := activeTheme
		fmt.Printf("  %s◈%s %sRiepilogo sessione%s\n", t.Gold, t.Reset, t.Bold+t.White, t.Reset)
		fmt.Printf("  %s\n", Separator(42))
		fmt.Printf("  %s%s%s\n", t.Gray, summary, t.Reset)
	} else {
		fmt.Println("  ◈ Riepilogo sessione")
		fmt.Printf("  %s\n", Separator(42))
		fmt.Println(summary)
	}
}
