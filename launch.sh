#!/usr/bin/env bash
#
# LokiFix - AI Problem Solver with Anthropic - Launcher Linux/macOS
# Configura l'ambiente dalla chiavetta USB e lancia il motore diagnostico.
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
DARKGRAY='\033[1;30m'
NC='\033[0m'

# === LOKIFIX IDENTITY PREFIX ===
LOKI_PREFIX_IT="Sei LokiFix, un motore diagnostico portatile creato per tecnici IT. Non menzionare mai Claude, Anthropic, o modelli di linguaggio AI. Se ti chiedono chi sei, rispondi che sei LokiFix. Rispondi sempre in modo professionale e utile."
LOKI_PREFIX_EN="You are LokiFix, a portable diagnostic engine built for IT technicians. Never mention Claude, Anthropic, or AI language models. If asked who you are, say you are LokiFix. Always be professional and helpful."
LOKI_PREFIX="$LOKI_PREFIX_IT"

# === DETECT OS E ARCHITETTURA ===
OS_TYPE=$(uname -s)
ARCH=$(uname -m)

case "$OS_TYPE" in
    Linux)
        case "$ARCH" in
            x86_64)  NODE_DIR="$USB_ROOT/runtime/node-linux-x64" ;;
            aarch64) NODE_DIR="$USB_ROOT/runtime/node-linux-arm64" ;;
            *)
                echo -e "${RED}[ERROR] Unsupported architecture: $ARCH${NC}"
                exit 1
                ;;
        esac
        ;;
    Darwin)
        case "$ARCH" in
            x86_64)  NODE_DIR="$USB_ROOT/runtime/node-darwin-x64" ;;
            arm64)   NODE_DIR="$USB_ROOT/runtime/node-darwin-arm64" ;;
            *)
                echo -e "${RED}[ERROR] Unsupported architecture: $ARCH${NC}"
                exit 1
                ;;
        esac
        ;;
    *)
        echo -e "${RED}[ERROR] Unsupported OS: $OS_TYPE${NC}"
        exit 1
        ;;
esac

# === VERIFICA NODE.JS ===
if [ ! -f "$NODE_DIR/bin/node" ]; then
    TAR_FILE=$(find "$NODE_DIR" -name "*.tar.xz" -o -name "*.tar.gz" 2>/dev/null | head -1)
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

# === VERIFICA MOTORE ===
if ! command -v claude &>/dev/null; then
    echo -e "${RED}[ERRORE] Motore LokiFix non trovato.${NC}"
    echo "Esegui setup-usb.ps1 su Windows per preparare la chiavetta."
    exit 1
fi

# === DETECT SISTEMA ===
if [ "$OS_TYPE" = "Darwin" ]; then
    OS_NAME=$(sw_vers -productName 2>/dev/null || echo "macOS")
    OS_VERSION=$(sw_vers -productVersion 2>/dev/null || echo "unknown")
    OS_NAME="$OS_NAME $OS_VERSION"
    KERNEL=$(uname -r)
    RAM_GB=$(sysctl -n hw.memsize 2>/dev/null | awk '{printf "%.0f", $1/1073741824}' || echo "N/A")
else
    OS_NAME=$(cat /etc/os-release 2>/dev/null | grep "^PRETTY_NAME=" | cut -d'"' -f2 || uname -s)
    KERNEL=$(uname -r)
    RAM_GB=$(free -g 2>/dev/null | awk '/Mem:/{print $2}' || echo "N/A")
fi
HOSTNAME_VAL=$(hostname)

# === LANGUAGE ===
set_language() {
    if [ "$1" = "en" ]; then
        LOKI_PREFIX="$LOKI_PREFIX_EN"
        M1="[1] Full system diagnosis"
        M2="[2] Interactive session"
        M3="[3] Analyze log file"
        M4="[4] Guided fix (describe problem)"
        M5="[5] Collect data for offline analysis"
        M6="[6] Connect to remote server (SSH)"
        M7="[7] Network diagnosis"
        M8="[8] Security analysis"
        M9="[9] Safely eject USB"
        M0="[0] Exit"
        MSG_CHOICE="Choice: "
        MSG_LOGPATH="Log file path: "
        MSG_PROBLEM="Describe the problem: "
        MSG_SSHHOST="Host (user@ip): "
        MSG_DIAGSTART="[*] Starting full diagnosis..."
        MSG_NETSTART="[*] Starting network diagnosis..."
        MSG_SECSTART="[*] Starting security analysis..."
        MSG_COLLECTING="Collecting system data..."
        MSG_SAVED="[OK] Data saved in"
        MSG_NOTFOUND="[ERROR] File not found:"
        MSG_INVALID="Invalid choice."
        MSG_BYE="Goodbye. No traces left on the system."
        MSG_EJECT_SYNC="Flushing buffers..."
        MSG_EJECT_OK="USB safely ejected. You can remove the drive now."
        MSG_EJECT_FAIL="Could not eject the USB drive. Close all open files and try again."
    else
        LOKI_PREFIX="$LOKI_PREFIX_IT"
        M1="[1] Diagnosi completa del sistema"
        M2="[2] Sessione interattiva"
        M3="[3] Analizza file di log"
        M4="[4] Fix guidato (descrivi problema)"
        M5="[5] Raccogli dati per analisi offline"
        M6="[6] Connetti a server remoto (SSH)"
        M7="[7] Diagnosi rete"
        M8="[8] Analisi sicurezza"
        M9="[9] Sgancia chiavetta USB"
        M0="[0] Esci"
        MSG_CHOICE="Scelta: "
        MSG_LOGPATH="Percorso del file di log: "
        MSG_PROBLEM="Descrivi il problema: "
        MSG_SSHHOST="Host (user@ip): "
        MSG_DIAGSTART="[*] Avvio diagnosi completa..."
        MSG_NETSTART="[*] Avvio diagnosi rete..."
        MSG_SECSTART="[*] Avvio analisi sicurezza..."
        MSG_COLLECTING="Raccolta dati di sistema..."
        MSG_SAVED="[OK] Dati salvati in"
        MSG_NOTFOUND="[ERRORE] File non trovato:"
        MSG_INVALID="Scelta non valida."
        MSG_BYE="Arrivederci. Nessuna traccia lasciata sul sistema."
        MSG_EJECT_SYNC="Scaricamento buffer in corso..."
        MSG_EJECT_OK="Chiavetta USB sganciata in sicurezza. Puoi rimuoverla."
        MSG_EJECT_FAIL="Impossibile sganciare la chiavetta. Chiudi tutti i file aperti e riprova."
    fi
}

# Default to Italian
set_language "it"

# === BANNER ===
show_banner() {
    echo ""
    echo -e "${GREEN}  ================================================${NC}"
    echo -e "${GREEN}   _       ___  _  _____ _____ ___ __  __${NC}"
    echo -e "${GREEN}  | |     / _ \\| |/ /_ _|  ___|_ _\\ \\/ /${NC}"
    echo -e "${GREEN}  | |    | | | | ' / | || |_   | | \\  / ${NC}"
    echo -e "${GREEN}  | |___ | |_| | . \\ | ||  _|  | | /  \\ ${NC}"
    echo -e "${GREEN}  |_____| \\___/|_|\\_\\___|_|   |___/_/\\_\\${NC}"
    echo ""
    echo -e "${GREEN}    >_ AI Problem Solver with Anthropic${NC}"
    echo ""
    echo -e "${DARKGRAY}    v0.2.0${NC}"
    echo -e "${GREEN}  ================================================${NC}"
    echo ""
    echo -e "${GRAY}  Sistema: $OS_NAME${NC}"
    echo -e "${GRAY}  Kernel:  $KERNEL${NC}"
    echo -e "${GRAY}  RAM:     ${RAM_GB} GB${NC}"
    echo -e "${GRAY}  Host:    $HOSTNAME_VAL${NC}"
    echo ""
    echo -e "  ${CYAN}[I]${NC} Italiano  ${CYAN}[E]${NC} English"
    echo ""
    echo -n "  Language / Lingua: "
    read -r lang_choice
    if [ "$lang_choice" = "E" ] || [ "$lang_choice" = "e" ]; then
        set_language "en"
    else
        set_language "it"
    fi
    echo ""
}

# === MENU ===
show_menu() {
    echo -e "${GREEN}  +-----------------------------------------+${NC}"
    printf "${GREEN}  |  %-37s|${NC}\n" "$M1"
    printf "${GREEN}  |  %-37s|${NC}\n" "$M2"
    printf "${GREEN}  |  %-37s|${NC}\n" "$M3"
    printf "${GREEN}  |  %-37s|${NC}\n" "$M4"
    printf "${GREEN}  |  %-37s|${NC}\n" "$M5"
    printf "${GREEN}  |  %-37s|${NC}\n" "$M6"
    printf "${GREEN}  |  %-37s|${NC}\n" "$M7"
    printf "${GREEN}  |  %-37s|${NC}\n" "$M8"
    printf "${GREEN}  |  %-37s|${NC}\n" "$M9"
    printf "${GREEN}  |  %-37s|${NC}\n" "$M0"
    echo -e "${GREEN}  +-----------------------------------------+${NC}"
}

# === FUNZIONI ===
do_interattivo() {
    claude --system-prompt "$LOKI_PREFIX"
}

do_diagnosi() {
    echo -e "${GREEN}${MSG_DIAGSTART}${NC}"
    if [ "$OS_TYPE" = "Darwin" ]; then
        claude -p "$LOKI_PREFIX This system is:
- OS: $OS_NAME
- Kernel: $KERNEL
- RAM: ${RAM_GB} GB
- Hostname: $HOSTNAME_VAL

Run a complete diagnosis:
1. Check critical services (launchctl list)
2. Check disk space (df -h, diskutil list)
3. Analyze RAM and CPU usage (vm_stat, top -l 1)
4. Check system.log and unified log for errors (last 24h)
5. Check network status (ifconfig, networksetup, DNS, routing)
6. Check pending software updates (softwareupdate -l)
7. Check Time Machine backup status
8. Check startup items and launch agents/daemons

For each problem: explain impact, propose fix, ask confirmation BEFORE applying."
    else
        claude -p "$LOKI_PREFIX Questo sistema e':
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
    fi
}

do_analizza_log() {
    echo -n "$MSG_LOGPATH"
    read -r log_path
    if [ ! -f "$log_path" ]; then
        echo -e "${RED}${MSG_NOTFOUND} $log_path${NC}"
        return
    fi
    claude -p "$LOKI_PREFIX Analizza il file di log '$log_path'. Identifica errori, warning, pattern anomali. Fornisci un riepilogo strutturato e suggerisci soluzioni."
}

do_fix_guidato() {
    echo -n "$MSG_PROBLEM"
    read -r problema
    claude -p "$LOKI_PREFIX Sistema: $OS_NAME ($KERNEL) - $HOSTNAME_VAL

Problema: $problema

1. Diagnostica con i comandi necessari
2. Identifica la causa root
3. Proponi il fix, chiedi conferma
4. Applica e verifica"
}

do_raccogli_dati() {
    if [ "$OS_TYPE" = "Darwin" ]; then
        local script_path="$USB_ROOT/toolkit/scripts/collect-macos.sh"
    else
        local script_path="$USB_ROOT/toolkit/scripts/collect-linux.sh"
    fi
    if [ -f "$script_path" ]; then
        chmod +x "$script_path" 2>/dev/null || true
        bash "$script_path" "$USB_ROOT/toolkit/logs"
        echo -e "${GREEN}${MSG_SAVED} $USB_ROOT/toolkit/logs${NC}"
    else
        echo -e "${RED}[ERROR] Script not found: $script_path${NC}"
    fi
}

do_ssh_remoto() {
    echo -n "$MSG_SSHHOST"
    read -r ssh_host
    claude "$LOKI_PREFIX Collegati via SSH a $ssh_host. Diagnostica: OS, servizi, disco, memoria, log errori. Per ogni problema proponi fix e chiedi conferma."
}

do_diagnosi_rete() {
    echo -e "${GREEN}${MSG_NETSTART}${NC}"
    if [ "$OS_TYPE" = "Darwin" ]; then
        claude -p "$LOKI_PREFIX Complete macOS network diagnosis: interfaces (ifconfig), IP config, DNS (scutil --dns), routing (netstat -rn), listening ports (lsof -i -P), active connections, firewall (socketfilterfw), connectivity test. Identify problems and propose fixes."
    else
        claude -p "$LOKI_PREFIX Diagnosi completa rete Linux: interfacce, IP, DNS, routing, porte in ascolto (ss/netstat), connessioni attive, firewall (iptables/nftables/firewalld), test connettivita'. Identifica problemi e proponi fix."
    fi
}

do_analisi_sicurezza() {
    echo -e "${GREEN}${MSG_SECSTART}${NC}"
    if [ "$OS_TYPE" = "Darwin" ]; then
        claude -p "$LOKI_PREFIX Run a COMPLETE and AUTONOMOUS macOS security analysis without asking for confirmation. Run all checks automatically in sequence. Check: users/groups (dscl), FileVault status, Gatekeeper, SIP (csrutil), firewall, SSH config, open ports, installed profiles (profiles list), suspicious launch agents/daemons, Keychain issues, software updates, remote login, screen sharing, AirDrop settings. Do NOT ask for confirmation, do NOT stop between checks. At the end produce a structured report with severity (CRITICAL/HIGH/MEDIUM/LOW) and remediation for each issue found."
    else
        claude -p "$LOKI_PREFIX Esegui un'analisi di sicurezza COMPLETA e AUTONOMA di questo sistema Linux senza chiedere conferma. Esegui tutti i controlli in sequenza automaticamente. Controlla: utenti/gruppi, sudoers, SUID/SGID, porte aperte, servizi esposti, SSH config, fail2ban, aggiornamenti sicurezza, permessi file sensibili (/etc/shadow, /etc/passwd), crontab sospetti, processi anomali, SELinux/AppArmor, chiavi SSH autorizzate. NON chiedere conferma, NON fermarti tra un controllo e l'altro. Alla fine produci un report strutturato con severita (CRITICO/ALTO/MEDIO/BASSO) e remediation per ogni problema trovato."
    fi
}

do_sgancia_usb() {
    echo -e "${CYAN}${MSG_EJECT_SYNC}${NC}"
    sync
    local mount_point device
    mount_point=$(df "$USB_ROOT" 2>/dev/null | tail -1 | awk '{print $NF}')
    device=$(df "$USB_ROOT" 2>/dev/null | tail -1 | awk '{print $1}')
    local eject_script="/tmp/lokifix-eject.sh"
    local msg_ok="$MSG_EJECT_OK"
    local msg_fail="$MSG_EJECT_FAIL"

    cat > "$eject_script" << EOFSCRIPT
#!/usr/bin/env bash
sleep 2
if [ "$OS_TYPE" = "Darwin" ]; then
    if diskutil eject "$device" 2>/dev/null; then
        echo "$msg_ok"
    else
        echo "$msg_fail"
    fi
else
    ejected=false
    if command -v udisksctl &>/dev/null; then
        if udisksctl unmount -b "$device" 2>/dev/null && \
           udisksctl power-off -b "$device" 2>/dev/null; then
            ejected=true
        fi
    fi
    if [ "\$ejected" = "false" ]; then
        if umount "$mount_point" 2>/dev/null || sudo -n umount "$mount_point" 2>/dev/null; then
            echo "$msg_ok"
        else
            echo "$msg_fail"
        fi
    else
        echo "$msg_ok"
    fi
fi
sleep 3
rm -f "\$0"
EOFSCRIPT
    chmod +x "$eject_script"
    nohup bash "$eject_script" &
    exit 0
}

# === MAIN ===
show_banner

while true; do
    show_menu
    echo ""
    echo -n "  $MSG_CHOICE"
    read -r choice
    echo ""

    case "$choice" in
        1) do_diagnosi ;;
        2) do_interattivo ;;
        3) do_analizza_log ;;
        4) do_fix_guidato ;;
        5) do_raccogli_dati ;;
        6) do_ssh_remoto ;;
        7) do_diagnosi_rete ;;
        8) do_analisi_sicurezza ;;
        9) do_sgancia_usb ;;
        0)
            echo -e "${GREEN}${MSG_BYE}${NC}"
            exit 0
            ;;
        *) echo -e "${RED}${MSG_INVALID}${NC}" ;;
    esac
    echo ""
done
