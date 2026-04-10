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

## Tool remoti disponibili

| Tool | Descrizione |
|------|-------------|
| `remote_shell` | Esecuzione comandi PowerShell/CMD |
| `remote_file_read` | Lettura file con numeri di riga |
| `remote_file_write` | Scrittura file |
| `remote_file_edit` | Sostituzione stringa in file |
| `remote_file_list` | Elenco directory |
| `remote_file_delete` | Eliminazione file |
| `remote_glob` | Ricerca file per pattern |
| `remote_grep` | Ricerca testo in file |
| `remote_sysinfo` | Info sistema (OS, CPU, RAM, uptime) |
| `remote_processes` | Processi in esecuzione (top 50) |
| `remote_services` | Servizi Windows |
| `remote_registry` | Lettura registro Windows |
| `remote_netinfo` | Interfacce di rete |
| `remote_env_vars` | Variabili d'ambiente |
| `remote_installed_software` | Software installato |
| `remote_event_log` | Event log Windows |

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
  fileops/            # Operazioni su file
  sysinfo/            # Info sistema, processi, servizi, registry
  mcp/                # Protocollo MCP per Claude Code
  agent/              # Handler dei tool remoti
```
