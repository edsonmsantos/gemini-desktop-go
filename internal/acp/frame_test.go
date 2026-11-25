package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestFramer_ResponseCorrelation(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	framer := NewFramer(clientConn, clientConn)
	framer.Start()
	defer framer.Close()

	go func() {
		buf := make([]byte, 1024)
		n, err := serverConn.Read(buf)
		if err != nil {
			return
		}
		var req struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(buf[:n], &req)
		resBytes := []byte(`{"jsonrpc":"2.0","id":` + fmt.Sprintf("%d", req.ID) + `,"result":{"status":"ok"}}` + "\n")
		_, _ = serverConn.Write(resBytes)
	}()

	res, err := framer.Call(ctx, "test_method", map[string]string{"foo": "bar"})
	if err != nil {
		t.Fatalf("esperava sucesso na correlação, obteve erro: %v", err)
	}

	var resMap map[string]string
	if err := json.Unmarshal(res, &resMap); err != nil {
		t.Fatalf("erro ao deserializar result: %v", err)
	}
	if resMap["status"] != "ok" {
		t.Fatalf("esperava status 'ok', obteve %q", resMap["status"])
	}
}

func TestFramer_NotificationRouting(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	framer := NewFramer(clientConn, clientConn)
	notifCh := make(chan IncomingMessage, 10)
	framer.SetNotificationHandler(func(msg IncomingMessage) {
		notifCh <- msg
	})
	framer.Start()
	defer framer.Close()

	notifPayload := `{"jsonrpc":"2.0","method":"session/update","params":{"session_id":"s1","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"hello world"}}}}` + "\n"
	_, err := serverConn.Write([]byte(notifPayload))
	if err != nil {
		t.Fatalf("falha ao enviar notificação: %v", err)
	}

	select {
	case msg := <-notifCh:
		if msg.Method != "session/update" {
			t.Fatalf("esperava method session/update, obteve %q", msg.Method)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout aguardando notificação")
	}
}

func TestFramer_InvalidLineDoesNotCrash(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	framer := NewFramer(clientConn, clientConn)
	framer.Start()
	defer framer.Close()

	go func() {
		buf := make([]byte, 1024)
		_, _ = serverConn.Read(buf) // Lê a requisição do cliente
		// Envia linha inválida seguida de uma linha de resposta válida para a requisição id 1
		_, _ = serverConn.Write([]byte("not a valid json string \n\n"))
		time.Sleep(10 * time.Millisecond)
		_, _ = serverConn.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"valid":true}}` + "\n"))
	}()

	res, err := framer.Call(ctx, "echo", nil)
	if err != nil {
		t.Fatalf("framer falhou após receber linha inválida: %v", err)
	}

	var resMap map[string]bool
	if err := json.Unmarshal(res, &resMap); err != nil {
		t.Fatalf("erro ao deserializar result: %v", err)
	}
	if !resMap["valid"] {
		t.Fatalf("esperava valid=true")
	}
}

func TestFramer_LargePayload(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	framer := NewFramer(clientConn, clientConn)
	framer.Start()
	defer framer.Close()

	// Gera string de 2MB
	bigText := strings.Repeat("A", 2*1024*1024)
	go func() {
		buf := make([]byte, 1024)
		_, _ = serverConn.Read(buf) // Lê a requisição do cliente
		// Envia resposta grande com 2MB
		payload := `{"jsonrpc":"2.0","id":1,"result":{"big":"` + bigText + `"}}` + "\n"
		_, _ = serverConn.Write([]byte(payload))
	}()

	res, err := framer.Call(ctx, "big", nil)
	if err != nil {
		t.Fatalf("framer falhou ao processar payload grande: %v", err)
	}

	var resMap map[string]string
	if err := json.Unmarshal(res, &resMap); err != nil {
		t.Fatalf("erro ao deserializar result grande: %v", err)
	}
	if len(resMap["big"]) != len(bigText) {
		t.Fatalf("tamanho recebido %d diferente do esperado %d", len(resMap["big"]), len(bigText))
	}
}
