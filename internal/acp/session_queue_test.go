package acp

import (
	"fmt"
	"testing"
	"time"
)

func TestSessionQueue_NonBlockingAndDrain(t *testing.T) {
	destCh := make(chan Event, 50)
	q := newSessionQueue("sess-test", destCh)
	go q.run()

	const count = 30
	for i := 0; i < count; i++ {
		q.push(Event{
			Type:      "message_chunk",
			SessionID: "sess-test",
			MessageChunk: &MessageChunk{
				Text: fmt.Sprintf("chunk-%d", i),
			},
		})
	}

	received := 0
	doneDraining := make(chan struct{})
	go func() {
		for range destCh {
			received++
			if received == count {
				close(doneDraining)
				return
			}
		}
	}()

	select {
	case <-doneDraining:
		// Sucesso: todos os eventos foram drenados sem perda
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout aguardando drenagem da fila; recebeu %d de %d", received, count)
	}

	q.closeAndWait()
}
