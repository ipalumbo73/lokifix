@echo off
title Wolfix - AI Diagnostic Toolkit

set "USB_ROOT=%~dp0"
set "USB_ROOT=%USB_ROOT:~0,-1%"

echo.
echo  ============================================
echo              W O L F I X
echo         ^>_ AI Problem Solver
echo           with Claude Code
echo  ============================================
echo.

set "NODE_DIR=%USB_ROOT%\runtime\node-win-x64"
if not exist "%NODE_DIR%\node.exe" (
    echo [ERRORE] Node.js non trovato in %NODE_DIR%
    echo Esegui prima setup-usb.ps1 per preparare la chiavetta.
    pause
    exit /b 1
)

set "CLAUDE_BIN=%USB_ROOT%\claude-code\claude.cmd"
if not exist "%CLAUDE_BIN%" (
    echo [ERRORE] Claude Code non trovato.
    echo Esegui prima setup-usb.ps1 per preparare la chiavetta.
    pause
    exit /b 1
)

set "PATH=%NODE_DIR%;%USB_ROOT%\claude-code;%PATH%"
set "NPM_CONFIG_PREFIX=%USB_ROOT%\claude-code"
set "CLAUDE_CONFIG_DIR=%USB_ROOT%\config"
set "NODE_PATH=%USB_ROOT%\claude-code\lib\node_modules"

echo [OK] Ambiente configurato.
echo.

:menu
echo  --------------------------------------------
echo    [1] Diagnosi completa del sistema
echo    [2] Claude Code interattivo
echo    [3] Analizza file di log
echo    [4] Fix guidato
echo    [5] Raccogli dati per analisi offline
echo    [6] Connetti a server remoto SSH
echo    [7] Diagnosi rete
echo    [8] Analisi sicurezza
echo    [0] Esci
echo  --------------------------------------------
echo.
set "CHOICE="
set /p "CHOICE=Scelta: "

if "%CHOICE%"=="1" goto diagnosi
if "%CHOICE%"=="2" goto interattivo
if "%CHOICE%"=="3" goto analizza_log
if "%CHOICE%"=="4" goto fix_guidato
if "%CHOICE%"=="5" goto raccogli_dati
if "%CHOICE%"=="6" goto ssh_remoto
if "%CHOICE%"=="7" goto diagnosi_rete
if "%CHOICE%"=="8" goto analisi_sicurezza
if "%CHOICE%"=="0" goto fine
echo Scelta non valida.
echo.
goto menu

:diagnosi
echo.
echo Avvio diagnosi completa...
echo Per uscire e tornare al menu: scrivi /exit oppure premi Ctrl+C
echo.
call "%CLAUDE_BIN%" "Diagnostica questo sistema Windows: servizi, disco, RAM, CPU, Event Log, rete, DNS, aggiornamenti. Proponi fix e chiedi conferma."
echo.
echo Tornato al menu.
echo.
goto menu

:interattivo
echo.
echo Per uscire e tornare al menu: scrivi /exit oppure premi Ctrl+C
echo.
call "%CLAUDE_BIN%"
echo.
echo Tornato al menu.
echo.
goto menu

:analizza_log
echo.
set "LOGPATH="
set /p "LOGPATH=Percorso del file di log: "
if "%LOGPATH%"=="" goto menu
if not exist "%LOGPATH%" (
    echo File non trovato.
    goto menu
)
echo Per uscire e tornare al menu: scrivi /exit oppure premi Ctrl+C
echo.
call "%CLAUDE_BIN%" "Analizza questo file di log, identifica errori e anomalie. File: %LOGPATH%"
echo.
echo Tornato al menu.
echo.
goto menu

:fix_guidato
echo.
set "PROBLEMA="
set /p "PROBLEMA=Descrivi il problema: "
if "%PROBLEMA%"=="" goto menu
echo Per uscire e tornare al menu: scrivi /exit oppure premi Ctrl+C
echo.
call "%CLAUDE_BIN%" "Diagnostica e ripara questo problema: %PROBLEMA%. Esegui comandi diagnostici, identifica la causa, proponi il fix e chiedi conferma prima di applicarlo."
echo.
echo Tornato al menu.
echo.
goto menu

:raccogli_dati
echo.
echo Raccolta dati di sistema...
powershell -ExecutionPolicy Bypass -File "%USB_ROOT%\toolkit\scripts\collect-win.ps1" -OutputDir "%USB_ROOT%\toolkit\logs"
echo Dati salvati in %USB_ROOT%\toolkit\logs
echo.
goto menu

:ssh_remoto
echo.
set "SSH_HOST="
set /p "SSH_HOST=Host (user@ip): "
if "%SSH_HOST%"=="" goto menu
echo Per uscire e tornare al menu: scrivi /exit oppure premi Ctrl+C
echo.
call "%CLAUDE_BIN%" "Collegati via SSH a %SSH_HOST% e diagnostica il sistema remoto. Proponi fix e chiedi conferma."
echo.
echo Tornato al menu.
echo.
goto menu

:diagnosi_rete
echo.
echo Avvio diagnosi rete...
echo Per uscire e tornare al menu: scrivi /exit oppure premi Ctrl+C
echo.
call "%CLAUDE_BIN%" "Esegui una diagnosi completa della rete su questo sistema Windows: interfacce di rete, configurazione IP, DNS, gateway, tabella routing, porte in ascolto, connessioni attive, firewall rules, test connettivita' verso internet e DNS. Identifica problemi e proponi fix."
echo.
echo Tornato al menu.
echo.
goto menu

:analisi_sicurezza
echo.
echo Avvio analisi sicurezza...
echo Per uscire e tornare al menu: scrivi /exit oppure premi Ctrl+C
echo.
call "%CLAUDE_BIN%" "Esegui un'analisi di sicurezza di questo sistema Windows: utenti e gruppi locali, policy password, servizi in esecuzione come SYSTEM, porte aperte, firewall, antivirus, aggiornamenti mancanti, share di rete, task schedulati sospetti, autorun. Segnala vulnerabilita' e proponi remediation."
echo.
echo Tornato al menu.
echo.
goto menu

:fine
echo Arrivederci.
timeout /t 2 >nul
exit /b 0
