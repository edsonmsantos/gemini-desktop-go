package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNormalizeToolOutput(t *testing.T) {
	// 1. rawOut string não vazia -> usar
	rawStr := json.RawMessage(`"saida direta em texto"`)
	if got := normalizeToolOutput(rawStr, nil); got != "saida direta em texto" {
		t.Fatalf("esperava 'saida direta em texto', obteve %q", got)
	}

	// 2. rawOut objeto -> JSON compacto
	rawObj := json.RawMessage(`{
		"campo": "valor",
		"numero": 42
	}`)
	expectedCompact := `{"campo":"valor","numero":42}`
	if got := normalizeToolOutput(rawObj, nil); got != expectedCompact {
		t.Fatalf("esperava %q, obteve %q", expectedCompact, got)
	}

	// 3. rawOut array -> JSON compacto
	rawArr := json.RawMessage(`[1,  2,   3]`)
	if got := normalizeToolOutput(rawArr, nil); got != "[1,2,3]" {
		t.Fatalf("esperava '[1,2,3]', obteve %q", got)
	}

	// 4. Só content (formato real do agy: [{type:"content", content:{type:"text", text:...}}]) -> texto
	contentAgy := json.RawMessage(`[{"type":"content","content":{"type":"text","text":"modo: agente (porta 9223)"}}]`)
	if got := normalizeToolOutput(nil, contentAgy); got != "modo: agente (porta 9223)" {
		t.Fatalf("esperava 'modo: agente (porta 9223)', obteve %q", got)
	}

	// 5. Content com formatos variados: [{type:"text",text}], {type:"text",text}, ["str"] e "str"
	contentArrObj := json.RawMessage(`[{"type":"text","text":"parte 1 "},{"type":"text","text":"parte 2"}]`)
	if got := normalizeToolOutput(nil, contentArrObj); got != "parte 1 parte 2" {
		t.Fatalf("esperava 'parte 1 parte 2', obteve %q", got)
	}

	contentSingleObj := json.RawMessage(`{"type":"text","text":"texto único"}`)
	if got := normalizeToolOutput(nil, contentSingleObj); got != "texto único" {
		t.Fatalf("esperava 'texto único', obteve %q", got)
	}

	contentArrStr := json.RawMessage(`["linha 1\n", "linha 2"]`)
	if got := normalizeToolOutput(nil, contentArrStr); got != "linha 1\nlinha 2" {
		t.Fatalf("esperava 'linha 1\\nlinha 2', obteve %q", got)
	}

	contentPlainStr := json.RawMessage(`"string direta no content"`)
	if got := normalizeToolOutput(nil, contentPlainStr); got != "string direta no content" {
		t.Fatalf("esperava 'string direta no content', obteve %q", got)
	}

	// 6. Content com imagem -> [imagem <mimeType> omitida]
	contentImg := json.RawMessage(`[{"type":"image","mimeType":"image/png"}]`)
	if got := normalizeToolOutput(nil, contentImg); got != "[imagem image/png omitida]" {
		t.Fatalf("esperava '[imagem image/png omitida]', obteve %q", got)
	}

	contentImgData := json.RawMessage(`{"data":"abc", "mimeType":"image/jpeg"}`)
	if got := normalizeToolOutput(nil, contentImgData); got != "[imagem image/jpeg omitida]" {
		t.Fatalf("esperava '[imagem image/jpeg omitida]', obteve %q", got)
	}

	// 7. Base64 longo (2048+ caracteres) -> [dados binários omitidos: ~N KB]
	b64Chunk := strings.Repeat("A", 2048)
	rawB64 := json.RawMessage(`"inicio ` + b64Chunk + ` fim"`)
	gotB64 := normalizeToolOutput(rawB64, nil)
	if !strings.Contains(gotB64, "[dados binários omitidos: ~2 KB]") {
		t.Fatalf("esperava marcador de binários omitidos ~2 KB, obteve %q", gotB64)
	}
	if strings.Contains(gotB64, b64Chunk) {
		t.Fatalf("não deveria conter o payload base64 original")
	}

	// 8. Saída de 100 KB -> truncamento em 16384 bytes + marcador
	bigText := strings.Repeat("a ", 51200) // 102400 bytes com espaços (não base64 contínuo)
	bigJSON, _ := json.Marshal(bigText)
	gotBig := normalizeToolOutput(json.RawMessage(bigJSON), nil)
	expectedMarker := "\n…[saída truncada: 16384 de 102400 bytes]"
	if !strings.HasSuffix(gotBig, expectedMarker) {
		t.Fatalf("esperava sufixo %q, obteve %q", expectedMarker, gotBig)
	}
	cutPart := strings.TrimSuffix(gotBig, expectedMarker)
	if len(cutPart) != 16384 {
		t.Fatalf("esperava corte em exatamente 16384 bytes, obteve %d", len(cutPart))
	}

	// 9. Nada (vazio) -> ""
	if got := normalizeToolOutput(nil, nil); got != "" {
		t.Fatalf("esperava string vazia para inputs nil, obteve %q", got)
	}
	if got := normalizeToolOutput(json.RawMessage(""), json.RawMessage("")); got != "" {
		t.Fatalf("esperava string vazia para inputs vazios, obteve %q", got)
	}
}

func TestHandleSessionUpdate_WireFrames(t *testing.T) {
	client := &Client{
		sessionChans: make(map[string]chan<- Event),
		errorEmitted: make(map[string]bool),
		pendingPerms: make(map[string]pendingPermission),
	}

	eventCh := make(chan Event, 10)
	client.sessionChans["S"] = eventCh

	// 1. Frame de abertura real do agy 1.5.16
	openWire := []byte(`{"sessionId":"S","update":{"sessionUpdate":"tool_call","toolCallId":"call_7530034839743884042","kind":"execute","name":"call_mcp_tool","title":"MCP Tool browser_status","status":"in_progress","args":{"Arguments":{},"ServerName":"browser-bridge","ToolName":"browser_status"},"rawInput":{"Arguments":{},"ServerName":"browser-bridge","ToolName":"browser_status"}}}`)
	client.handleSessionUpdate(openWire)

	select {
	case ev := <-eventCh:
		if ev.Type != "tool_call" {
			t.Fatalf("esperava tipo tool_call, obteve %s", ev.Type)
		}
		tc := ev.ToolCall
		if tc == nil {
			t.Fatalf("ToolCall nil no evento")
		}
		if tc.ID != "call_7530034839743884042" {
			t.Fatalf("esperava ID 'call_7530034839743884042', obteve %q", tc.ID)
		}
		if tc.Name != "call_mcp_tool" {
			t.Fatalf("esperava Name 'call_mcp_tool', obteve %q", tc.Name)
		}
		if tc.Kind != "execute" {
			t.Fatalf("esperava Kind 'execute', obteve %q", tc.Kind)
		}
		if tc.Status != ToolCallInProgress {
			t.Fatalf("esperava status in_progress, obteve %s", tc.Status)
		}
		if tc.Title != "MCP Tool browser_status" {
			t.Fatalf("esperava title 'MCP Tool browser_status', obteve %q", tc.Title)
		}
	default:
		t.Fatalf("nenhum evento emitido para openWire")
	}

	// 2. Frame de sucesso real do agy 1.5.16
	successWire := []byte(`{"sessionId":"S","update":{"sessionUpdate":"tool_call_update","toolCallId":"call_7530034839743884042","kind":"execute","name":"call_mcp_tool","title":"MCP Tool browser_status","status":"completed","args":{"Arguments":{},"ServerName":"browser-bridge","ToolName":"browser_status"},"rawInput":{"Arguments":{},"ServerName":"browser-bridge","ToolName":"browser_status"},"content":[{"type":"content","content":{"type":"text","text":"modo: agente (porta 9223)"}}],"rawOutput":"modo: agente (porta 9223)"}}`)
	client.handleSessionUpdate(successWire)

	select {
	case ev := <-eventCh:
		tc := ev.ToolCall
		if tc == nil {
			t.Fatalf("ToolCall nil no evento de sucesso")
		}
		if tc.ID != "call_7530034839743884042" {
			t.Fatalf("esperava mesmo ID no update, obteve %q", tc.ID)
		}
		if tc.Status != ToolCallCompleted {
			t.Fatalf("esperava status completed, obteve %s", tc.Status)
		}
		if tc.Name != "call_mcp_tool" {
			t.Fatalf("esperava Name 'call_mcp_tool', obteve %q", tc.Name)
		}
		if tc.RawOutput != "modo: agente (porta 9223)" {
			t.Fatalf("esperava rawOutput 'modo: agente (porta 9223)', obteve %q", tc.RawOutput)
		}
	default:
		t.Fatalf("nenhum evento emitido para successWire")
	}

	// 3. Frame de falha
	failedWire := []byte(`{"sessionId":"S","update":{"sessionUpdate":"tool_call_update","toolCallId":"call_7530034839743884042","kind":"execute","name":"call_mcp_tool","title":"MCP Tool browser_status","status":"failed","content":[{"type":"content","content":{"type":"text","text":"erro ao conectar na porta 9223"}}],"rawOutput":"erro ao conectar na porta 9223"}}`)
	client.handleSessionUpdate(failedWire)

	select {
	case ev := <-eventCh:
		tc := ev.ToolCall
		if tc.Status != ToolCallFailed {
			t.Fatalf("esperava status failed, obteve %s", tc.Status)
		}
		if tc.RawOutput != "erro ao conectar na porta 9223" {
			t.Fatalf("esperava texto do erro em rawOutput, obteve %q", tc.RawOutput)
		}
	default:
		t.Fatalf("nenhum evento emitido para failedWire")
	}

	// 4. Fallback tool_call_id
	legacyWire := []byte(`{"sessionId":"S","update":{"sessionUpdate":"tool_call","tool_call_id":"tc-legacy-99","name":"cmd","title":"Legacy tool","status":"in_progress"}}`)
	client.handleSessionUpdate(legacyWire)
	select {
	case ev := <-eventCh:
		if ev.ToolCall.ID != "tc-legacy-99" {
			t.Fatalf("esperava ID 'tc-legacy-99', obteve %q", ev.ToolCall.ID)
		}
	default:
		t.Fatalf("nenhum evento emitido para legacyWire")
	}

	// 5. Fallback id
	idWire := []byte(`{"sessionId":"S","update":{"sessionUpdate":"tool_call","id":"tc-id-88","status":"in_progress"}}`)
	client.handleSessionUpdate(idWire)
	select {
	case ev := <-eventCh:
		if ev.ToolCall.ID != "tc-id-88" {
			t.Fatalf("esperava ID 'tc-id-88', obteve %q", ev.ToolCall.ID)
		}
	default:
		t.Fatalf("nenhum evento emitido para idWire")
	}

	// 6. Sem nenhum ID -> gera "tc-<UnixNano>"
	noIDWire := []byte(`{"sessionId":"S","update":{"sessionUpdate":"tool_call","title":"Sem ID","status":"in_progress"}}`)
	client.handleSessionUpdate(noIDWire)
	select {
	case ev := <-eventCh:
		if ev.ToolCall.ID == "" || !strings.HasPrefix(ev.ToolCall.ID, "tc-") {
			t.Fatalf("esperava ID gerado com prefixo tc-, obteve %q", ev.ToolCall.ID)
		}
	default:
		t.Fatalf("nenhum evento emitido para noIDWire")
	}

	// 7. Duas tool calls diferentes na mesma sessão -> dois eventos com IDs diferentes
	call1 := []byte(`{"sessionId":"S","update":{"sessionUpdate":"tool_call","toolCallId":"call_A","name":"toolA"}}`)
	call2 := []byte(`{"sessionId":"S","update":{"sessionUpdate":"tool_call","toolCallId":"call_B","name":"toolB"}}`)
	client.handleSessionUpdate(call1)
	client.handleSessionUpdate(call2)

	ev1 := <-eventCh
	ev2 := <-eventCh
	if ev1.ToolCall.ID == ev2.ToolCall.ID {
		t.Fatalf("duas chamadas diferentes geraram o mesmo ID: %q", ev1.ToolCall.ID)
	}
	if ev1.ToolCall.ID != "call_A" || ev2.ToolCall.ID != "call_B" {
		t.Fatalf("IDs incorretos: %q e %q", ev1.ToolCall.ID, ev2.ToolCall.ID)
	}
}

func TestHandlePermissionRequest_ToolCallId(t *testing.T) {
	client := &Client{
		sessionChans: make(map[string]chan<- Event),
		errorEmitted: make(map[string]bool),
		pendingPerms: make(map[string]pendingPermission),
	}

	eventCh := make(chan Event, 10)
	client.sessionChans["S"] = eventCh

	// 1. handlePermissionRequest com toolCall.toolCallId
	reqID := int64(123)
	reqMsg := IncomingMessage{
		JSONRPC: "2.0",
		ID:      &reqID,
		Method:  "session/request_permission",
		Params: json.RawMessage(`{
			"sessionId": "S",
			"options": [{"optionId": "opt-1", "name": "Permitir", "kind": "allow_once"}],
			"toolCall": {
				"toolCallId": "call_perm_123",
				"name": "call_mcp_tool",
				"title": "Permissão teste",
				"status": "pending"
			}
		}`),
	}
	client.handlePermissionRequest(reqMsg)

	select {
	case ev := <-eventCh:
		if ev.Type != "permission_request" {
			t.Fatalf("esperava tipo permission_request, obteve %s", ev.Type)
		}
		pr := ev.PermissionRequest
		if pr.CallID != "call_perm_123" {
			t.Fatalf("esperava CallID 'call_perm_123', obteve %q", pr.CallID)
		}
		if pr.ToolCall.ID != "call_perm_123" {
			t.Fatalf("esperava ToolCall.ID 'call_perm_123', obteve %q", pr.ToolCall.ID)
		}
		if pr.ToolCall.Name != "call_mcp_tool" {
			t.Fatalf("esperava ToolCall.Name 'call_mcp_tool', obteve %q", pr.ToolCall.Name)
		}
	default:
		t.Fatalf("nenhum evento emitido para permission_request")
	}

	// 2. handleSessionUpdate com sessionUpdate "request_permission"
	updatePerm := []byte(`{
		"sessionId": "S",
		"update": {
			"sessionUpdate": "request_permission",
			"toolCallId": "call_perm_456",
			"name": "exec_cmd",
			"title": "Executar comando",
			"options": [{"optionId": "opt-2", "name": "OK", "kind": "allow_once"}]
		}
	}`)
	client.handleSessionUpdate(updatePerm)

	select {
	case ev := <-eventCh:
		pr := ev.PermissionRequest
		if pr.CallID != "call_perm_456" {
			t.Fatalf("esperava CallID 'call_perm_456', obteve %q", pr.CallID)
		}
		if pr.ToolCall.ID != "call_perm_456" {
			t.Fatalf("esperava ToolCall.ID 'call_perm_456', obteve %q", pr.ToolCall.ID)
		}
		if pr.ToolCall.Name != "exec_cmd" {
			t.Fatalf("esperava ToolCall.Name 'exec_cmd', obteve %q", pr.ToolCall.Name)
		}
	default:
		t.Fatalf("nenhum evento emitido para request_permission em session/update")
	}
}

// safeBuffer implementa io.ReadWriter thread-safe para testes com Framer
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) Read(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Read(p)
}

func TestClient_PromptTimeout_Cancellation(t *testing.T) {
	// Cria pipes para simular stdin/stdout do agente
	agentInReader, agentInWriter := io.Pipe()
	agentOutReader, agentOutWriter := io.Pipe()

	defer agentInReader.Close()
	defer agentInWriter.Close()
	defer agentOutReader.Close()
	defer agentOutWriter.Close()

	framer := NewFramer(agentOutReader, agentInWriter)

	// Inicia um processo vivo simples para que IsAlive() possa checar o processo do SO
	dummyCmd := exec.Command("cat")
	if err := dummyCmd.Start(); err != nil {
		t.Fatalf("falha ao iniciar processo dummy: %v", err)
	}
	defer func() {
		if dummyCmd.Process != nil {
			_ = dummyCmd.Process.Kill()
		}
	}()

	client := &Client{
		cmd:          dummyCmd,
		framer:       framer,
		sessionChans: make(map[string]chan<- Event),
		errorEmitted: make(map[string]bool),
		pendingPerms: make(map[string]pendingPermission),
	}

	// Goroutine que monitora o que o cliente escreve para o agente
	cancelReceived := make(chan bool, 1)
	go func() {
		dec := json.NewDecoder(agentInReader)
		for {
			var msg map[string]interface{}
			if err := dec.Decode(&msg); err != nil {
				return
			}
			if msg["method"] == "session/cancel" {
				cancelReceived <- true
			}
		}
	}()

	eventCh := make(chan Event, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := client.PromptSessionWithContent(ctx, "sess-1", []interface{}{"oi"}, eventCh)
	if err == nil {
		t.Fatalf("esperava erro de timeout")
	}

	// 1. O erro devolvido deve satisfazer errors.Is(err, context.DeadlineExceeded)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("esperava que errors.Is(err, context.DeadlineExceeded) fosse true, obteve %v", err)
	}

	// 2. IsAlive() continua true (não deve marcar c.dead = true)
	if !client.IsAlive() {
		t.Fatalf("esperava que cliente continuasse vivo (IsAlive() == true)")
	}

	// 3. Notificação session/cancel foi enviada ao agente
	select {
	case <-cancelReceived:
		// Sucesso: cancel foi recebido pelo pipe
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("esperava envio de notificação session/cancel")
	}

	// 4. O evento "error" emitido contém "tempo expirou"
	select {
	case ev := <-eventCh:
		if ev.Type != "error" || ev.Error == nil {
			t.Fatalf("esperava evento de erro, obteve %+v", ev)
		}
		if !strings.Contains(ev.Error.Msg, "tempo expirou") {
			t.Fatalf("esperava mensagem contendo 'tempo expirou', obteve %q", ev.Error.Msg)
		}
	default:
		t.Fatalf("nenhum evento de erro emitido no canal")
	}
}
