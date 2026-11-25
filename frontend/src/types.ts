export type ToolCallStatus = 'pending' | 'in_progress' | 'completed' | 'failed'

export interface ToolCall {
  id: string
  status: ToolCallStatus
  title: string
  kind?: string
  rawInput?: Record<string, unknown>
  rawOutput?: string
}

export interface PlanEntry {
  content: string
  status: string
  priority?: string
}

export interface Plan {
  entries: PlanEntry[]
}

export interface PermissionOption {
  optionId: string
  name: string
  kind: string
}

export interface PermissionRequest {
  callId: string
  options?: PermissionOption[]
  toolCall: ToolCall
}

export interface Conversation {
  id: string
  title: string
  agent: string
  modelId: string
  modeId?: string
  cwd: string
  sessionId: string
  createdAt: number
  updatedAt: number
}

export interface Message {
  id: number | string // number do backend; string em mensagens otimistas (makeTempId)
  conversationId: string
  seq: number
  role: 'user' | 'assistant' | 'thought' | 'tool_call' | 'plan' | 'permission' | 'error'
  content: string
  meta?: string
  createdAt: number
}

export interface Workspace {
  cwd: string
  lastUsedAt: number
}

export interface Model {
  id: string
  name: string
  description?: string
  isDefault?: boolean
}

export interface ModeInfo {
  id: string
  name: string
  description?: string
}

export interface ModesInfo {
  currentModeId?: string
  current?: string
  availableModes?: ModeInfo[]
  available?: ModeInfo[]
}

export interface TierInfo {
  name: string
  id?: string
  upgradeUri?: string
}

export interface Skill {
  slug: string
  name: string
  description: string
  scope: 'user' | 'workspace'
  path: string
}

export interface SkillsResponse {
  skills: Skill[]
  userDir: string
  workspaceDir: string | null
}

export interface MCPServer {
  name: string
  type: string
  status: 'enabled' | 'disabled' | 'other'
  command: string
}

export interface MCPResponse {
  servers: MCPServer[]
  available: boolean
}

export interface SearchResult {
  conversationId: string
  title: string
  cwd: string
  snippet: string
  updatedAt: number
}

export interface ConfigOption {
  id: string
  name: string
  category?: string
  type?: string
  currentValue?: unknown
  options?: Array<Record<string, unknown>>
}

export interface CapabilitiesResponse {
  modes?: ModesInfo | null
  tier?: TierInfo | null
  configOptions?: ConfigOption[]
}

export interface AgentInfo {
  email: string | null
  tier?: TierInfo | null
}

export interface WSEvent {
  type: 'message_chunk' | 'thought_chunk' | 'tool_call' | 'plan' | 'permission_request' | 'turn_done' | 'error'
  sessionId?: string
  conversationId?: string
  messageChunk?: { text: string }
  thoughtChunk?: { text: string }
  toolCall?: ToolCall
  plan?: Plan
  permissionRequest?: PermissionRequest
  turnDone?: { stopReason: string }
  error?: { msg: string }
}

export interface Artifact {
  path: string
  name: string
  size: number
  modified: number
}

export interface DiagnosticsInfo {
  agentBinary: string
  agentFound: boolean
  agentAlive: boolean
  googleEmail: string | null
  dbOk: boolean
  port: number
}

export interface OnboardingStatus {
  done: boolean
}

export interface FileContent {
  kind: 'text' | 'image'
  mime: string
  content: string
}

export interface SourceResolution {
  from: string
  to: string
  status: number
}

export interface SavedPrompt {
  id: string
  title: string
  content: string
  createdAt?: number
  updatedAt?: number
}

