package ui

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// ANSI color codes for Loki theme
const (
	// Rich colors (ANSI 256) - used when terminal supports it
	richGreen  = "\033[38;5;35m"  // Emerald green - primary
	richGold   = "\033[38;5;178m" // Gold/amber - accent
	richRed    = "\033[38;5;196m" // Flame red - danger
	richCyan   = "\033[38;5;80m"  // Light cyan - info/progress
	richGray   = "\033[38;5;245m" // Muted gray - secondary
	richWhite  = "\033[38;5;255m" // Bright white - text
	richBold   = "\033[1m"
	richReset  = "\033[0m"
)

// Theme holds the active color set.
type Theme struct {
	Green  string
	Gold   string
	Red    string
	Cyan   string
	Gray   string
	White  string
	Bold   string
	Reset  string
	active bool // true if ANSI colors are active
}

var activeTheme Theme

// Init detects terminal capabilities and initializes the theme.
// Must be called once at startup.
func Init() {
	if enableANSI() {
		activeTheme = Theme{
			Green:  richGreen,
			Gold:   richGold,
			Red:    richRed,
			Cyan:   richCyan,
			Gray:   richGray,
			White:  richWhite,
			Bold:   richBold,
			Reset:  richReset,
			active: true,
		}
	} else {
		activeTheme = Theme{
			active: false,
		}
	}
}

// T returns the active theme.
func T() Theme { return activeTheme }

// IsColor returns true if ANSI colors are active.
func IsColor() bool { return activeTheme.active }

// enableANSI tries to enable ANSI escape processing on Windows.
func enableANSI() bool {
	// Check if explicitly disabled
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode := kernel32.NewProc("GetConsoleMode")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")

	handle, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err != nil {
		return false
	}

	var mode uint32
	r, _, _ := getConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&mode)))
	if r == 0 {
		return false
	}

	// ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004
	const enableVTP = 0x0004
	r, _, _ = setConsoleMode.Call(uintptr(handle), uintptr(mode|enableVTP))
	return r != 0
}

// --- Convenience formatting functions ---

// Green formats text in emerald green.
func Green(s string) string {
	if !activeTheme.active {
		return s
	}
	return activeTheme.Green + s + activeTheme.Reset
}

// Gold formats text in gold/amber.
func Gold(s string) string {
	if !activeTheme.active {
		return s
	}
	return activeTheme.Gold + s + activeTheme.Reset
}

// Red formats text in flame red.
func Red(s string) string {
	if !activeTheme.active {
		return s
	}
	return activeTheme.Red + s + activeTheme.Reset
}

// Cyan formats text in light cyan.
func Cyan(s string) string {
	if !activeTheme.active {
		return s
	}
	return activeTheme.Cyan + s + activeTheme.Reset
}

// Gray formats text in muted gray.
func Gray(s string) string {
	if !activeTheme.active {
		return s
	}
	return activeTheme.Gray + s + activeTheme.Reset
}

// Bold formats text in bold.
func BoldText(s string) string {
	if !activeTheme.active {
		return s
	}
	return activeTheme.Bold + s + activeTheme.Reset
}

// BoldGreen formats text in bold emerald green.
func BoldGreen(s string) string {
	if !activeTheme.active {
		return s
	}
	return activeTheme.Bold + activeTheme.Green + s + activeTheme.Reset
}

// BoldGold formats text in bold gold.
func BoldGold(s string) string {
	if !activeTheme.active {
		return s
	}
	return activeTheme.Bold + activeTheme.Gold + s + activeTheme.Reset
}

// BoldRed formats text in bold red.
func BoldRed(s string) string {
	if !activeTheme.active {
		return s
	}
	return activeTheme.Bold + activeTheme.Red + s + activeTheme.Reset
}

// --- UI Components ---

// ProgressBar returns a visual progress bar string.
func ProgressBar(current, total int, width int) string {
	if total <= 0 {
		total = 1
	}
	pct := current * 100 / total
	filled := width * pct / 100

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	pctStr := fmt.Sprintf("%3d%%", pct)

	if activeTheme.active {
		return activeTheme.Cyan + bar + activeTheme.Reset + " " + activeTheme.White + pctStr + activeTheme.Reset
	}
	return bar + " " + pctStr
}

// StatusSymbol returns a themed status symbol.
func StatusSymbol(status string) string {
	switch status {
	case "OK":
		return Green("✓")
	case "ERROR":
		return Red("✗")
	case "DENIED":
		return Gray("⊘")
	case "PROGRESS":
		return Cyan("⚡")
	case "APPROVED":
		return Gold("✓")
	default:
		return Gray("·")
	}
}

// Separator returns a themed horizontal separator.
func Separator(width int) string {
	line := strings.Repeat("─", width)
	if activeTheme.active {
		return activeTheme.Gray + line + activeTheme.Reset
	}
	return line
}
