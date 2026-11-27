# Relatório de Avaliação Técnica e Diagnóstico de Riscos — GeminiDesktop-Go

**Data**: 26 de Novembro de 2026  
**Alvo da Análise**: Repositório `geminidesktop-go` (v0.3.0) & Ecossistema Integrado (`browser-bridge` v0.4.1 / `agy acp`)  
**Propósito**: Avaliação de conformidade arquitetural, segurança, robustez operacional, portabilidade multiplataforma (Linux/Windows) e eficiência de recursos para discussão com modelo revisor externo.

---

## 1. Sumário Executivo

O **GeminiDesktop-Go** é uma solução bem estruturada e inovadora: implementa um cliente desktop nativo leve em Go (Wails v2 + React 19) que consome agentes de IA oficiais (`agy acp` e `gemini --acp`) através do protocolo aberto **ACP (Agent Client Protocol)** via JSON-RPC 2.0 sobre `stdio`, dispensando chaves de API proprietárias ou web scraping. A integração in-process com o **browser-bridge** (porta 9223) e o tratamento humanizado de ações do navegador representam uma evolução ergonômica significativa.

Contudo, a auditoria minuciosa do código-fonte revelou **8 vulnerabilidades e gargalos técnicos substanciais**, divididos entre falhas críticas de segurança na API local, quebras severas de portabilidade no Windows (onde chamadas ao sistema Linux foram deixadas hardcoded), vazamentos potenciais de memória/disco e latência na experiência de aprovação humana (`modo ask`).

---

## 2. Arquitetura Geral & Pontos Fortes

### Destaques Positivos do Projeto
1. **Desacoplamento de Protocolo**: A comunicação com o LLM via ACP sobre `stdio` isola o app do modelo em si, permitindo alternar entre Antigravity e Gemini CLI sem refatorar a UI.
2. **Persistência Confiável**: Uso de SQLite pure-Go (`modernc.org/sqlite`) em modo WAL com transações atômicas e foreign keys ativadas, eliminando dependências externas de CGO no Linux.
3. **Eleição Graciosa da Ponte**: O mecanismo `relay.ElectAndRun` na porta 9223 permite coexistência transparente entre modo in-process (máquina virgem) e daemon standalone existente sem conflito de portas.
4. **UX Humanizada de Ações**: Tratamento refinado de eventos de navegador no `BrowserToolCard.tsx`, exibindo títulos legíveis, detecção de badges (`trusted`, `modal`) e status em tempo real.

---

## 3. Diagnóstico Detalhado de Problemas e Vulnerabilidades

### 🔴 SEVERIDADE CRÍTICA

#### 3.1. Vulnerabilidade de CSRF e Ausência de Autenticação na API REST Local
- **Localização**: `internal/server/server.go` (linhas 384–405, `corsMiddleware`)
- **Descrição**:
  O servidor HTTP local faz bind em `127.0.0.1:8399` sem nenhum token de autenticação ou cookie de sessão. Para controle de acesso, há o middleware `corsMiddleware`:
  ```go
  origin := r.Header.Get("Origin")
  allowed := isAllowedOrigin(origin)
  if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
      if allowed {
          w.Header().Set("Access-Control-Allow-Origin", origin)
      }
      if r.Method == http.MethodOptions {
          if allowed { ... }
          w.WriteHeader(http.StatusNoContent)
          return
      }
  }
  next.ServeHTTP(w, r)
  ```
- **O Risco Real**:
  Se `allowed` for `false` (por exemplo, uma página web maliciosa aberta no navegador com `Origin: https://site-malicioso.com`), o middleware **apenas deixa de emitir os cabeçalhos CORS, mas executa normalmente a requisição** (`next.ServeHTTP(w, r)`).
  Como requisições simples de formulário ou `POST` com `no-cors` não exigem preflight `OPTIONS` pelo navegador, **qualquer site na internet visitado pelo usuário no Brave ou Chrome pode emitir comandos para a API do GeminiDesktop-Go**, disparando chamadas ao agente, instalando políticas corporativas no navegador ou acionando exportações.
- **Impacto**: Execução arbitrária de ações locais via Cross-Site Request Forgery (CSRF).
- **Correção Recomendada**:
  Se `origin != ""` e `!allowed`, rejeitar imediatamente com `http.StatusForbidden` (403). Adicionalmente, implementar um token aleatório gerado na inicialização e exigido no header `X-Gemini-Token` para todas as chamadas de API feitas pela WebView.

---

#### 3.2. Leitura Arbitrária de Arquivos do Host sem Restrição de Sandbox
- **Localização**: `internal/server/files.go` (linhas 79–130, `handleFileContent`)
- **Descrição**:
  O endpoint `POST /api/files/content` recebe um caminho de arquivo em JSON:
  ```go
  path := strings.TrimSpace(req.Path)
  if path == "" || !filepath.IsAbs(path) { ... }
  data, err := os.ReadFile(path)
  ```
  Ele apenas valida se o caminho é absoluto (`filepath.IsAbs`) e se a extensão está em um mapa de extensões de texto ou imagem.
- **O Risco Real**:
  Combinado com o fato de que `isAllowedOrigin` permite `http://localhost:*` (qualquer servidor local de desenvolvimento) e a falha de CSRF acima, qualquer script rodando localmente ou web app pode requisitar e ler arquivos arbitrários como `~/.bashrc`, `~/.config/gh/hosts.yml`, chaves SSH privadas renomeadas, arquivos de configuração do sistema ou históricos de conversa. Não há checagem de "chroot" ou confinamento ao diretório do workspace da conversa.
- **Impacto**: Vazamento de dados sensíveis e credenciais do usuário.
- **Correção Recomendada**:
  Restringir a leitura exclusivamente a caminhos contidos dentro do workspace ativo da conversa (`conv.CWD`), do diretório temporário do app (`~/.geminidesktop/`) ou pastas expressamente autorizadas.

---

#### 3.3. Risco de Crash Silencioso do Parser ACP por Limite Estático de 4MB
- **Localização**: `internal/acp/framer.go` (linhas 73–118, `readLoop`)
- **Descrição**:
  O cliente ACP lê mensagens delimitadas por quebra de linha via `bufio.Scanner`:
  ```go
  scanner := bufio.NewScanner(f.reader)
  scanner.Buffer(make([]byte, 64*1024), 4*1024*1024) // 4MB máximo
  for scanner.Scan() { ... }
  ```
- **O Risco Real**:
  Se o agente ACP (`agy` ou `gemini`) emitir uma mensagem com mais de 4MB em uma única linha (por exemplo: um dump de captura de tela em alta definição / retina em base64, payload do `browser_save` com DOM completo serializado, ou transcrição de múltiplos documentos), o `bufio.Scanner` interrompe com `bufio.ErrTooLong`.
  Pior: **o código não verifica nem loga `scanner.Err()`**. O loop simplesmente se encerra, fecha todos os canais de pending requests e o framer morre silenciosamente. A UI do chat para de responder a novos comandos e não oferece nenhuma pista ao usuário.
- **Impacto**: Parada total e silenciosa do chat após tool calls com payloads pesados.
- **Correção Recomendada**:
  Substituir `bufio.Scanner` por um leitor baseado em `bufio.Reader.ReadBytes('\n')` ou aumentar o teto com fallback dinâmico de bufferização (`bytes.Buffer`), garantindo log detalhado de falhas no framer.

---

### 🟠 SEVERIDADE ALTA

#### 3.4. Quebra de Funcionalidades no Windows (Dependências Unix Hardcoded)
Apesar do binário Windows (`gemini-desktop-go.exe`) ter sido compilado com sucesso, trechos críticos do backend utilizam utilitários exclusivos de Linux/X11:
1. **Abertura de Arquivos e URLs (`internal/server/export_pdf.go`, L308-335)**:
   - Usa `exec.CommandContext(ctx, "xdg-open", ...)` incondicionalmente.
   - Testa caminho absoluto apenas com `strings.HasPrefix(trimmed, "/")`, falhando em letras de unidade do Windows (`C:\...`).
   - *Resultado no Windows*: Clicar em "Abrir no Navegador" ou abrir arquivos de artefato gera erro `exec: "xdg-open": executable file not found in %PATH%`.
2. **Notificações do Sistema (`internal/server/server.go`, L1396-1402)**:
   - Chama `notify-send -a "Gemini Desktop" ...`.
   - *Resultado no Windows*: Falha silenciosa com log de aviso; usuário nunca recebe notificação de resposta concluída.
3. **Colar Imagem da Área de Transferência (`internal/server/server.go`, L1484-1512)**:
   - Invoca `xclip -selection clipboard -t ...`.
   - *Resultado no Windows* (e Wayland puro sem xclip): `Ctrl+V` para colar prints falha 100% das vezes.
- **Impacto**: Degradação funcional severa da versão Windows.
- **Correção Recomendada**:
  Criar helpers específicos por plataforma via `_windows.go` e `_unix.go`:
  - Abertura de URL: `rundll32 url.dll,FileProtocolHandler` ou `cmd /c start ""` no Windows.
  - Clipboard: Utilizar APIs da WebView2 ou biblioteca Go nativa multiplataforma (ex: `golang.design/x/clipboard`).

---

#### 3.5. Vazamento de Processos Filhos no Windows (`process_windows.go`)
- **Localização**: `internal/acp/process_windows.go`
- **Descrição**:
  No Linux, o app atribui o subprocesso a um Process Group (`Setpgid: true`) e finaliza a árvore com `syscall.Kill(-pid, SIGKILL)`.
  No Windows, o arquivo `process_windows.go` implementa `configureProcessGroup` e `killProcessGroup` como **no-ops**:
  ```go
  func configureProcessGroup(cmd *exec.Cmd) {
      // no-op: Windows gerencia a árvore de processos via Job Objects (não usado aqui)
  }
  func killProcessGroup(pid int) error { return nil }
  ```
- **O Risco Real**:
  O `agy` e o `gemini` são executáveis que disparam subagentes, compilers, scripts e processos de navegador. Quando o usuário fecha a janela do Gemini Desktop ou reinicia o agente, apenas o processo pai imediato é morto (`proc.Kill()`). Todos os filhos continuam rodando em segundo plano no Windows como processos zumbis, consumindo memória e travando arquivos.
- **Impacto**: Vazamento de memória e processos órfãos na máquina Windows do usuário.
- **Correção Recomendada**:
  Implementar atribuição do processo a um Windows **Job Object** com a flag `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` na criação do processo (`CREATE_SUSPENDED` -> `AssignProcessToJobObject` -> `ResumeThread`), garantindo que o encerramento do app limpe 100% da árvore de processos.

---

#### 3.6. Latência de até 15 Segundos no Diálogo de Aprovação Humana (`modo ask`)
- **Localização**: `frontend/src/App.tsx` (linhas 145–170)
- **Descrição**:
  O diálogo de aprovação de segurança (`BrowserApprovalModal.tsx`) é acionado quando a ponte intercepta uma ação sensível do navegador.
  No entanto, o estado `approvalsPending` é alimentado por um polling de 15 segundos:
  ```typescript
  const checkStatus = async () => {
    const st = await fetchBrowserStatus()
    setBrowserApprovalsPending(st.approvalsPending || 0)
  }
  const timer = setInterval(checkStatus, 15000)
  ```
  Somente após esse timer de 15s detectar pendência é que o polling rápido de 2s entra em ação.
- **O Risco Real**:
  Quando um agente precisa clicar ou preencher um campo restrito, ele bloqueia aguardando a decisão do usuário. O usuário fica olhando para a tela achando que o agente travou, pois o modal pode demorar até 15 segundos para surgir.
- **Impacto**: Experiência de uso truncada e risco de expiração do timeout de turno do agente.
- **Correção Recomendada**:
  A ponte ou o backend devem despachar um evento WebSocket imediato (`type: "browser_approval_requested"`) no canal `/api/stream`, abrindo o modal instantaneamente.

---

### 🟡 SEVERIDADE MÉDIA

#### 3.7. Sobrecarga de Contexto e Custo Desnecessário de Tokens (29 Tools MCP)
- **Descrição**:
  Todas as 29 tools do `browser-bridge` são registradas no ACP para todas as conversas indiscriminadamente.
  Conforme demonstrado na medição real do fluxo RAB:
  - Legado (sem schemas no prompt inicial): ~25k tokens.
  - Versão v2 com 29 tools: subiu para ~40k–59k tokens.
- **Análise**:
  Mesmo em tarefas simples de programação ou perguntas puramente conceituais, o agente carrega o schema completo de 29 ferramentas de navegador no contexto de atenção do modelo.
- **Correção Recomendada**:
  Implementar carregamento condicional de tools (ex: habilitar ferramentas de browser apenas se a conversa estiver com o modo "Navegador" ativo, ou carregar sob demanda via MCP tool proxying).

---

#### 3.8. Bloat de Memória RAM com Screenshots em Base64 no React
- **Localização**: `frontend/src/components/BrowserToolCard.tsx` (linhas 148–158)
- **Descrição**:
  Para exibir screenshots inline no chat, o componente chama `/api/files/content`, que converte a imagem em string base64 (`data:image/png;base64,...`) e a mantém no estado da memória do React.
- **O Risco Real**:
  Em investigações com dezenas de passos e screenshots (ex: navegação no SIGO ou e-SAJ), o DOM e a memória do WebKitGTK/WebView2 ficam sobrecarregados com centenas de megabytes de strings base64 duplicadas no heap do V8/JavaScriptCore.
- **Correção Recomendada**:
  Servir as capturas diretamente via endpoint HTTP estático (ex: `GET /api/screenshots/{convId}/{filename}`) com headers padrão de cache e renderizar `<img src="/api/screenshots/..." />`, permitindo que o motor nativo do navegador faça o streaming e descarte de texturas GPU eficientemente.

---

#### 3.9. Acúmulo de Pastas Órfãs no Disco ao Excluir Conversas
- **Localização**: `internal/server/server.go` (linhas 892–923, `handleDeleteConversation`)
- **Descrição**:
  Ao excluir uma conversa (`DELETE /api/conversations/{id}`), o registro no SQLite é deletado em cascata e a sessão ACP é fechada. Porém, o diretório de screenshots da conversa em `~/.geminidesktop/screenshots/<id>` **não é removido**.
- **Impacto**: Vazamento gradual de espaço em disco ao longo de meses de uso.
- **Correção Recomendada**:
  Adicionar `os.RemoveAll(filepath.Join(s.screenshotsBaseDir, id))` no manipulador de deleção.

---

#### 3.10. Acoplamento Exclusivo ao CLI `agy` para Comandos MCP
- **Localização**: `internal/server/server.go` (linha 149) e `internal/mcp/agy.go`
- **Descrição**:
  O cliente MCP é instanciado como `mcp.NewClient("agy")`. Se o usuário optar por rodar com o agente oficial `@google/gemini-cli`, as rotas `/api/mcp` retornam erro `agy CLI indisponível`.
- **Correção Recomendada**:
  Tornar o cliente MCP ciente do agente configurado em `config.json` ou tolerar a ausência de subcomando `mcp` com degradação graciosa na UI.

---

### 🟢 SEVERIDADE BAIXA

#### 3.11. Ausência de Ping/Pong no WebSocket e Risco de Conexões Zumbis
- **Localização**: `internal/server/server.go` (linhas 1922–1945 e 1968–1982)
- **Descrição**:
  A rota `/api/stream` não configura `SetReadDeadline` nem monitora `Ping/Pong`. Se o frontend fechar abruptamente sem TCP FIN (ex: suspensão de energia ou kill do processo), a conexão fica em `wsClients` até o próximo `broadcastJSON`, onde trava no mutex compartilhado `writeMu` pelo tempo de timeout (2s por cliente morto).
- **Correção Recomendada**:
  Adicionar ticker de Ping periódico no servidor com detecção de desconexão em 10s.

---

## 4. Matriz de Severidade e Impacto

| ID | Componente | Descrição Resumida | Severidade | Esforço p/ Corrigir |
|---|---|---|---|---|
| **3.1** | `server.go` | CORS permissivo permite CSRF na API local | 🔴 Crítica | Baixo (10 linhas) |
| **3.2** | `files.go` | Leitura arbitrária de arquivos sem sandbox de pasta | 🔴 Crítica | Médio |
| **3.3** | `framer.go` | Buffer de 4MB no `bufio.Scanner` causa crash silencioso | 🔴 Crítica | Médio |
| **3.4** | `export_pdf.go`, `server.go` | Comandos Unix (`xdg-open`, `notify-send`, `xclip`) no Windows | 🟠 Alta | Médio |
| **3.5** | `process_windows.go` | Processos filhos órfãos no Windows por falta de Job Object | 🟠 Alta | Médio |
| **3.6** | `App.tsx` | Polling de 15s retarda diálogo de aprovação de segurança | 🟠 Alta | Baixo |
| **3.7** | `acp/client.go` | Contexto de prompt inflado por 29 tools em toda conversa | 🟡 Média | Médio |
| **3.8** | `BrowserToolCard.tsx` | Imagens em base64 no React consomem RAM excessiva | 🟡 Média | Baixo |
| **3.9** | `server.go` | Pasta `screenshots/<id>` não deletada ao excluir conversa | 🟡 Média | Baixo |
| **3.10** | `mcp/agy.go` | Gestor MCP hardcoded no `agy`, ignorando `gemini` CLI | 🟡 Média | Baixo |
| **3.11** | `server.go` | Falta de Ping/Pong no WebSocket | 🟢 Baixa | Baixo |

---

## 5. Questões Propostas para o Modelo Revisor

Ao submeter este diagnóstico ao outro modelo para contra-análise e debate, sugerem-se as seguintes perguntas diretivas:

1. **Sobre Segurança da API Local**:
   *Concorda que a falta de bloqueio de requisições de origem cruzada (`Origin`) desconhecidas no `corsMiddleware` expõe a aplicação a CSRF a partir do navegador real do usuário? Qual a melhor estratégia de baixo atrito: tokens de sessão efêmeros em memória ou checagem estrita de cabeçalho `Origin` / `Sec-Fetch-Site: same-origin`?*
2. **Sobre Portabilidade Windows**:
   *Considerando a stack Go + Wails v2, qual a forma mais limpa de substituir o `xclip` e `xdg-open` no Windows sem inflar o binário com dependências CGO? O uso de Win32 Job Objects via syscall no Go para matar a árvore de processos do agente é a abordagem canônica recomendada?*
3. **Sobre Consumo de Tokens no ACP**:
   *Para agentes baseados em ACP com 29 tools de navegador, qual arquitetura você recomenda para evitar que o custo de tokens dispare de 25k para 50k tokens em fluxos triviais: tools dinâmicas via submodos (ex: modo `browse` vs modo `code`), ou resumo de schemas?*
4. **Sobre o Parser JSON-RPC (`framer.go`)**:
   *Qual padrão em Go você preconiza para substituir o `bufio.Scanner` em streams JSON-RPC onde payloads de imagem base64 ou dados de snapshot podem ultrapassar limites estáticos sem causar alocação infinita de memória?*
