#!/usr/bin/env bash
#
# Raccoglie dati diagnostici da un sistema Linux.
# Salva tutto in un file di testo sulla chiavetta USB.
#
# Uso: bash collect-linux.sh [output_dir]
#

set -uo pipefail

OUTPUT_DIR="${1:-./toolkit/logs}"
TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
HOSTNAME_VAL=$(hostname)
OUTPUT_FILE="${OUTPUT_DIR}/${HOSTNAME_VAL}_${TIMESTAMP}.txt"

mkdir -p "$OUTPUT_DIR"

section() {
    local title="$1"
    echo ""
    echo "============================================================"
    echo "### $title"
    echo "============================================================"
}

echo "[*] Raccolta dati diagnostici per $HOSTNAME_VAL"
echo "    Output: $OUTPUT_FILE"
echo ""

{
    echo "DIAGNOSTICA SISTEMA LINUX"
    echo "========================="
    echo "Data: $(date '+%Y-%m-%d %H:%M:%S')"
    echo "Hostname: $HOSTNAME_VAL"
    echo "Utente: $(whoami)"

    section "INFORMAZIONI OS"
    cat /etc/os-release 2>/dev/null || echo "os-release non disponibile"
    echo ""
    uname -a

    section "UPTIME E LOAD"
    uptime
    echo ""
    cat /proc/loadavg 2>/dev/null

    section "CPU"
    lscpu 2>/dev/null | grep -E "^(Architecture|CPU\(s\)|Model name|Thread|Core|Socket)" || cat /proc/cpuinfo | head -25

    section "MEMORIA"
    free -h 2>/dev/null || cat /proc/meminfo | head -10

    section "SPAZIO DISCO"
    df -h 2>/dev/null
    echo ""
    echo "--- Inodes ---"
    df -i 2>/dev/null

    section "MOUNT POINTS"
    mount | grep -v "tmpfs\|cgroup\|proc\|sys\|dev" 2>/dev/null

    section "SERVIZI CRITICI"
    if command -v systemctl &>/dev/null; then
        echo "--- Servizi falliti ---"
        systemctl --failed 2>/dev/null
        echo ""
        echo "--- Servizi attivi ---"
        systemctl list-units --type=service --state=running 2>/dev/null | head -30
    elif command -v service &>/dev/null; then
        service --status-all 2>/dev/null
    fi

    section "LOG ERRORI RECENTI (ultime 24h)"
    if command -v journalctl &>/dev/null; then
        journalctl --since "24 hours ago" -p err --no-pager 2>/dev/null | tail -50
    elif [ -f /var/log/syslog ]; then
        grep -i "error\|critical\|fail" /var/log/syslog 2>/dev/null | tail -50
    elif [ -f /var/log/messages ]; then
        grep -i "error\|critical\|fail" /var/log/messages 2>/dev/null | tail -50
    fi

    section "CONFIGURAZIONE RETE"
    if command -v ip &>/dev/null; then
        ip addr show 2>/dev/null
        echo ""
        echo "--- Routes ---"
        ip route show 2>/dev/null
    else
        ifconfig 2>/dev/null
        route -n 2>/dev/null
    fi
    echo ""
    echo "--- DNS ---"
    cat /etc/resolv.conf 2>/dev/null

    section "PORTE IN ASCOLTO"
    if command -v ss &>/dev/null; then
        ss -tlnp 2>/dev/null
    else
        netstat -tlnp 2>/dev/null
    fi

    section "CONNESSIONI ATTIVE"
    if command -v ss &>/dev/null; then
        ss -tnp 2>/dev/null | head -30
    else
        netstat -tnp 2>/dev/null | head -30
    fi

    section "FIREWALL"
    if command -v firewall-cmd &>/dev/null; then
        firewall-cmd --list-all 2>/dev/null
    elif command -v ufw &>/dev/null; then
        ufw status verbose 2>/dev/null
    elif command -v iptables &>/dev/null; then
        iptables -L -n 2>/dev/null | head -40
    fi

    section "TOP 15 PROCESSI PER MEMORIA"
    ps aux --sort=-%mem 2>/dev/null | head -16

    section "TOP 15 PROCESSI PER CPU"
    ps aux --sort=-%cpu 2>/dev/null | head -16

    section "UTENTI LOGGATI"
    who 2>/dev/null
    echo ""
    echo "--- Ultimi login ---"
    last -10 2>/dev/null

    section "CRONTAB ROOT"
    crontab -l 2>/dev/null || echo "Nessun crontab per root"
    echo ""
    echo "--- Cron system ---"
    ls -la /etc/cron.d/ 2>/dev/null

    section "AGGIORNAMENTI"
    if command -v apt &>/dev/null; then
        apt list --upgradable 2>/dev/null | head -20
    elif command -v yum &>/dev/null; then
        yum check-update 2>/dev/null | tail -20
    elif command -v dnf &>/dev/null; then
        dnf check-update 2>/dev/null | tail -20
    fi

    section "SICUREZZA RAPIDA"
    echo "--- SUID files ---"
    find / -perm -4000 -type f 2>/dev/null | head -20
    echo ""
    echo "--- SSH config ---"
    grep -v "^#\|^$" /etc/ssh/sshd_config 2>/dev/null | head -20
    echo ""
    echo "--- Utenti con shell ---"
    grep -v "nologin\|false" /etc/passwd 2>/dev/null

} > "$OUTPUT_FILE" 2>&1

FILE_SIZE=$(du -k "$OUTPUT_FILE" 2>/dev/null | cut -f1)
echo "[COMPLETATO] Report salvato: $OUTPUT_FILE"
echo "  Dimensione: ${FILE_SIZE} KB"
