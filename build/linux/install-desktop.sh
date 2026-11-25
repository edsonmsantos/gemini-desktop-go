#!/usr/bin/env bash
# Instala entrada de desktop + ícones temáticos do GeminiDesktop-Go no usuário atual.
# Garante ícone nítido no dock (plank/docky usam .desktop + tema de ícones via WM_CLASS,
# em vez do _NET_WM_ICON de baixa resolução).
set -euo pipefail

APP="geminidesktop-go"
SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ICONS_SRC="$SRC_DIR/icons"

APPS_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
ICONS_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor"

mkdir -p "$APPS_DIR" "$ICONS_DIR"

# 1. Arquivo .desktop
cp "$SRC_DIR/$APP.desktop" "$APPS_DIR/"

# 2. Ícones temáticos em todos os tamanhos
for dir in "$ICONS_SRC"/*x*/; do
    size="$(basename "$dir")"
    mkdir -p "$ICONS_DIR/$size/apps"
    cp "$dir/apps/$APP.png" "$ICONS_DIR/$size/apps/"
done

# 3. Atualiza caches
update-desktop-database "$APPS_DIR" 2>/dev/null || true
gtk-update-icon-cache -f -t "$ICONS_DIR" 2>/dev/null || true

echo "Instalado:"
echo "  $APPS_DIR/$APP.desktop"
echo "  $ICONS_DIR/{16..256}x*/apps/$APP.png"
echo "Reinicie o plank para limpar o cache de ícones: killall plank && plank &"
