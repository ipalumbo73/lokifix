#!/usr/bin/env bash
#
# Wolfix - AI Diagnostic Toolkit - Launcher Linux
# Configura l'ambiente dalla chiavetta USB e lancia Claude Code.
#

set -euo pipefail

# === AUTO-DETECT USB ROOT ===
USB_ROOT="$(cd "$(dirname "$0")" && pwd)"

# === COLORI ===
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
GRAY='\033[0;37m'
NC='\033[0m'

# === DETECT ARCHITETTURA ===
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  NODE_DIR="$USB_ROOT/runtime/node-linux-x64" ;;
    aarch64) NODE_DIR="$USB_ROOT/runtime/node-linux-arm64" ;;
    *)
        echo -e "${RED}[ERRORE] Architettura non supportata: $ARCH${NC}"
        exit 1
        ;;
esac

# === VERIFICA NODE.JS ===
# Se il .tar.xz non e' ancora estratto, prova a farlo
if [ ! -f "$NODE_DIR/bin/node" ]; then
    TAR_FILE=$(find "$NODE_DIR" -name "*.tar.xz" 2>/dev/null | head -1)
    if [ -n "$TAR_FILE" ]; then
        echo -e "${YELLOW}[*] Estrazione Node.js...${NC}"
        tar -xf "$TAR_FILE" -C "$NODE_DIR" --strip-components=1
        echo -e "${GREEN}[OK] Node.js estratto.${NC}"
    else
        echo -e "${RED}[ERRORE] Node.js non trovato in $NODE_DIR${NC}"
        echo "Esegui setup-usb.ps1 su Windows per preparare la chiavetta."
        exit 1
    fi
fi

# === FIX PERMESSI (exFAT non ha permessi Unix) ===
chmod +x "$NODE_DIR/bin/node" 2>/dev/null || true
chmod +x "$NODE_DIR/bin/npm" 2>/dev/null || true

CLAUDE_BIN="$USB_ROOT/claude-code/bin/claude"
if [ -f "$CLAUDE_BIN" ]; then
    chmod +x "$CLAUDE_BIN" 2>/dev/null || true
fi

# === CONFIGURA AMBIENTE ===
export PATH="$NODE_DIR/bin:$USB_ROOT/claude-code/bin:$PATH"
export NPM_CONFIG_PREFIX="$USB_ROOT/claude-code"
export CLAUDE_CONFIG_DIR="$USB_ROOT/config"
export NODE_PATH="$USB_ROOT/claude-code/lib/node_modules"

# === VERIFICA CLAUDE CODE ===
if ! command -v claude &>/dev/null; then
    echo -e "${RED}[ERRORE] Claude Code non trovato.${NC}"
    echo "Installa con: npm install -g @anthropic-ai/claude-code --prefix $USB_ROOT/claude-code"
    exit 1
fi

# === DETECT SISTEMA ===
OS_NAME=$(cat /etc/os-release 2>/dev/null | grep "^PRETTY_NAME=" | cut -d'"' -f2 || uname -s)
KERNEL=$(uname -r)
RAM_GB=$(free -g 2>/dev/null | awk '/Mem:/{print $2}' || echo "N/A")
HOSTNAME_VAL=$(hostname)

# === BANNER ===
show_banner() {
    echo ""
    echo -e "${CYAN}  â•”â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•—${NC}"
    echo -e "${CYAN}  â•’            W O L F I X                    â•’${NC}"
    echo -e "${CYAN}  ║       >_ AI Problem Solver                ║${NC}"
    echo -e "${CYAN}  ║         with Claude Code                  ║${NC}"
    echo -e "${CYAN}  â•šâ•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•${NC}"
    echo ""
    echo -e "${GRAY}  Sistema: $OS_NAME${NC}"
    echo -e "${GRAY}  Kernel:  $KERNEL${NC}"
    echo -e "${GRAY}  RAM:     ${RAM_GB} GB${NC}"
    echo -e "${GRAY}  Host:    $HOSTNAME_VAL${NC}"
    echo ""
}

# === MENU ===
show_menu() {
    echo -e "${YELLOW}  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”${NC}"
    echo -e "${YELLOW}  â”‚  [1] Diagnosi completa del sistema      â”‚${NC}"
    echo -e "${YELLOW}  â”‚  [2] Claude Code interattivo            â”‚${NC}"
    echo -e "${YELLOW}  â”‚  [3] Analizza file di log               â”‚${NC}"
    echo -e "${YELLOW}  â”‚  [4] Fix guidato (descrivi problema)    â”‚${NC}"
    echo -e "${YELLOW}  â”‚  [5] Raccogli dati per analisi offline  â”‚${NC}"
    echo -e "${YELLOW}  â”‚  [6] Connetti a server remoto (SSH)     â”‚${NC}"
    echo -e "${YELLOW}  â”‚  [7] Diagnosi rete                      â”‚${NC}"
    echo -e "${YELLOW}  â”‚  [8] Analisi sicurezza                  â”‚${NC}"
    echo -e "${YELLOW}  â”‚  [0] Esci                               â”‚${NC}"
    echo -e "${YELLOW}  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜${NC}"
}

# === FUNZIONI ===
do_diagnosi() {
    echo -e "${GREEN}[*] Avvio diagnosi completa...${NC}"
    claude -p "Sei un esperto di diagnostica sistemi Linux. Questo sistema e':
- OS: $OS_NAME
- Kernel: $KERNEL
- RAM: ${RAM_GB} GB
- Hostname: $HOSTNAME_VAL

Esegui una diagnosi completa:
1. Controlla servizi critici (systemctl o service)
2. Verifica spazio disco (df, inodes)
3. Analizza utilizzo RAM e CPU (free, top/ps)
4. Cerca errori in syslog/journalctl (ultime 24h)
5. Verifica stato rete (ip, DNS, routing)
6. Controlla aggiornamenti pendenti
7. Verifica cron job falliti
8. Controlla mount points e fstab

Per ogni problema trovato: spiega l'impatto, proponi il fix, chiedi conferma PRIMA di eseguirlo."
}

do_analizza_log() {
    echo -n "Percorso del file di log: "
    read -r log_path
    if [ ! -f "$log_path" ]; then
        echo -e "${RED}[ERRORE] File non trovato: $log_path${NC}"
        return
    fi
    claude -p "Analizza il file di log '$log_path'. Identifica errori, warning, pattern anomali. Fornisci un riepilogo strutturato e suggerisci soluzioni."
}

do_fix_guidato() {
    echo -n "Descrivi il problema: "
    read -r problema
    claude -p "Sei un esperto di diagnostica e riparazione sistemi Linux.
Sistema: $OS_NAME ($KERNEL) - $HOSTNAME_VAL

Problema: $problema

1. Diagnostica con i comandi necessari
2. Identifica la causa root
3. Proponi il fix, chiedi conferma
4. Applica e verifica"
}

do_raccogli_dati() {
    local script_path="$USB_ROOT/toolkit/scripts/collect-linux.sh"
    if [ -f "$script_path" ]; then
        chmod +x "$script_path" 2>/dev/null || true
        bash "$script_path" "$USB_ROOT/toolkit/logs"
        echo -e "${GREEN}[OK] Dati salvati in $USB_ROOT/toolkit/logs${NC}"
    else
        echo -e "${RED}[ERRORE] Script non trovato: $script_path${NC}"
    fi
}

do_ssh_remoto() {
    echo -n "Host (user@ip): "
    read -r ssh_host
    # Modalita interattiva invece di -p
    claude "Collegati via SSH a $ssh_host. Diagnostica: OS, servizi, disco, memoria, log errori. Per ogni problema proponi fix e chiedi conferma."
}

do_diagnosi_rete() {
    claude -p "Diagnosi completa rete Linux: interfacce, IP, DNS, routing, porte in ascolto (ss/netstat), connessioni attive, firewall (iptables/nftables/firewalld), test connettivita'. Identifica problemi e proponi fix."
}

do_analisi_sicurezza() {
    claude -p "Analisi sicurezza Linux: utenti/gruppi, sudoers, SUID/SGID, porte aperte, servizi esposti, SSH config, fail2ban, aggiornamenti sicurezza, permessi file sensibili (/etc/shadow, /etc/passwd), crontab sospetti, processi anomali. Segnala vulnerabilita' e proponi remediation."
}

# === MAIN ===
show_banner

while true; do
    show_menu
    echo ""
    echo -n "  Scelta: "
    read -r choice
    echo ""

    case "$choice" in
        1) do_diagnosi ;;
        2) claude ;;
        3) do_analizza_log ;;
        4) do_fix_guidato ;;
        5) do_raccogli_dati ;;
        6) do_ssh_remoto ;;
        7) do_diagnosi_rete ;;
        8) do_analisi_sicurezza ;;
        0)
            echo -e "${GREEN}Arrivederci. Nessuna traccia lasciata sul sistema.${NC}"
            exit 0
            ;;
        *) echo -e "${RED}Scelta non valida.${NC}" ;;
    esac
    echo ""
done
