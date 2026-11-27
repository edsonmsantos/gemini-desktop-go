# GeminiDesktop-Go

Um cliente desktop nativo (Go + React/Wails) no estilo do Gemini, que conversa com agentes de CLI oficiais — **Antigravity CLI** e **Gemini CLI** — via **ACP (Agent Client Protocol)**: JSON-RPC 2.0 delimitado por newline sobre stdio. Sem chave de API, sem scraping, sem telemetria.

## ✨ Destaques

- 🖥️ **Janela nativa** (Wails v2 + WebKitGTK) — binário único de ~15 MB, nunca abre navegador
- 🤖 **Multi-agente ACP**: `agy acp` (default, usa sua conta Google autenticada) e `gemini --acp`
- 💬 Chat com **streaming**, *thinking* colapsável, tool calls coloridos, planos e **Permitir/Negar**
- 📎 **Anexos multimodais**: cole arquivos do gerenciador, **cole screenshots com Ctrl+V**, **arraste arquivos** para a conversa ou use o seletor nativo — imagens vão como base64, demais arquivos por caminho
- ✏️ **Editar e Regenerar**: corrija a última mensagem ou gere outra resposta com um clique
- 📋 **Copiar, 📄 Salvar como PDF e 🔗 Fontes**: ações em cada resposta — PDF gerado no backend cai em `~/Downloads/Gemini Desktop/`; fontes (links citados) abrem no navegador padrão com um clique
- 📁 **Artefatos**: pill no topo do chat lista arquivos criados/modificados pelo agente na conversa (varredura do workspace por data de modificação); clique abre no app padrão do sistema
- 🔔 **Notificações nativas** quando o agente termina a resposta (`notify-send`; desabilite com `"notifications": false` no config.json)
- 🩹 **Erros em linguagem humana** com botão "Tentar novamente" — sem mensagens técnicas
- 🗂️ **Workspaces**: pastas recentes, escolha manual, seletor nativo ou **pasta temporária automática** por conversa
- 💾 **Histórico persistente** (SQLite pure-Go) com busca, renomear, retomar sessões (`session/load`) e troca de modelo
- 🌗 **Temas dark/light** fiéis ao Gemini, avatar condicional com plano da conta (ex.: "Google AI Pro")
- 🔒 Bind exclusivo em `127.0.0.1`, zero telemetria, zero binários de terceiros

## 📦 Instalação & Atualização (Linux & Windows)

### 🐧 Linux (Instalador e Atualizador Automático)

O script `install.sh` gerencia tanto a **primeira instalação** quanto as **atualizações futuras** de forma atômica e segura (finaliza instâncias antigas em execução, preserva dados e atualiza caches de ícones e desktop):

#### 1. Instalação Rápida (via terminal)
```bash
curl -fsSL https://raw.githubusercontent.com/edsonmsantos/gemini-desktop-go/main/install.sh | bash -s -- --download
```

#### 2. Instalação a partir do Repositório ou Clone Local
```bash
git clone https://github.com/edsonmsantos/gemini-desktop-go.git
cd gemini-desktop-go
chmod +x install.sh
./install.sh
```
> [!TIP]
> **Como Atualizar:** Basta executar `./install.sh` (ou repetir o comando `curl`) a qualquer momento. O script detecta a instalação existente, finaliza o processo em execução com segurança, substitui o binário de forma atômica e atualiza os atalhos.

#### Opções do `install.sh`:
| Comando | Descrição |
|---|---|
| `./install.sh` | Instala ou atualiza no escopo do usuário atual (`~/.local/share/geminidesktop`, binário em `~/.local/bin`) |
| `sudo ./install.sh` | Instalação global para todos os usuários do sistema (`/opt/geminidesktop`, binário em `/usr/local/bin`) |
| `./install.sh --download` | Força o download da versão compilada mais recente diretamente das Releases do GitHub |
| `./install.sh --uninstall` | Remove completamente o executável, atalhos `.desktop`, links simbólicos e ícones do sistema |

#### Estrutura de Diretórios Criada no Linux:
- **Binário do App:** `~/.local/share/geminidesktop/geminidesktop-go`
- **Links Simbólicos no PATH:** `~/.local/bin/geminidesktop-go` e `~/.local/bin/gemini-desktop`
- **Atalho no Menu Iniciar:** `~/.local/share/applications/geminidesktop-go.desktop`
- **Ícones do Sistema:** `~/.local/share/icons/hicolor/{16x16..512x512}/apps/geminidesktop-go.png`

---

### 🪟 Windows

- Baixe o executável pré-compilado na [Página de Releases do GitHub](https://github.com/edsonmsantos/gemini-desktop-go/releases/latest):
  - **`geminidesktop-go.exe`** (Executável nativo PE32+ para arquitetura x86-64, requer WebView2 runtime do Windows 10/11).
- Execute diretamente ou crie um atalho na Área de Trabalho/Menu Iniciar.
*(O instalador dedicado para Windows via InnoSetup está em desenvolvimento para as próximas versões).*

---

## 🤖 Guia de Instalação e Operação para Agentes LLM

Se você é um **agente de IA autônomo** (Antigravity CLI, Claude Code, Cursor, OpenCode, AutoGPT ou orquestrador MCP/ACP) encarregado de instalar, configurar ou operar o GeminiDesktop-Go, siga estas diretrizes:

### 1. Pré-requisitos do Host
Verifique e assegure os pré-requisitos antes da execução:
- **Biblioteca WebKitGTK (Linux com interface gráfica):**
  - Debian/Ubuntu: `sudo apt-get install -y libwebkit2gtk-4.1-dev` (ou `libwebkit2gtk-4.1-0` para apenas executar)
  - Fedora: `sudo dnf install webkit2gtk4.1`
  - Arch: `sudo pacman -S webkit2gtk-4.1`
- **Agente ACP Subjacente (Obrigatório):**
  - O GeminiDesktop atua como cliente ACP e precisa de um executável ACP no `PATH`.
  - Preferencial: **Antigravity CLI (`agy`)** — verifique se `which agy` retorna um caminho válido e se o usuário está logado (`agy` responde a comandos).
  - Alternativa: **Gemini CLI (`gemini`)** — via `npm install -g @google/gemini-cli`.

### 2. Instalação Automatizada (Não-Interativa)
O script de instalação é 100% não-interativo e seguro para automações de agentes:
```bash
# Download e instalação direta da última release estável:
curl -fsSL https://raw.githubusercontent.com/edsonmsantos/gemini-desktop-go/main/install.sh | bash -s -- --download
```
Certifique-se de que `~/.local/bin` está no `PATH` da sessão do agente.

### 3. Modo Headless / API Server (`serve`)
Quando operando em servidores remotos, containers, ambientes sem servidor gráfico (X11/Wayland) ou quando você quiser orquestrar conversas via REST/WebSocket:

```bash
geminidesktop-go serve --port 8399
```

> [!IMPORTANT]
> **Fluxo de Autenticação para Agentes:**
> - Ao inicializar em modo `serve`, o servidor gera um token criptográfico efêmero e emite no log:
>   `[AUTH] Token de sessão gerado: <TOKEN_HEX>`
> - O agente deve ler este token da saída padrão do processo e utilizá-lo em todas as requisições:
>   - **Cabeçalho HTTP:** `X-Gemini-Token: <TOKEN_HEX>` ou `Authorization: Bearer <TOKEN_HEX>`
>   - **WebSocket:** `ws://127.0.0.1:8399/api/stream?token=<TOKEN_HEX>`
> - Endpoints públicos (não exigem token): `GET /` e `GET /api/health` (ideais para healthcheck de prontidão).

### 4. Orquestração Programática de Sessões ACP
Para interagir com o GeminiDesktop via API:
1. **Criar Sessão com Workspace Específico:**
   ```bash
   curl -X POST http://127.0.0.1:8399/api/conversations \
     -H "X-Gemini-Token: $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"cwd": "/caminho/do/projeto"}'
   ```
2. **Enviar Prompt ao Agente:**
   ```bash
   curl -X POST http://127.0.0.1:8399/api/conversations/$CONV_ID/prompt \
     -H "X-Gemini-Token: $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"text": "Escreva um teste unitário para auth.go"}'
   ```
3. **Consumir Eventos em Tempo Real:** Conecte um cliente WebSocket em `ws://127.0.0.1:8399/api/stream?token=$TOKEN` para receber chunks de resposta, pensamentos (*thinking*), notificações de planos e chamadas de ferramentas (*tool calls*).
4. **Responder a Pedidos de Permissão:** Caso uma tool call exija confirmação (`permission`), envie:
   ```bash
   curl -X POST http://127.0.0.1:8399/api/permission \
     -H "X-Gemini-Token: $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"id": "$TOOL_CALL_ID", "decision": "allow"}'
   ```

### 5. Configuração Persistente (`~/.geminidesktop/config.json`)
Agentes podem pré-configurar o ambiente antes de subir o aplicativo gravando em `~/.geminidesktop/config.json`:
```json
{
  "agent": {
    "command": "agy",
    "args": ["acp"]
  },
  "defaultCwd": "/caminho/do/projeto",
  "browserBridgeInProcess": true,
  "browserBridgePort": 9223,
  "port": 8399,
  "turnTimeoutSec": 900
}
```

### 6. Localização dos Dados e Limpeza
- **Banco SQLite:** `~/.geminidesktop/geminidesktop.db` (WAL mode habilitado).
- **Screenshots e Evidências:** `~/.geminidesktop/screenshots/<conversationId>/`.
- **Anexos Enviados:** `~/.geminidesktop/attachments/`.
- **Encerramento Limpo:** Envie `SIGTERM` ao PID do processo para garantir que as transações SQLite e o processo filho do ACP sejam fechados graciosamente.

---

## Agentes Suportados

O GeminiDesktop-Go suporta múltiplos agentes ACP com autodetecção inteligente de comandos e argumentos:

1. **Antigravity CLI (`agy acp`)** *(Recomendado / Default)*:
   - Utiliza `agy acp` com handshake protocolVersion v1.
   - Suporte a modelos Gemini 2.5 Flash, thinking tokens, e sessões autenticadas via conta Google.

2. **Gemini CLI (`gemini --acp`)**:
   - Utiliza `@google/gemini-cli` com a flag oficial `--acp` (e fallback gracioso para `--experimental-acp` caso necessário).

### Alternando Agentes via `~/.geminidesktop/config.json`

O arquivo de configuração suporta o novo formato estruturado `agent`:

```json
{
  "agent": {
    "command": "agy",
    "args": ["acp"]
  },
  "defaultCwd": "/home/seu-usuario/projetos",
  "tempWorkspacesDir": "/tmp/geminidesktop",
  "port": 8399,
  "turnTimeoutSec": 900
}
```

- `turnTimeoutSec`: timeout de turno em segundos (padrão 900, 0 = sem limite).

Para usar o Gemini CLI oficial:
```json
{
  "agent": {
    "command": "gemini",
    "args": ["--acp"]
  },
  "port": 8399
}
```

*(O formato anterior com o campo `"cliPath"` é 100% retrocompatível e migrado automaticamente)*.

---

## Arquitetura Backend v2

```text
               ┌────────────────────────────────────────────────────────┐
               │                  GeminiDesktop HTTP/WS                 │
               │                   (127.0.0.1:8399)                     │
               └──────────┬─────────────────────────────┬───────────────┘
                          │                             │
                          ▼                             ▼
              ┌──────────────────────┐      ┌───────────────────────┐
              │  REST API & WS Hub   │      │   internal/store      │
              │   internal/server    │◄────►│  (SQLite pure-Go      │
              │                      │      │   modernc.org/sqlite) │
              └──────────┬───────────┘      └───────────┬───────────┘
                         │                              │
                         ▼                              ▼
              ┌──────────────────────┐      ┌───────────────────────┐
              │   internal/acp       │      │ ~/.geminidesktop/     │
              │   (Client & Sessions)│      │  geminidesktop.db     │
              └──────────┬───────────┘      └───────────────────────┘
                         │
                         ▼ (JSON-RPC 2.0 stdio \n)
              ┌──────────────────────┐
              │  agy acp / gemini    │
              │  (Agent Subprocess)  │
              └──────────────────────┘
```

1. **`internal/store` (Persistência SQLite pure-Go)**:
   - Driver `modernc.org/sqlite` sem dependência de CGO.
   - Banco persistente em `~/.geminidesktop/geminidesktop.db`.
   - Modos `WAL` e `foreign_keys = ON` habilitados para concorrência de leitura e integridade.
   - Tabelas:
     - `conversations`: ID (UUID), título (gerado automaticamente dos ~60 chars iniciais do 1º prompt), agent, model_id, cwd, acp_session_id, timestamps.
     - `messages`: histórico sequencial indexado por conversa (`seq`), roles (`user`, `assistant`, `thought`, `tool_call`, `plan`, `permission`, `error`), concatenação atômica de chunks de texto/thought, e metadados JSON para tool calls, permissões e status.
     - `workspaces`: diretórios de trabalho (`cwd`) com `last_used_at` atualizados a cada sessão.
     - `schema_version`: controle simples de migrações automáticas.

2. **`internal/acp` (Cliente ACP Multissessão)**:
   - Gerenciamento de múltiplos IDs de sessão sobre o mesmo processo do agente (`agy` ou `gemini`).
   - Roteamento de eventos assíncronos (`session/update`) isolado por canal de sessão.
   - Retomada de sessões existentes (`session/load`) com tratamento do erro tipado `ErrResumeNotSupported`.
   - Seleção dinâmica de modelo (`session/set_model`) com tratamento do erro tipado `ErrModelNotSupported`.
   - Catálogo de modelos suportados (`Models()`) coletado via `initialize` e `session/new`.

3. **`internal/config`**:
   - Carrega ou inicializa `~/.geminidesktop/config.json` com campos `{agent: {command, args}, defaultCwd, port}`. Nenhuma telemetria.

4. **`internal/server` (API REST & WebSocket)**:
   - Servidor HTTP/WebSocket restrito a `127.0.0.1`.
   - **Tabela de Endpoints v2**:
     | Método | Rota | Descrição |
     |---|---|---|
     | `GET` | `/api/health` | Status de saúde do serviço |
     | `GET` | `/api/agent-info` | Retorna email ativo da conta Google (`~/.gemini/google_accounts.json`) ou `null` |
     | `GET` | `/api/conversations` | Lista de conversas persistidas ordenadas por `updated_at DESC` |
     | `POST` | `/api/conversations` | Cria nova conversa e sessão ACP (`cwd` opcional; se vazio, cria dir temporário `/tmp/geminidesktop/<uuid>`) |
     | `GET` | `/api/conversations/{id}` | Retorna detalhes da conversa e histórico de mensagens sequenciais |
     | `PATCH` | `/api/conversations/{id}` | Atualiza título ou altera modelo da conversa via `session/set_model` |
     | `DELETE` | `/api/conversations/{id}` | Remove a conversa e seu histórico do banco |
     | `POST` | `/api/conversations/{id}/prompt` | Envia prompt do usuário com suporte a anexos multimodais (`text` + `files: []string`) |
     | `POST` | `/api/conversations/{id}/regenerate` | Descarta o último turno e reenvia o último prompt (`text` opcional para edição) |
     | `POST` | `/api/attachments` | Salva anexo enviado em base64 (print colado/drag & drop) e retorna o caminho |
     | `POST` | `/api/export/pdf` | Exporta uma resposta markdown como PDF em `~/Downloads/Gemini Desktop/` |
     | `POST` | `/api/open-url` | Abre URL externa exclusivamente http/https no navegador padrão |
     | `POST` | `/api/artifacts/open` | Abre arquivo de artefato ou anexo no app padrão (com validação estrita de allowlist e symlinks) |
     | `POST` | `/api/files/content` | Retorna o conteúdo de arquivo permitido dentro da allowlist do workspace |
     | `GET` | `/api/conversations/{id}/artifacts` | Lista arquivos criados/modificados no workspace desde a criação da conversa |
     | `POST` | `/api/conversations/{id}/resume` | Retoma sessão anterior via `session/load` do ACP |
     | `GET` | `/api/models` | Lista modelos disponíveis anunciados pelo agente ACP |
     | `GET` | `/api/workspaces` | Lista diretórios de trabalho recentes registrados |
     | `POST` | `/api/permission` | Decisão do usuário (Permitir/Negar) sobre ações do agente |
     | `POST` | `/api/cancel` | Cancelamento do turno em execução |
     | `GET` | `/api/stream` | Stream WebSocket com frames enriquecidos com `conversationId` |
     | `GET` | `/api/skills` | Lista skills dos escopos global (`~/.agents/skills`) e workspace (`<cwd>/.agents/skills`) com deduplicação |
     | `GET` | `/api/mcp` | Lista servidores MCP registrados via CLI `agy mcp list` com status de disponibilidade |
     | `POST` | `/api/mcp/{name}/toggle` | Habilita ou desabilita um servidor MCP via CLI `agy mcp enable/disable` |
     | `GET` | `/api/search` | Busca full-text nas conversas e mensagens com snippets de contexto e limite configurável |
     | `GET` | `/api/browser/status` | Retorna o status de conexão da ponte do navegador (daemon Go ou Node na porta 9223) |
     | `GET` | `/api/browser/approvals` | Lista pedidos de aprovação pendentes da ponte do navegador (modo ask) |
     | `POST` | `/api/browser/approvals/{id}` | Resolve uma aprovação pendente (`approve: bool`) na ponte do navegador |
     | `GET` | `/api/browser/wizard/state` | Estado do wizard do navegador (agentFound, browsers detectados/políticas, extensionConnected, updateBaseURL) |
     | `POST` | `/api/browser/install-policy` | Instala política de extensão automaticamente no navegador informado |
     | `POST` | `/api/browser/remove-policy` | Remove política de extensão no navegador informado |

### Navegador integrado (browser-bridge in-process)

O GeminiDesktop fecha o produto único integrando nativamente o daemon do **browser-bridge em-processo**, eliminando a necessidade de iniciar um daemon separado manualmente:
- **Eleição de Porta Automática (9223)**: na inicialização do app, o GeminiDesktop tenta assumir a porta 9223 como primário in-process. Se a porta estiver livre, ele hospeda a ponte diretamente. Se a porta já estiver ocupada (por exemplo, por um daemon CLI externo `browser-bridge serve`), ele coexiste graciosamente como cliente REST sem conflitos.
- **Wizard de Primeiro Executo (Browser Setup Wizard)**: integrado ao OnboardingModal no primeiro executo, detecta os navegadores instalados no sistema operacional (Brave, Google Chrome, Microsoft Edge, Chromium), verifica a existência de políticas `ExtensionSettings` e permite a instalação da extensão com apenas 1 clique (via `policymgr`). Em seguida, acompanha o reinício do navegador com polling automático até a conexão ser estabelecida, dispondo de fallback guiado documentado (`Load unpacked` na pasta `~/.browser-bridge/extension` ou comando CLI `browser-bridge install browsers`).
- **Dependência Externa Explícita (agy CLI)**: o assistente detecta a presença do binário `agy` (ou `gemini`) no sistema. Se ausente, exibe um cartão orientando a instalação do Antigravity CLI e login com a conta Google via [https://antigravity.google](https://antigravity.google), sem bloquear o fluxo de configuração do navegador.
- **Configurações**: em `~/.geminidesktop/config.json`:
  - `browserBridgeInProcess`: booleano (padrão `true`). Permite desativar a hospedagem do daemon em-processo caso o usuário deseje delegar exclusivamente a processos externos.
  - `browserBridgeUpdateBaseURL`: string opcional com a URL base para manifestos e pacotes CRX de atualização da extensão (padrão local: `http://127.0.0.1:9223/update`).
  - `browserBridgePort`: porta TCP da ponte (padrão `9223`).
  - `browserBridgeTokenPath`: caminho do token de autenticação (padrão `~/.browser-bridge/token.txt`).

- **Status Pill em Tempo Real**: pill visual no rodapé indica o status de prontidão da ponte e da extensão do navegador.
- **BrowserToolCard**: chamadas MCP para `browser-bridge` recebem visualização humanizada com títulos em português (`Ler página`, `Capturar tela`, `Abrir URL`), badges de contexto e segurança (`trusted`, `modal`, `falhou`), além de argumentos e saídas colapsáveis.
- **Screenshots Inline**: ao concluir uma captura de tela (`browser_screenshot` ou `browser_som_screenshot`), a imagem mais recente do cache é copiada para `~/.geminidesktop/screenshots/<conversationId>/` e renderizada inline no card com suporte a clique para ampliar.
- **Aprovações Nativas (Modo Ask)**: se a ponte opera em modo ask (daemon Go expondo `/api/approvals`), o aplicativo faz polling e exibe um modal nativo para Permitir ou Negar a ação; se o daemon for Node (sem endpoint de aprovações), a funcionalidade permanece ocultada de forma transparente.

5. **`frontend/` (React + Vite + TypeScript)**:
   - Interface desktop fiel ao Gemini (tema dual dark `#131314` + light, glow radial azul, tipografia Google Sans).
   - Modal de Nova Conversa com suporte a seleção de workspaces recentes, caminho manual, navegação nativa no sistema de arquivos e modo temporário (sem workspace persistente).
   - Anexos no Prompt: botão `+` abrindo seletor nativo do SO (`SelectFiles`), suporte a colar arquivos via clipboard (`file://` URIs) e renderização de chips com preview e remoção.
   - Envio multimodal: imagens enviadas diretamente como blocos base64 para o agente ACP e demais arquivos como referências absolutas.
   - Sidebar colapsável de 280px com histórico real, workspaces e rodapé condicional com avatar de iniciais estilo Google e engrenagem.
   - Renderização Markdown com destaque e botão de cópia de código monoespaçado.
   - Suporte completo a blocos de thinking, tool calls com status em tempo real, cards de permissão e checklist de planos.
   - Zero telemetria e ícones em SVG inline minimalistas.

6. **Janela Nativa Desktop (Wails v2)**:
   - Integrado via Wails v2 + WebKitGTK-4.1 (`webkit2_41`).
   - Produz binário desktop nativo em `build/bin/geminidesktop-go`.

---

## Como Rodar

### Pré-requisitos
- Go 1.22+ instalado (testado com Go 1.26.4).
- Node 20+ e npm.
- Wails CLI v2.11+ e `webkit2gtk-4.1`.
- `agy` (`~/.local/bin/agy`) ou `gemini` CLI instalado.

### 1. Compilando e Executando a Janela Nativa Desktop (Wails)
Para compilar o binário desktop nativo (o Wails executa `npm install` e `npm run build` automaticamente via hooks):
```bash
wails build
```
O executável nativo é gerado em:
```bash
./build/bin/geminidesktop-go
```

Para executar em modo desenvolvimento com hot-reload:
```bash
wails dev
```

### 2. Desenvolvimento do Frontend React Isolado
```bash
cd frontend
npm install
npm run dev
```

### 3. Executando em Modo Servidor API Headless (`serve`)
```bash
go run . serve
```
O modo `serve` executa o backend como uma **API JSON headless** (sem interface web legada, eliminando superfícies de ataque como XSS armazenado).

Por padrão, a API escuta em:
**http://127.0.0.1:8399**

> [!IMPORTANT]
> **Autenticação Obrigatória:** Ao iniciar, o servidor gera automaticamente um token efêmero de sessão e o exibe no console (`X-Gemini-Token: <token>`).
> Todas as chamadas para a API (GET, POST, DELETE e WebSocket `/api/stream?token=<token>`) exigem o envio do token no cabeçalho `X-Gemini-Token` ou `Authorization: Bearer <token>`.

Você também pode especificar flags personalizadas:
```bash
go run . serve --port 9000 --cwd /caminho/do/projeto
```

### 4. Perfil de Ferramentas Browser Bridge (`BROWSER_BRIDGE_TOOL_PROFILE`)
O MCP Server do `browser-bridge` suporta perfis de ferramentas via variável de ambiente:
- `full` (padrão): Expõe todas as 31 ferramentas (14 v2 + 17 legadas), garantindo 100% de compatibilidade com as skills instaladas (`anac-rab`, `bnmp-cnj`, `detetive-forense`, `esaj-tjms`, `sigo-pcms`).
- `compact`: Expõe exclusivamente as 14 ferramentas v2 modernas (`browser_act`, `snapshot`, `extract`, `eval`, etc.), reduzindo em ~45% a carga de tokens de contexto para fluxos modernos.

---

## Como Testar

### 1. Testes Unitários e com Agente Falso
```bash
go test -v ./...
```

### 2. Testes de Integração Real com Agentes do Host
- Handshake real com Antigravity CLI (`agy acp`):
  ```bash
  go test -v -run TestIntegration_AntigravityHandshake .
  ```
- Handshake real com Gemini CLI (`gemini --acp`):
  ```bash
  go test -v -run TestIntegration_RealGeminiHandshake .
  ```

### 3. Testes E2E Completos com Prompt Real (Gated)
- E2E com Antigravity CLI:
  ```bash
  AGY_E2E=1 go test -v -run TestE2E_AntigravityPrompt .
  ```
- E2E com Gemini CLI:
  ```bash
  GEMINI_E2E=1 go test -v -run TestE2E_RealPrompt .
  ```

### 4. Validação de Formatação e Vet
```bash
gofmt -l .
go vet ./...
```

---

## Roadmap

- [x] Fase 3A: capacidades do agy (`modes`, `configOptions`, plano da conta) expostas na UI
- [x] Fase 3B: paleta de **skills** (`/` e `$`) lendo `~/.agents/skills` e `.agents/skills` do workspace
- [x] Fase 3C: gestão de **servidores MCP** (status, enable/disable via `agy mcp`, `mcpServers` por conversa)
- [x] Fase 3D: polish inspirado em ZCode — permission dialog com preview (diff/terminal), renderers de tool call, TodoPanel minimalista, Command Center `Ctrl+K` com busca full-text
- [ ] Próximos: voz, painel de uso de contexto, Gen UI (HTML do agente em sandbox), terminal embutido

## Créditos & Tecnologia

- [ACP — Agent Client Protocol](https://agentclientprotocol.com) (Zed) — protocolo aberto de comunicação com agentes
- [Antigravity CLI](https://antigravity.google) e [Gemini CLI](https://github.com/google-gemini/gemini-cli) — agentes oficiais do Google
- [Wails v2](https://wails.io) — desktop Go com webview nativo
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — SQLite pure-Go (sem CGO)
- React 19 · Vite · Arco de ícones SVG inline

## Licença

MIT — veja [LICENSE](LICENSE).
