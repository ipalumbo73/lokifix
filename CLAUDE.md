# LokiFix Remote Agent

Sistema a due componenti per operare su macchine Windows remote tramite Claude Code, senza installare Claude Code sulla macchina remota.

## Architettura

```
[PC Operatore]                              [PC Remoto]
Claude Code ←stdio→ lokifix-mcp.exe         lokifix-agent.exe
                     ↕ WebSocket/TLS          ↕
                     cloudflared tunnel  ←→  connessione in uscita
```

## Componenti

- `build/lokifix-mcp.exe` — MCP server per Claude Code (lato operatore)
- `build/lokifix-agent.exe` — Agent portabile (lato remoto, singolo .exe)

## Setup rapido

```powershell
.\setup.ps1
```

Lo script compila, configura Claude Code, e scarica cloudflared.

## Configurazione manuale MCP

In `~/.claude/claude_desktop_config.json`:
```json
{
  "mcpServers": {
    "lokifix-remote": {
      "command": "C:\\percorso\\build\\lokifix-mcp.exe"
    }
  }
}
```

## Tool remoti disponibili (19 tool)

| Tool | Descrizione |
|------|-------------|
| `remote_shell` | Esecuzione comandi PowerShell/CMD (con campo description per audit) |
| `remote_file_read` | Lettura file con numeri di riga, offset e limit |
| `remote_file_write` | Scrittura file (auto-crea directory) |
| `remote_file_edit` | Sostituzione stringa in file (supporta replace_all) |
| `remote_file_list` | Elenco directory con metadati |
| `remote_file_delete` | Eliminazione file/directory (ricorsivo, con conferma) |
| `remote_file_upload` | Trasferimento file operatore → remoto (base64, binari, max 50MB) |
| `remote_file_download` | Trasferimento file remoto → operatore (base64, binari, max 50MB) |
| `remote_glob` | Ricerca file per pattern (supporta ** ricorsivo, sort by modtime) |
| `remote_grep` | Ricerca regex completa (output_mode, case_insensitive, context, type filter, head_limit, multiline) |
| `remote_sysinfo` | Info sistema (OS, CPU, RAM, uptime) |
| `remote_processes` | Processi in esecuzione (top 50) |
| `remote_services` | Servizi Windows |
| `remote_registry` | Lettura registro Windows |
| `remote_netinfo` | Interfacce di rete |
| `remote_env_vars` | Variabili d'ambiente |
| `remote_installed_software` | Software installato |
| `remote_event_log` | Event log Windows |
| `remote_connection_status` | Stato connessione agente (locale, non richiede agente) |

## Build

```bash
go build -ldflags="-s -w" -o build/lokifix-agent.exe ./cmd/lokifix-agent/
go build -ldflags="-s -w" -o build/lokifix-mcp.exe ./cmd/lokifix-mcp/
```

## Test

```bash
go test ./... -v
```

## Struttura progetto

```
cmd/lokifix-agent/    # Entrypoint agent remoto
cmd/lokifix-mcp/      # Entrypoint MCP server
internal/
  protocol/           # Tipi e messaggi condivisi
  auth/               # Generazione codice connessione e token
  transport/          # WebSocket client/server
  tunnel/             # Integrazione cloudflared
  executor/           # Esecuzione comandi Windows
  fileops/            # Operazioni su file, glob, grep, transfer chunked
  sysinfo/            # Info sistema, processi, servizi, registry
  mcp/                # Protocollo MCP per Claude Code
  agent/              # Handler dei tool remoti
  ui/                 # Tema Loki: colori ANSI adattivi, banner, componenti UI
  crypto/             # Crittografia E2E: AES-256-GCM con derivazione chiave HKDF
```
