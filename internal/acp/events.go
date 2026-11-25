package acp

// Status de chamadas de ferramenta suportados
type ToolCallStatus string

const (
	ToolCallPending    ToolCallStatus = "pending"
	ToolCallInProgress ToolCallStatus = "in_progress"
	ToolCallCompleted  ToolCallStatus = "completed"
	ToolCallFailed     ToolCallStatus = "failed"
)

// MessageChunk representa um pedaço de texto gerado pelo agente
type MessageChunk struct {
	Text string `json:"text"`
}

// ThoughtChunk representa um pedaço de pensamento interno ("thinking")
type ThoughtChunk struct {
	Text string `json:"text"`
}

// ToolCall representa a invocação ou atualização de uma ferramenta
type ToolCall struct {
	ID       string                 `json:"id"`
	Status   ToolCallStatus         `json:"status"`
	Title    string                 `json:"title"`
	Kind     string                 `json:"kind,omitempty"`
	RawInput map[string]interface{} `json:"rawInput,omitempty"`
}

// PlanEntry representa uma etapa do plano de execução
type PlanEntry struct {
	Content  string `json:"content"`
	Status   string `json:"status"`
	Priority string `json:"priority,omitempty"`
}

// Plan representa o plano de ação geral
type Plan struct {
	Entries []PlanEntry `json:"entries"`
}

// PermissionOption define uma opção disponível em um pedido de confirmação
type PermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

// PermissionRequest representa o pedido do agente para executar uma ferramenta
type PermissionRequest struct {
	CallID   string             `json:"callId"`
	Options  []PermissionOption `json:"options,omitempty"`
	ToolCall ToolCall           `json:"toolCall"`
}

// TurnDone indica a conclusão de um turno de interação
type TurnDone struct {
	StopReason string `json:"stopReason"`
}

// ErrorEvent reporta falhas durante o processamento do turno
type ErrorEvent struct {
	Msg string `json:"msg"`
}

// Event encapsula todos os tipos de eventos transmitidos para a UI
type Event struct {
	Type              string             `json:"type"` // message_chunk, thought_chunk, tool_call, plan, permission_request, turn_done, error
	SessionID         string             `json:"sessionId,omitempty"`
	ConversationID    string             `json:"conversationId,omitempty"`
	MessageChunk      *MessageChunk      `json:"messageChunk,omitempty"`
	ThoughtChunk      *ThoughtChunk      `json:"thoughtChunk,omitempty"`
	ToolCall          *ToolCall          `json:"toolCall,omitempty"`
	Plan              *Plan              `json:"plan,omitempty"`
	PermissionRequest *PermissionRequest `json:"permissionRequest,omitempty"`
	TurnDone          *TurnDone          `json:"turnDone,omitempty"`
	Error             *ErrorEvent        `json:"error,omitempty"`
}
