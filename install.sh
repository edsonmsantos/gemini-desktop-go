#!/usr/bin/env bash
# ==============================================================================
# Gemini Desktop - Instalador e Atualizador para Linux
# ==============================================================================
# Script idempotente para instalação inicial ou atualização do Gemini Desktop:
# - Instala o binário em ~/.local/share/geminidesktop (ou /opt se executado como root)
# - Cria links simbólicos em ~/.local/bin (ou /usr/local/bin)
# - Configura o atalho de inicialização no desktop (.desktop) com suporte a URLs
# - Instala os ícones temáticos de alta resolução (16x16 até 512x512)
# - Atualiza caches de desktop e ícones (GTK / XDG)
# - Suporta desinstalação via flag --uninstall
# ==============================================================================

set -euo pipefail

APP_NAME="Gemini Desktop"
APP_ID="geminidesktop-go"
REPO="edsonmsantos/gemini-desktop-go"

# Cores para terminal
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

info() {
    echo -e "${BLUE}${BOLD}[INFO]${NC} $*"
}

success() {
    echo -e "${GREEN}${BOLD}[OK]${NC} $*"
}

warn() {
    echo -e "${YELLOW}${BOLD}[AVISO]${NC} $*"
}

error() {
    echo -e "${RED}${BOLD}[ERRO]${NC} $*" >&2
}

show_help() {
    cat << EOF
Uso: $0 [OPÇÕES]

Opções:
  -u, --update         Atualiza a instalação existente (padrão se já instalado)
  -d, --download       Força o download da versão mais recente do GitHub Releases
  --uninstall          Remove o Gemini Desktop, ícones e atalhos do sistema
  -h, --help           Exibe esta mensagem de ajuda

Exemplos:
  ./install.sh                # Instala ou atualiza usando o build local
  ./install.sh --download     # Baixa e instala a última versão estável do GitHub
  ./install.sh --uninstall    # Remove a aplicação do sistema
EOF
}

# Detecta se é root
if [ "$(id -u)" -eq 0 ]; then
    IS_ROOT=1
    INSTALL_DIR="/opt/geminidesktop"
    BIN_DIR="/usr/local/bin"
    APPS_DIR="/usr/share/applications"
    ICONS_BASE="/usr/share/icons/hicolor"
else
    IS_ROOT=0
    INSTALL_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/geminidesktop"
    BIN_DIR="${XDG_BIN_HOME:-$HOME/.local/bin}"
    APPS_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
    ICONS_BASE="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor"
fi

BIN_TARGET="$INSTALL_DIR/geminidesktop-go"
DESKTOP_TARGET="$APPS_DIR/geminidesktop-go.desktop"

# Diretório base deste script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Processamento de argumentos
DO_DOWNLOAD=0
DO_UNINSTALL=0

while [[ $# -gt 0 ]]; do
    case "$1" in
        -d|--download)
            DO_DOWNLOAD=1
            shift
            ;;
        --uninstall)
            DO_UNINSTALL=1
            shift
            ;;
        -u|--update)
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            warn "Opção desconhecida: $1"
            show_help
            exit 1
            ;;
    esac
done

# ==============================================================================
# MODO DESINSTALAÇÃO
# ==============================================================================
if [ "$DO_UNINSTALL" -eq 1 ]; then
    echo -e "${BOLD}Desinstalando $APP_NAME...${NC}"

    # Encerra processos em execução
    if pgrep -x "$APP_ID" >/dev/null 2>&1; then
        info "Encerrando processo $APP_ID em execução..."
        pkill -TERM -x "$APP_ID" || true
        sleep 1
    fi

    # Remove atalhos e diretórios
    rm -f "$BIN_DIR/geminidesktop-go" "$BIN_DIR/gemini-desktop"
    rm -f "$DESKTOP_TARGET"
    rm -rf "$INSTALL_DIR"

    # Remove ícones
    for size in 16x16 24x24 32x32 48x48 64x64 128x128 256x256 512x512; do
        rm -f "$ICONS_BASE/$size/apps/geminidesktop-go.png"
    done

    # Atualiza caches do desktop
    update-desktop-database "$APPS_DIR" 2>/dev/null || true
    gtk-update-icon-cache -f -t "$ICONS_BASE" 2>/dev/null || true

    success "$APP_NAME foi completamente removido do sistema."
    exit 0
fi

# ==============================================================================
# MODO INSTALAÇÃO / ATUALIZAÇÃO
# ==============================================================================

echo -e "${CYAN}${BOLD}=== Instalador / Atualizador do $APP_NAME (Linux) ===${NC}"

# 1. Encerra processo em execução para evitar 'Text file busy' ao substituir binário
if pgrep -x "$APP_ID" >/dev/null 2>&1; then
    info "Uma instância do $APP_NAME está em execução. Encerrando para atualização..."
    pkill -TERM -x "$APP_ID" || true
    for i in {1..5}; do
        if ! pgrep -x "$APP_ID" >/dev/null 2>&1; then
            break
        fi
        sleep 1
    done
    if pgrep -x "$APP_ID" >/dev/null 2>&1; then
        warn "Processo não encerrou graciosamente. Forçando encerramento..."
        pkill -9 -x "$APP_ID" || true
        sleep 1
    fi
fi

# 2. Localização do binário fonte
BIN_SOURCE=""
TEMP_DIR=""

cleanup() {
    if [ -n "$TEMP_DIR" ] && [ -d "$TEMP_DIR" ]; then
        rm -rf "$TEMP_DIR"
    fi
}
trap cleanup EXIT

if [ "$DO_DOWNLOAD" -eq 1 ]; then
    info "Baixando última versão estável do GitHub ($REPO)..."
    TEMP_DIR="$(mktemp -d)"
    if command -v gh >/dev/null 2>&1; then
        gh release download --repo "$REPO" --pattern "geminidesktop-go" --dir "$TEMP_DIR"
    else
        LATEST_URL="https://github.com/$REPO/releases/latest/download/geminidesktop-go"
        curl -sSL "$LATEST_URL" -o "$TEMP_DIR/geminidesktop-go"
    fi
    BIN_SOURCE="$TEMP_DIR/geminidesktop-go"
elif [ -f "$SCRIPT_DIR/build/bin/geminidesktop-go" ]; then
    BIN_SOURCE="$SCRIPT_DIR/build/bin/geminidesktop-go"
elif [ -f "$SCRIPT_DIR/geminidesktop-go" ]; then
    BIN_SOURCE="$SCRIPT_DIR/geminidesktop-go"
else
    info "Binário compilado local não encontrado. Baixando do GitHub ($REPO)..."
    TEMP_DIR="$(mktemp -d)"
    if command -v gh >/dev/null 2>&1; then
        gh release download --repo "$REPO" --pattern "geminidesktop-go" --dir "$TEMP_DIR"
    else
        LATEST_URL="https://github.com/$REPO/releases/latest/download/geminidesktop-go"
        curl -sSL "$LATEST_URL" -o "$TEMP_DIR/geminidesktop-go"
    fi
    BIN_SOURCE="$TEMP_DIR/geminidesktop-go"
fi

if [ ! -f "$BIN_SOURCE" ]; then
    error "Não foi possível encontrar nem baixar o binário geminidesktop-go."
    exit 1
fi

chmod +x "$BIN_SOURCE"

# 3. Cria diretórios de destino
mkdir -p "$INSTALL_DIR" "$BIN_DIR" "$APPS_DIR" "$ICONS_BASE"

# 4. Copia o binário atomicamente
info "Instalando executável em $INSTALL_DIR/geminidesktop-go..."
cp "$BIN_SOURCE" "$INSTALL_DIR/geminidesktop-go.tmp"
chmod 755 "$INSTALL_DIR/geminidesktop-go.tmp"
mv -f "$INSTALL_DIR/geminidesktop-go.tmp" "$BIN_TARGET"

# 5. Cria links simbólicos no PATH do usuário / sistema
ln -sf "$BIN_TARGET" "$BIN_DIR/geminidesktop-go"
ln -sf "$BIN_TARGET" "$BIN_DIR/gemini-desktop"
success "Links simbólicos criados em $BIN_DIR/geminidesktop-go e gemini-desktop"

# 6. Instalação dos ícones do sistema
info "Instalando ícones no tema do sistema..."
if [ -d "$SCRIPT_DIR/build/linux/icons" ]; then
    for dir in "$SCRIPT_DIR/build/linux/icons"/*x*/; do
        if [ -d "$dir" ]; then
            size="$(basename "$dir")"
            mkdir -p "$ICONS_BASE/$size/apps"
            if [ -f "$dir/apps/$APP_ID.png" ]; then
                cp "$dir/apps/$APP_ID.png" "$ICONS_BASE/$size/apps/$APP_ID.png"
            fi
        fi
    done
fi

# Copia appicon de alta resolução (512x512)
if [ -f "$SCRIPT_DIR/build/appicon.png" ]; then
    mkdir -p "$ICONS_BASE/512x512/apps"
    cp "$SCRIPT_DIR/build/appicon.png" "$ICONS_BASE/512x512/apps/$APP_ID.png"
    cp "$SCRIPT_DIR/build/appicon.png" "$INSTALL_DIR/appicon.png"
fi

# 7. Criação / Atualização do arquivo .desktop
info "Configurando entrada no menu de aplicações..."
cat << EOF > "$DESKTOP_TARGET"
[Desktop Entry]
Version=1.0
Type=Application
Name=Gemini Desktop
GenericName=Cliente ACP para agentes Gemini/Antigravity
Comment=Cliente desktop nativo para agentes ACP e automação de navegador (Browser Bridge)
Exec=$BIN_DIR/geminidesktop-go %U
Icon=$APP_ID
Terminal=false
StartupNotify=true
StartupWMClass=geminidesktop-go
Categories=Development;Network;Utility;
Keywords=gemini;antigravity;acp;agent;chat;browser-bridge;
MimeType=x-scheme-handler/geminidesktop;
EOF

chmod 644 "$DESKTOP_TARGET"
success "Atalho criado em $DESKTOP_TARGET"

# 8. Atualização dos caches do sistema
info "Atualizando caches do ambiente desktop..."
update-desktop-database "$APPS_DIR" 2>/dev/null || true
gtk-update-icon-cache -f -t "$ICONS_BASE" 2>/dev/null || true
xdg-desktop-menu forceupdate 2>/dev/null || true

# 9. Verificação do PATH
echo ""
success "${BOLD}$APP_NAME instalado/atualizado com sucesso!${NC}"
echo -e "  - Executável: ${CYAN}$BIN_TARGET${NC}"
echo -e "  - Atalho:     ${CYAN}$DESKTOP_TARGET${NC}"
echo -e "  - Menu:       Disponível no menu de aplicações (Iniciar) como '${BOLD}$APP_NAME${NC}'"

if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
    echo ""
    warn "O diretório $BIN_DIR não parece estar no seu PATH atual."
    echo -e "  Para executar 'geminidesktop-go' direto do terminal, adicione ao seu ~/.bashrc ou ~/.zshrc:"
    echo -e "  ${BOLD}export PATH=\"\$PATH:$BIN_DIR\"${NC}"
fi

echo ""
echo -e "Para iniciar o app agora, execute: ${BOLD}geminidesktop-go${NC} ou abra pelo menu Iniciar."
