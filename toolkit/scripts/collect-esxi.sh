#!/usr/bin/env bash
#
# Raccoglie dati diagnostici da un host ESXi via SSH.
# Esegui questo script dalla chiavetta su una macchina con accesso SSH all'host ESXi.
#
# Uso: bash collect-esxi.sh <esxi_host> <user> [output_dir]
#
# Esempio: bash collect-esxi.sh 192.168.1.100 root ./toolkit/logs
#

set -uo pipefail

ESXI_HOST="${1:-}"
ESXI_USER="${2:-root}"
OUTPUT_DIR="${3:-./toolkit/logs}"

if [ -z "$ESXI_HOST" ]; then
    echo "Uso: $0 <esxi_host> [user] [output_dir]"
    echo "Esempio: $0 192.168.1.100 root ./toolkit/logs"
    exit 1
fi

TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
OUTPUT_FILE="${OUTPUT_DIR}/esxi_${ESXI_HOST}_${TIMESTAMP}.txt"

mkdir -p "$OUTPUT_DIR"

echo "[*] Raccolta dati diagnostici ESXi: $ESXI_HOST"
echo "    Utente: $ESXI_USER"
echo "    Output: $OUTPUT_FILE"
echo ""
echo "NOTA: Verra' richiesta la password SSH."
echo ""

# Comandi da eseguire sull'host ESXi (BusyBox-compatible)
REMOTE_COMMANDS=$(cat <<'ESXI_EOF'
echo "DIAGNOSTICA ESXI"
echo "================"
echo "Data: $(date)"
echo ""

echo "============================================================"
echo "### VERSIONE ESXI"
echo "============================================================"
vmware -v 2>/dev/null
esxcli system version get 2>/dev/null

echo ""
echo "============================================================"
echo "### HOSTNAME E NETWORK"
echo "============================================================"
esxcli system hostname get 2>/dev/null
esxcli network ip interface ipv4 get 2>/dev/null
esxcli network ip dns server list 2>/dev/null

echo ""
echo "============================================================"
echo "### UPTIME E RISORSE"
echo "============================================================"
uptime 2>/dev/null
esxcli hardware memory get 2>/dev/null
esxcli hardware cpu global get 2>/dev/null

echo ""
echo "============================================================"
echo "### DATASTORE"
echo "============================================================"
esxcli storage filesystem list 2>/dev/null

echo ""
echo "============================================================"
echo "### VM IN ESECUZIONE"
echo "============================================================"
esxcli vm process list 2>/dev/null

echo ""
echo "============================================================"
echo "### STATO ADATTATORI DI RETE"
echo "============================================================"
esxcli network nic list 2>/dev/null
esxcli network vswitch standard list 2>/dev/null

echo ""
echo "============================================================"
echo "### STATO HARDWARE (sensori)"
echo "============================================================"
esxcli hardware platform get 2>/dev/null

echo ""
echo "============================================================"
echo "### LOG ERRORI RECENTI"
echo "============================================================"
tail -100 /var/log/vmkernel.log 2>/dev/null | grep -i "error\|warning\|fail\|critical" | tail -30

echo ""
echo "============================================================"
echo "### SERVIZI"
echo "============================================================"
esxcli system service list 2>/dev/null | head -40

echo ""
echo "============================================================"
echo "### FIREWALL"
echo "============================================================"
esxcli network firewall get 2>/dev/null
esxcli network firewall ruleset list 2>/dev/null | grep -i "true"

echo ""
echo "============================================================"
echo "### NTP"
echo "============================================================"
esxcli system ntp get 2>/dev/null

echo ""
echo "============================================================"
echo "### STORAGE ADAPTER"
echo "============================================================"
esxcli storage core adapter list 2>/dev/null

echo ""
echo "============================================================"
echo "### PATCH / VIB INSTALLATI (ultimi 10)"
echo "============================================================"
esxcli software vib list 2>/dev/null | tail -10
ESXI_EOF
)

ssh -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new "${ESXI_USER}@${ESXI_HOST}" "$REMOTE_COMMANDS" > "$OUTPUT_FILE" 2>&1

if [ $? -eq 0 ]; then
    FILE_SIZE=$(du -k "$OUTPUT_FILE" 2>/dev/null | cut -f1)
    echo ""
    echo "[COMPLETATO] Report salvato: $OUTPUT_FILE"
    echo "  Dimensione: ${FILE_SIZE} KB"
    echo ""
    echo "Ora puoi analizzare il report con Claude Code:"
    echo "  claude -p \"Analizza questo report ESXi e identifica problemi: $OUTPUT_FILE\""
else
    echo ""
    echo "[ERRORE] Connessione SSH fallita verso $ESXI_HOST"
    echo "Verifica: host raggiungibile, SSH abilitato, credenziali corrette."
fi
