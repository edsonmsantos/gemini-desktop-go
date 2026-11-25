# GeminiDesktop-Go

Um cliente desktop nativo (Go + React/Wails) no estilo do Gemini, que conversa com agentes de CLI oficiais — **Antigravity CLI** e **Gemini CLI** — via **ACP (Agent Client Protocol)**: JSON-RPC 2.0 delimitado por newline sobre stdio. Sem chave de API, sem scraping, sem telemetria.

## ✨ Destaques

- 🖥️ **Janela nativa** (Wails v2 + WebKitGTK) — binário único de ~15 MB, nunca abre navegador
- 🤖 **Multi-agente ACP**: `agy acp` (default, usa sua conta Google autenticada) e `gemini --acp`
- 💬 Chat com **streaming**, *thinking* colapsável, tool calls coloridos, planos e **Permitir/Negar**
- 📎 **Anexos multimodais**: cole arquivos do gerenciador ou use o seletor nativo — imagens vão como base64, demais arquivos por caminho
- 🗂️ **Workspaces**: pastas recentes, escolha manual, seletor nativo ou **pasta temporária automática** por conversa
- 💾 **Histórico persistente** (SQLite pure-Go) com busca, renomear, retomar sessões (`session/load`) e troca de modelo
- 🌗 **Temas dark/light** fiéis ao Gemini, avatar condicional com plano da conta (ex.: "Google AI Pro")
- 🔒 Bind exclusivo em `127.0.0.1`, zero telemetria, zero binários de terceiros

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
  "port": 8399
}
```

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

### 3. Executando em Modo Servidor Web Local (Fallback / Debug)
```bash
go run . serve
```
Por padrão, a interface estará disponível em:
**http://127.0.0.1:8399**

Você também pode especificar flags personalizadas:
```bash
go run . serve --port 9000 --cwd /caminho/do/projeto
```

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
