package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// IncomingMessage representa qualquer mensagem recebida no canal stdio
type IncomingMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *jsonRPCError) Error() string {
	return fmt.Sprintf("jsonrpc error (code %d): %s", e.Code, e.Message)
}

// Framer gerencia o framing newline-delimited de mensagens JSON-RPC 2.0
type Framer struct {
	reader io.Reader
	writer io.Writer

	reqID   int64
	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[int64]chan *IncomingMessage

	notifMu      sync.RWMutex
	notifHandler func(IncomingMessage)

	done chan struct{}
}

// NewFramer cria um novo Framer sobre streams de leitura e escrita
func NewFramer(r io.Reader, w io.Writer) *Framer {
	return &Framer{
		reader:  r,
		writer:  w,
		pending: make(map[int64]chan *IncomingMessage),
		done:    make(chan struct{}),
	}
}

// SetNotificationHandler define a função de callback para notificações ou requests de entrada
func (f *Framer) SetNotificationHandler(fn func(IncomingMessage)) {
	f.notifMu.Lock()
	defer f.notifMu.Unlock()
	f.notifHandler = fn
}

// Start inicia o loop de leitura das mensagens delimitadas por \n
func (f *Framer) Start() {
	go f.readLoop()
}

func (f *Framer) readLoop() {
	scanner := bufio.NewScanner(f.reader)
	// Aloca buffer inicial de 64KB e máximo de pelo menos 4MB per spec
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg IncomingMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			// Linha inválida ou corrompida: não derruba o framer
			continue
		}

		if msg.ID != nil && msg.Method == "" {
			// Resposta correlacionada por id
			f.pendingMu.Lock()
			ch, ok := f.pending[*msg.ID]
			delete(f.pending, *msg.ID)
			f.pendingMu.Unlock()

			if ok {
				ch <- &msg
			}
		} else {
			// Notificação ou requisição iniciada pelo agente
			f.notifMu.RLock()
			handler := f.notifHandler
			f.notifMu.RUnlock()

			if handler != nil {
				handler(msg)
			}
		}
	}

	// Scanner terminou (EOF ou erro de conexão)
	f.pendingMu.Lock()
	for id, ch := range f.pending {
		close(ch)
		delete(f.pending, id)
	}
	f.pendingMu.Unlock()
}

// Call envia uma requisição JSON-RPC e aguarda a resposta correspondente pelo id
func (f *Framer) Call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := atomic.AddInt64(&f.reqID, 1)

	var rawParams json.RawMessage
	if params != nil {
		pBytes, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("erro ao serializar params: %w", err)
		}
		rawParams = pBytes
	}

	req := struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int64           `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  rawParams,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar request: %w", err)
	}
	reqBytes = append(reqBytes, '\n')

	respCh := make(chan *IncomingMessage, 1)
	f.pendingMu.Lock()
	f.pending[id] = respCh
	f.pendingMu.Unlock()

	f.writeMu.Lock()
	_, err = f.writer.Write(reqBytes)
	f.writeMu.Unlock()
	if err != nil {
		f.pendingMu.Lock()
		delete(f.pending, id)
		f.pendingMu.Unlock()
		return nil, fmt.Errorf("erro ao escrever requisição: %w", err)
	}

	select {
	case <-ctx.Done():
		f.pendingMu.Lock()
		delete(f.pending, id)
		f.pendingMu.Unlock()
		return nil, ctx.Err()
	case resp, ok := <-respCh:
		if !ok || resp == nil {
			return nil, fmt.Errorf("conexão encerrada antes da resposta para requisição %d", id)
		}
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	}
}

// Notify envia uma notificação JSON-RPC (sem id)
func (f *Framer) Notify(method string, params interface{}) error {
	var rawParams json.RawMessage
	if params != nil {
		pBytes, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("erro ao serializar params da notificação: %w", err)
		}
		rawParams = pBytes
	}

	notif := struct {
		JSONRPC string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}{
		JSONRPC: "2.0",
		Method:  method,
		Params:  rawParams,
	}

	bytes, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("erro ao serializar notificação: %w", err)
	}
	bytes = append(bytes, '\n')

	f.writeMu.Lock()
	_, err = f.writer.Write(bytes)
	f.writeMu.Unlock()
	return err
}

// Respond envia uma resposta JSON-RPC a uma solicitação vinda do outro lado
func (f *Framer) Respond(id int64, result interface{}, rpcErr *jsonRPCError) error {
	var rawResult json.RawMessage
	if result != nil {
		b, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("erro ao serializar resultado da resposta: %w", err)
		}
		rawResult = b
	}

	resp := struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int64           `json:"id"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   *jsonRPCError   `json:"error,omitempty"`
	}{
		JSONRPC: "2.0",
		ID:      id,
		Result:  rawResult,
		Error:   rpcErr,
	}

	bytes, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("erro ao serializar resposta: %w", err)
	}
	bytes = append(bytes, '\n')

	f.writeMu.Lock()
	_, err = f.writer.Write(bytes)
	f.writeMu.Unlock()
	return err
}

// Close limpa recursos
func (f *Framer) Close() error {
	f.pendingMu.Lock()
	for id, ch := range f.pending {
		close(ch)
		delete(f.pending, id)
	}
	f.pendingMu.Unlock()
	return nil
}
