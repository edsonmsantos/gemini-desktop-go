package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type rpcReq struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

var (
	writeMu       sync.Mutex
	cancelCh      = make(chan struct{}, 1)
	permRespCh    = make(chan string, 1) // transmite o optionId recebido
	authenticated = false
	requireAuth   = false
	sessCounter   = 0
	dieMidTurn    = false
	dieMarker     = ""
)

func send(msg interface{}) {
	b, _ := json.Marshal(msg)
	writeMu.Lock()
	fmt.Fprintf(os.Stdout, "%s\n", string(b))
	writeMu.Unlock()
}

func sendResponse(id int64, result interface{}) {
	send(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func sendError(id int64, code int, message string) {
	send(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

func sendNotification(method string, params interface{}) {
	send(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
}

func main() {
	reqAuthFlag := flag.Bool("require-auth", false, "Exige authenticate antes de permitir session/new")
	dieMidTurnFlag := flag.Bool("die-mid-turn", false, "Simula morte no meio do primeiro prompt")
	dieMarkerFlag := flag.String("die-marker", "", "Arquivo marcador para morrer apenas uma vez")
	_ = flag.Bool("acp", false, "Modo ACP")
	_ = flag.Bool("experimental-acp", false, "Modo ACP experimental")
	flag.Parse()
	requireAuth = *reqAuthFlag
	dieMidTurn = *dieMidTurnFlag
	dieMarker = *dieMarkerFlag

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req rpcReq
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}

		// Se recebemos a resposta de permissão enviada (id 999)
		if req.ID != nil && *req.ID == 999 && req.Method == "" {
			var res struct {
				Allow    bool   `json:"allow"`
				Outcome  string `json:"outcome"`
				OptionID string `json:"optionId"`
			}
			_ = json.Unmarshal(req.Result, &res)
			permRespCh <- res.OptionID
			continue
		}

		switch req.Method {
		case "initialize":
			sendResponse(*req.ID, map[string]interface{}{
				"protocolVersion": 1,
				"agentInfo": map[string]interface{}{
					"name":    "fake-gemini",
					"title":   "Fake Gemini Agent",
					"version": "1.0.0",
				},
				"authMethods": []map[string]interface{}{
					{"id": "oauth-personal", "name": "Log in with Google", "description": "Log in with your Google account"},
					{"id": "gemini-api-key", "name": "Gemini API key", "description": "Use an API key with Gemini Developer API"},
				},
				"models": []map[string]string{
					{"id": "gemini-2.5-flash", "label": "Gemini 2.5 Flash"},
				},
			})

		case "authenticate":
			var params struct {
				MethodID string `json:"methodId"`
			}
			_ = json.Unmarshal(req.Params, &params)
			authenticated = true
			sendResponse(*req.ID, map[string]interface{}{
				"status":   "authenticated",
				"methodId": params.MethodID,
			})

		case "session/new":
			if requireAuth && !authenticated {
				sendError(*req.ID, -32000, "Gemini API key is missing or not configured.")
				continue
			}
			sessCounter++
			sID := fmt.Sprintf("fake-sess-%d", sessCounter)
			if sessCounter == 1 {
				sID = "fake-sess-123"
			}
			sendResponse(*req.ID, map[string]interface{}{
				"sessionId": sID,
				"models": map[string]interface{}{
					"currentModelId": "gemini-2.5-flash",
					"availableModels": []map[string]string{
						{"id": "gemini-2.5-flash", "label": "Gemini 2.5 Flash"},
						{"id": "gemini-2.5-pro", "label": "Gemini 2.5 Pro"},
					},
				},
				"modes": map[string]interface{}{
					"currentModeId": "default",
					"availableModes": []map[string]string{
						{"id": "default", "name": "Default", "description": "Ask before edits and commands"},
						{"id": "accept-edits", "name": "Accept edits", "description": "Auto accept edits"},
						{"id": "plan-only", "name": "Plan only", "description": "Only plan without edits"},
					},
				},
				"configOptions": []map[string]interface{}{
					{
						"id":           "model",
						"name":         "Model",
						"category":     "model",
						"type":         "select",
						"currentValue": "gemini-2.5-flash",
						"options": []map[string]string{
							{"value": "gemini-2.5-flash", "name": "Gemini 2.5 Flash"},
						},
					},
				},
				"_meta": map[string]interface{}{
					"antigravity": map[string]interface{}{
						"tier": map[string]string{
							"name":       "Google AI Pro",
							"id":         "g1-pro-tier",
							"upgradeUri": "https://one.google.com/explore-plan",
						},
					},
				},
			})

		case "session/load":
			var params struct {
				SessionID string `json:"sessionId"`
			}
			_ = json.Unmarshal(req.Params, &params)
			if strings.Contains(params.SessionID, "trigger-unsupported-resume-32601") {
				sendError(*req.ID, -32601, "Method not found: session/load")
				continue
			}
			sendResponse(*req.ID, map[string]interface{}{
				"sessionId": params.SessionID,
			})

		case "session/set_mode":
			var params struct {
				SessionID string `json:"sessionId"`
				ModeID    string `json:"modeId"`
			}
			_ = json.Unmarshal(req.Params, &params)
			if strings.Contains(params.ModeID, "trigger-32601") || strings.Contains(params.SessionID, "trigger-32601") {
				sendError(*req.ID, -32601, "Method not found: session/set_mode")
				continue
			}
			sendResponse(*req.ID, map[string]interface{}{})

		case "session/set_model":
			var params struct {
				ModelID string `json:"modelId"`
			}
			_ = json.Unmarshal(req.Params, &params)
			if strings.Contains(params.ModelID, "trigger-32601") {
				sendError(*req.ID, -32601, "Method not found: session/set_model")
				continue
			}
			sendResponse(*req.ID, map[string]interface{}{})

		case "session/close":
			sendResponse(*req.ID, map[string]interface{}{})

		case "session/cancel":
			select {
			case cancelCh <- struct{}{}:
			default:
			}

		case "session/prompt":
			go handlePrompt(req)

		default:
			if req.ID != nil {
				sendResponse(*req.ID, map[string]interface{}{})
			}
		}
	}
}

func handlePrompt(req rpcReq) {
	var params struct {
		SessionID string `json:"sessionId"`
		Prompt    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"prompt"`
	}
	_ = json.Unmarshal(req.Params, &params)

	promptText := ""
	if len(params.Prompt) > 0 {
		promptText = params.Prompt[0].Text
	}

	sessID := params.SessionID
	if sessID == "" {
		sessID = "fake-sess-123"
	}

	markerPath := dieMarker
	if markerPath == "" {
		markerPath = filepath.Join(os.TempDir(), fmt.Sprintf("fake_agent_die_%d.marker", os.Getppid()))
	}
	shouldDie := false
	if dieMidTurn {
		if _, err := os.Stat(markerPath); os.IsNotExist(err) {
			_ = os.WriteFile(markerPath, []byte("died"), 0644)
			shouldDie = true
		}
	} else if strings.Contains(promptText, "die-mid-turn") {
		shouldDie = true
	}

	if shouldDie {
		sendNotification("session/update", map[string]interface{}{
			"sessionId": sessID,
			"update": map[string]interface{}{
				"sessionUpdate": "agent_thought_chunk",
				"content": map[string]interface{}{
					"type": "text",
					"text": "Pensando antes da falha fatal...",
				},
			},
		})
		time.Sleep(50 * time.Millisecond)
		_ = os.Stdout.Close()
		os.Exit(1)
		return
	}

	if strings.Contains(promptText, "test-array-chunks") {
		// Envia chunks onde content é um ARRAY de múltiplos itens com type e text
		sendNotification("session/update", map[string]interface{}{
			"sessionId": sessID,
			"update": map[string]interface{}{
				"sessionUpdate": "agent_message_chunk",
				"content": []map[string]interface{}{
					{"type": "text", "text": "Parte Um. "},
					{"type": "text", "text": "Parte Dois."},
				},
			},
		})

		sendResponse(*req.ID, map[string]interface{}{
			"stopReason": "end_turn",
		})
		return
	}

	if strings.Contains(promptText, "test-permission") {
		// Envia mensagem inicial
		sendNotification("session/update", map[string]interface{}{
			"sessionId": sessID,
			"update": map[string]interface{}{
				"sessionUpdate": "agent_message_chunk",
				"content": map[string]interface{}{
					"type": "text",
					"text": "Solicitando permissão para executar ação...",
				},
			},
		})

		// Tool call
		sendNotification("session/update", map[string]interface{}{
			"sessionId": sessID,
			"update": map[string]interface{}{
				"sessionUpdate": "tool_call",
				"tool_call_id":  "tc-1",
				"status":        "pending",
				"title":         "Executar comando shell",
				"kind":          "execute",
				"rawInput": map[string]interface{}{
					"command": "rm -rf /tmp/test",
				},
			},
		})

		// Envia pedido de permissão com optionIds customizados e específicos
		reqID := int64(999)
		send(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      reqID,
			"method":  "session/request_permission",
			"params": map[string]interface{}{
				"sessionId": sessID,
				"options": []map[string]interface{}{
					{"optionId": "opt-custom-allow-77", "name": "Permitir Execução Segura", "kind": "allow_once"},
					{"optionId": "opt-custom-reject-77", "name": "Negar Execução", "kind": "reject_once"},
				},
				"toolCall": map[string]interface{}{
					"id":     "tc-1",
					"status": "pending",
					"title":  "Executar comando shell",
					"kind":   "execute",
				},
			},
		})

		// Aguarda o optionId retornado pelo cliente
		receivedOptionID := <-permRespCh

		if receivedOptionID == "opt-custom-allow-77" || receivedOptionID == "allow_once" {
			sendNotification("session/update", map[string]interface{}{
				"sessionId": sessID,
				"update": map[string]interface{}{
					"sessionUpdate": "tool_call_update",
					"tool_call_id":  "tc-1",
					"status":        "completed",
					"title":         "Executar comando shell",
				},
			})
			sendNotification("session/update", map[string]interface{}{
				"sessionId": sessID,
				"update": map[string]interface{}{
					"sessionUpdate": "agent_message_chunk",
					"content": map[string]interface{}{
						"type": "text",
						"text": "Opção aceita: " + receivedOptionID,
					},
				},
			})
		} else {
			sendNotification("session/update", map[string]interface{}{
				"sessionId": sessID,
				"update": map[string]interface{}{
					"sessionUpdate": "tool_call_update",
					"tool_call_id":  "tc-1",
					"status":        "failed",
					"title":         "Executar comando shell",
				},
			})
			sendNotification("session/update", map[string]interface{}{
				"sessionId": sessID,
				"update": map[string]interface{}{
					"sessionUpdate": "agent_message_chunk",
					"content": map[string]interface{}{
						"type": "text",
						"text": "Opção negada: " + receivedOptionID,
					},
				},
			})
		}

		sendResponse(*req.ID, map[string]interface{}{
			"stopReason": "end_turn",
		})
		return
	}

	if strings.Contains(promptText, "test-cancel") {
		sendNotification("session/update", map[string]interface{}{
			"sessionId": sessID,
			"update": map[string]interface{}{
				"sessionUpdate": "agent_message_chunk",
				"content": map[string]interface{}{
					"type": "text",
					"text": "Processando tarefa de longa duração...",
				},
			},
		})

		// Aguarda notificação de cancelamento
		<-cancelCh

		sendResponse(*req.ID, map[string]interface{}{
			"stopReason": "cancelled",
		})
		return
	}

	if strings.Contains(promptText, "test-slow") {
		// Aguarda cancelamento ou timeout longo
		select {
		case <-cancelCh:
			sendResponse(*req.ID, map[string]interface{}{
				"stopReason": "cancelled",
			})
			return
		case <-time.After(10 * time.Second):
			sendResponse(*req.ID, map[string]interface{}{
				"stopReason": "end_turn",
			})
			return
		}
	}

	if strings.Contains(promptText, "test-tools-camel") {
		// Tool call 1 (in_progress) com toolCallId camelCase real
		sendNotification("session/update", map[string]interface{}{
			"sessionId": sessID,
			"update": map[string]interface{}{
				"sessionUpdate": "tool_call",
				"toolCallId":    "call_7530034839743884041",
				"kind":          "execute",
				"name":          "call_mcp_tool",
				"title":         "MCP Tool browser_status",
				"status":        "in_progress",
				"rawInput": map[string]interface{}{
					"ServerName": "browser-bridge",
					"ToolName":   "browser_status",
				},
			},
		})

		// Tool call 2 (in_progress)
		sendNotification("session/update", map[string]interface{}{
			"sessionId": sessID,
			"update": map[string]interface{}{
				"sessionUpdate": "tool_call",
				"toolCallId":    "call_7530034839743884042",
				"kind":          "execute",
				"name":          "call_mcp_tool",
				"title":         "MCP Tool browser_click",
				"status":        "in_progress",
				"rawInput": map[string]interface{}{
					"ServerName": "browser-bridge",
					"ToolName":   "browser_click",
				},
			},
		})

		// Update de sucesso da primeira chamada
		sendNotification("session/update", map[string]interface{}{
			"sessionId": sessID,
			"update": map[string]interface{}{
				"sessionUpdate": "tool_call_update",
				"toolCallId":    "call_7530034839743884041",
				"kind":          "execute",
				"name":          "call_mcp_tool",
				"title":         "MCP Tool browser_status",
				"status":        "completed",
				"rawInput": map[string]interface{}{
					"ServerName": "browser-bridge",
					"ToolName":   "browser_status",
				},
				"content": []map[string]interface{}{
					{
						"type": "content",
						"content": map[string]interface{}{
							"type": "text",
							"text": "modo: agente (porta 9223)",
						},
					},
				},
				"rawOutput": "modo: agente (porta 9223)",
			},
		})

		// Update de falha da segunda chamada
		sendNotification("session/update", map[string]interface{}{
			"sessionId": sessID,
			"update": map[string]interface{}{
				"sessionUpdate": "tool_call_update",
				"toolCallId":    "call_7530034839743884042",
				"kind":          "execute",
				"name":          "call_mcp_tool",
				"title":         "MCP Tool browser_click",
				"status":        "failed",
				"rawInput": map[string]interface{}{
					"ServerName": "browser-bridge",
					"ToolName":   "browser_click",
				},
				"content": []map[string]interface{}{
					{
						"type": "content",
						"content": map[string]interface{}{
							"type": "text",
							"text": "elemento #btn não encontrado",
						},
					},
				},
				"rawOutput": "elemento #btn não encontrado",
			},
		})

		sendResponse(*req.ID, map[string]interface{}{
			"stopReason": "end_turn",
		})
		return
	}

	// Fluxo normal
	sendNotification("session/update", map[string]interface{}{
		"sessionId": sessID,
		"update": map[string]interface{}{
			"sessionUpdate": "agent_thought_chunk",
			"content": map[string]interface{}{
				"type": "text",
				"text": "Analisando a solicitação do usuário...",
			},
		},
	})

	sendNotification("session/update", map[string]interface{}{
		"sessionId": sessID,
		"update": map[string]interface{}{
			"sessionUpdate": "agent_message_chunk",
			"content": map[string]interface{}{
				"type": "text",
				"text": "Olá! Esta é a resposta do fake agent.",
			},
		},
	})

	sendNotification("session/update", map[string]interface{}{
		"sessionId": sessID,
		"update": map[string]interface{}{
			"sessionUpdate": "plan",
			"entries": []map[string]interface{}{
				{"content": "Planejar resposta", "status": "completed"},
				{"content": "Executar ferramentas", "status": "in_progress"},
			},
		},
	})

	sendNotification("session/update", map[string]interface{}{
		"sessionId": sessID,
		"update": map[string]interface{}{
			"sessionUpdate": "tool_call",
			"tool_call_id":  "tc-0",
			"status":        "completed",
			"title":         "Consulta ao sistema",
			"kind":          "read",
		},
	})

	sendResponse(*req.ID, map[string]interface{}{
		"stopReason": "end_turn",
	})
}
