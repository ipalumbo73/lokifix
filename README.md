# LokiFix  >_ AI Problem Solver with Anthropic

**Portable AI-powered diagnostic toolkit for Windows, Linux, macOS, and VMware systems.**

*Powered by Anthropic*

---

## What It Is

LokiFix is a portable USB toolkit that diagnoses and repairs IT systems interactively. Named after Loki, the Norse god who could take any form, LokiFix adapts to any system it encounters. It runs entirely from a USB drive -- no installation required on the target machine. Plug it in, launch the menu, and let the diagnostic engine guide you through system diagnostics, log analysis, security audits, and guided repairs.

## Features

- **Zero installation** -- runs entirely from USB, leaves no trace on the target system
- **Multi-platform** -- supports Windows, Linux, macOS (Intel and Apple Silicon), ESXi, and VMware vCenter from a single drive
- **Interactive diagnostics** -- AI-driven analysis of services, disk, RAM, CPU, event logs, network, and updates
- **Guided repair** -- proposes fixes with clear explanations and asks for confirmation before applying
- **Log analysis** -- parses any log file to identify errors, anomalies, and patterns
- **Security auditing** -- checks users, permissions, open ports, firewall rules, and known vulnerabilities
- **Network diagnostics** -- full stack analysis from interfaces to DNS to firewall
- **Remote access** -- connect to remote servers via SSH for diagnosis and repair
- **Offline data collection** -- gathers system data for later analysis on an air-gapped machine

## Supported Platforms

| Platform | Versions | Mode |
|----------|----------|------|
| Windows Desktop | 10, 11 | Full diagnostic and repair |
| Windows Server | 2016, 2019, 2022, 2025 | Full diagnostic and repair |
| Windows Server (legacy) | 2008, 2012 | Data collection only |
| Linux | All major distributions | Full diagnostic and repair |
| macOS (Intel) | 11+ (Big Sur and later) | Full diagnostic and repair |
| macOS (Apple Silicon) | 11+ (Big Sur and later) | Full diagnostic and repair |
| VMware ESXi | 6.x, 7.x, 8.x | Diagnostic and health check |
| VMware vCenter | 6.x, 7.x, 8.x | Diagnostic and health check |

## Quick Start

### Prerequisites

- Internet connection on the target machine (for API calls)
- An API key or subscription (configured during setup)
- A USB drive with at least 1 GB of free space

### Setup (one-time, from your main PC)

```powershell
.\setup-usb.ps1 -UsbDrive E
```

This downloads the required runtimes (Node.js, Git Portable), installs the diagnostic engine, and copies all toolkit files to the USB drive. You will be prompted to authenticate during setup.

### Usage

Plug the USB drive into the target machine, then:

- **Windows (cmd):** double-click `launch.bat`
- **Windows (PowerShell):** run `.\launch.ps1`
- **Linux / ESXi:** run `bash launch.sh`
- **macOS:** open Terminal, run `bash launch.sh` (auto-detects Darwin)

## Menu Options

| # | Option | Description |
|---|--------|-------------|
| 1 | Full System Diagnosis | Checks services, disk, RAM, CPU, event logs, network, updates, and firewall |
| 2 | Interactive Session | Opens a free-form session for custom queries |
| 3 | Analyze Log File | Parses a log file to find errors, warnings, and anomalies |
| 4 | Guided Fix | Describe a problem and get a step-by-step diagnosis and repair |
| 5 | Collect Data for Offline Analysis | Gathers system information and saves it to the USB drive |
| 6 | Connect to Remote Server (SSH) | Diagnoses and repairs a remote system over SSH |
| 7 | Network Diagnosis | Analyzes interfaces, IP, DNS, routing, ports, and firewall rules |
| 8 | Security Analysis | Audits users, permissions, open ports, services, and scheduled tasks |
| 9 | Safely Eject USB | Flushes buffers and safely ejects the USB drive |
| 0 | Exit | Closes the toolkit |

## Project Structure

```
lokifix/
├── launch.bat                         # Windows CMD launcher
├── launch.ps1                         # Windows PowerShell launcher
├── launch.sh                          # Linux / macOS / ESXi launcher
├── lokifix-eject.ps1                  # Safe USB eject script
├── setup-usb.ps1                      # One-time USB setup script
├── toolkit/
│   ├── prompts/
│   │   ├── windows-health.md          # Windows diagnostic prompt
│   │   ├── linux-health.md            # Linux diagnostic prompt
│   │   ├── macos-health.md            # macOS diagnostic prompt
│   │   ├── esxi-health.md            # ESXi diagnostic prompt
│   │   ├── vmware-health.md           # vCenter diagnostic prompt
│   │   └── server-2008-2012.md        # Legacy server data collection prompt
│   ├── scripts/
│   │   ├── collect-win.ps1            # Windows data collection script
│   │   ├── collect-linux.sh           # Linux data collection script
│   │   ├── collect-macos.sh           # macOS data collection script
│   │   └── collect-esxi.sh           # ESXi data collection script
│   └── logs/                          # Collected data output directory
├── runtime/                           # Portable runtimes (created by setup)
│   ├── node-win-x64/
│   ├── node-linux-x64/
│   ├── node-darwin-x64/               # macOS Intel
│   ├── node-darwin-arm64/             # macOS Apple Silicon (M1/M2/M3/M4)
│   └── git-win-x64/                   # Git Portable for Windows
├── claude-code/                       # Diagnostic engine (created by setup)
└── config/                            # Authentication and configuration (created by setup)
```

## How It Works

LokiFix bundles a portable Node.js runtime, Git Portable (for Windows), and an AI diagnostic engine on a USB drive. When launched, it temporarily adds these to the system PATH -- no need for anything to be installed on the target machine. The engine then runs diagnostic commands, reads their output, identifies problems, and proposes targeted fixes -- all through an interactive conversation. Every destructive action requires explicit user confirmation before execution.

The toolkit includes pre-built diagnostic prompts for each supported platform. These prompts instruct the engine to perform a structured analysis covering services, storage, memory, logs, networking, and security. You can also open an interactive session to ask anything or describe a specific problem for guided troubleshooting.

## Zero Footprint -- Nothing Left Behind

LokiFix is designed so that **nothing is installed on the target machine**. The runtime, engine, configuration files, and authentication credentials all live on the USB drive. The launcher scripts only set temporary environment variables (`PATH`, `NODE_PATH`) that exist in the current shell session and disappear the moment the terminal is closed.

When you remove the USB drive, the target machine is exactly as it was before:

- **No software required** -- even Git runs from the USB drive
- **No files written to disk** -- no binaries, no libraries, no config files
- **No registry changes** (Windows) -- no entries added or modified
- **No system services installed** -- no daemons, no launch agents, no scheduled tasks
- **No PATH modifications** -- the temporary PATH change is lost when the shell session ends
- **No credentials stored locally** -- API keys and authentication tokens stay on the USB drive in the `config/` directory

This makes LokiFix ideal for technicians working on client machines where you cannot (or should not) install software, and for environments where compliance requires that no third-party tools are left behind after an intervention.

## Requirements

- **USB drive:** 2 GB minimum free space (Git Portable adds ~300 MB)
- **Internet connection:** required on the target machine for API communication
- **Authentication:** API key or subscription (configured during setup)
- **Windows:** PowerShell 5.1+ (for setup and PowerShell launcher)
- **Linux:** Bash, tar (for Node.js extraction on first run)
- **macOS:** Bash, tar (for Node.js extraction on first run); supports both Intel and Apple Silicon

## Security Note

The USB drive stores your authentication credentials in the `config/` directory. Consider encrypting the drive with BitLocker (Windows) or LUKS (Linux) to protect your credentials if the drive is lost or stolen.

## Disclaimer

LokiFix is powered by Anthropic AI technology. The diagnostic engine uses AI to analyze systems, identify problems, and propose solutions. All destructive actions require explicit user confirmation. Use at your own risk and always verify proposed fixes before applying them to production systems.

## Contributing

Contributions are welcome. Please open an issue to discuss proposed changes before submitting a pull request. Follow existing code conventions and test on at least one supported platform before submitting.

## License

This project is licensed under the [MIT License](LICENSE).
