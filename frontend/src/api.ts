import { AgentInfo, CapabilitiesResponse, Conversation, MCPResponse, MCPServer, Message, Model, SearchResult, SkillsResponse, Workspace, WSEvent } from './types'

declare global {
  interface Window {
    __GMD_API__?: string
    __GEMINI_DESKTOP_CONFIG__?: {
      port?: number
      baseUrl?: string
    }
    go?: {
      main?: {
        App?: {
          GetServerURL?: () => Promise<string>
          GetConfig?: () => Promise<{ port: number; baseUrl: string; agent: string }>
          SelectDirectory?: () => Promise<string>
          SelectFiles?: () => Promise<string[]>
        }
      }
    }
  }
}

export function getBaseURL(): string {
  if (window.__GMD_API__) {
    return window.__GMD_API__
  }
  if (window.__GEMINI_DESKTOP_CONFIG__?.baseUrl) {
    return window.__GEMINI_DESKTOP_CONFIG__.baseUrl
  }
  if (typeof window !== 'undefined' && window.location && window.location.origin && window.location.origin.startsWith('http://127.0.0.1:')) {
    return window.location.origin
  }
  return 'http://127.0.0.1:8399'
}

export function getWSURL(): string {
  const base = getBaseURL()
  return base.replace(/^http/, 'ws') + '/api/stream'
}

export async function fetchAgentInfo(): Promise<AgentInfo> {
  const res = await fetch(`${getBaseURL()}/api/agent-info`)
  if (!res.ok) {
    return { email: null }
  }
  return res.json()
}

export async function fetchModels(): Promise<Model[]> {
  try {
    const res = await fetch(`${getBaseURL()}/api/models`)
    if (!res.ok) return []
    return await res.json()
  } catch {
    return []
  }
}

export async function fetchWorkspaces(): Promise<Workspace[]> {
  try {
    const res = await fetch(`${getBaseURL()}/api/workspaces`)
    if (!res.ok) return []
    return await res.json()
  } catch {
    return []
  }
}

export async function fetchConversations(): Promise<Conversation[]> {
  const res = await fetch(`${getBaseURL()}/api/conversations`)
  if (!res.ok) {
    throw new Error('Falha ao listar conversas')
  }
  return res.json()
}

export async function fetchConversation(id: string): Promise<{ conversation: Conversation; messages: Message[] }> {
  const res = await fetch(`${getBaseURL()}/api/conversations/${encodeURIComponent(id)}`)
  if (!res.ok) {
    throw new Error('Falha ao carregar conversa')
  }
  return res.json()
}

export async function createConversation(cwd?: string, modelId?: string, modeId?: string): Promise<{
  id: string
  title: string
  sessionId: string
  cwd: string
  modelId: string
  modeId?: string
}> {
  const payload: Record<string, string> = {}
  if (cwd) payload.cwd = cwd
  if (modelId) payload.modelId = modelId
  if (modeId) payload.modeId = modeId

  const res = await fetch(`${getBaseURL()}/api/conversations`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  })
  if (!res.ok) {
    const errText = await res.text()
    throw new Error(errText || 'Falha ao criar conversa')
  }
  return res.json()
}

export async function fetchCapabilities(conversationId: string): Promise<CapabilitiesResponse> {
  const res = await fetch(`${getBaseURL()}/api/conversations/${encodeURIComponent(conversationId)}/capabilities`)
  if (!res.ok) {
    return { modes: null, tier: null, configOptions: [] }
  }
  return res.json()
}

export async function setConversationMode(conversationId: string, modeId: string): Promise<{ status: string; modeId: string }> {
  const res = await fetch(`${getBaseURL()}/api/conversations/${encodeURIComponent(conversationId)}/mode`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ modeId })
  })
  if (!res.ok) {
    const errText = await res.text()
    throw new Error(errText || 'Falha ao alterar modo da conversa')
  }
  return res.json()
}

export async function patchConversation(id: string, updates: { title?: string; modelId?: string }): Promise<void> {
  const res = await fetch(`${getBaseURL()}/api/conversations/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(updates)
  })
  if (!res.ok) {
    throw new Error('Falha ao atualizar conversa')
  }
}

export async function deleteConversation(id: string): Promise<void> {
  const res = await fetch(`${getBaseURL()}/api/conversations/${encodeURIComponent(id)}`, {
    method: 'DELETE'
  })
  if (!res.ok) {
    throw new Error('Falha ao excluir conversa')
  }
}

export async function sendPrompt(conversationId: string, text: string, files?: string[]): Promise<void> {
  const payload: { text: string; files?: string[] } = { text }
  if (files && files.length > 0) {
    payload.files = files
  }

  const res = await fetch(`${getBaseURL()}/api/conversations/${encodeURIComponent(conversationId)}/prompt`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  })
  if (!res.ok) {
    const errText = await res.text()
    throw new Error(errText || 'Falha ao enviar prompt')
  }
}

export async function selectDirectoryViaWails(): Promise<string | null> {
  if (window.go?.main?.App?.SelectDirectory) {
    try {
      const dir = await window.go.main.App.SelectDirectory()
      return dir && dir.trim() ? dir.trim() : null
    } catch {
      return null
    }
  }
  return null
}

export async function selectFilesViaWails(): Promise<string[] | null> {
  if (window.go?.main?.App?.SelectFiles) {
    try {
      const files = await window.go.main.App.SelectFiles()
      return files && files.length > 0 ? files : []
    } catch {
      return null
    }
  }
  return null
}

export async function resumeConversation(id: string): Promise<{ sessionId: string }> {
  const res = await fetch(`${getBaseURL()}/api/conversations/${encodeURIComponent(id)}/resume`, {
    method: 'POST'
  })
  if (!res.ok) {
    throw new Error('Falha ao resumir conversa')
  }
  return res.json()
}

export async function respondPermission(callId: string, allow: boolean): Promise<void> {
  const res = await fetch(`${getBaseURL()}/api/permission`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ callId, allow })
  })
  if (!res.ok) {
    throw new Error('Falha ao responder permissão')
  }
}

export async function cancelTurn(sessionId?: string): Promise<void> {
  const res = await fetch(`${getBaseURL()}/api/cancel`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ sessionId: sessionId || '' })
  })
  if (!res.ok) {
    throw new Error('Falha ao cancelar turno')
  }
}

type EventCallback = (event: WSEvent) => void

export class WSManager {
  private ws: WebSocket | null = null
  private listeners: Set<EventCallback> = new Set()
  private reconnectTimer: number | null = null
  private backoffMs = 1000
  private maxBackoffMs = 10000
  private isDestroyed = false

  connect() {
    if (this.isDestroyed) return
    if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
      return
    }

    try {
      const url = getWSURL()
      this.ws = new WebSocket(url)

      this.ws.onopen = () => {
        this.backoffMs = 1000
      }

      this.ws.onmessage = (event) => {
        try {
          const parsed = JSON.parse(event.data) as WSEvent
          this.listeners.forEach((listener) => listener(parsed))
        } catch {
          // ignore malformed frame
        }
      }

      this.ws.onclose = () => {
        this.scheduleReconnect()
      }

      this.ws.onerror = () => {
        if (this.ws) {
          this.ws.close()
        }
      }
    } catch {
      this.scheduleReconnect()
    }
  }

  private scheduleReconnect() {
    if (this.isDestroyed || this.reconnectTimer !== null) return
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null
      this.backoffMs = Math.min(this.backoffMs * 1.5, this.maxBackoffMs)
      this.connect()
    }, this.backoffMs)
  }

  subscribe(listener: EventCallback): () => void {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  destroy() {
    this.isDestroyed = true
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
    this.listeners.clear()
  }
}

export const wsManager = new WSManager()

const skillsClientCache = new Map<string, { data: SkillsResponse; timestamp: number }>()

export async function fetchSkills(cwd?: string): Promise<SkillsResponse> {
  const key = cwd || ''
  const cached = skillsClientCache.get(key)
  if (cached && Date.now() - cached.timestamp < 30000) {
    return cached.data
  }
  const url = cwd ? `${getBaseURL()}/api/skills?cwd=${encodeURIComponent(cwd)}` : `${getBaseURL()}/api/skills`
  const res = await fetch(url)
  if (!res.ok) {
    throw new Error('Falha ao carregar skills')
  }
  const data = (await res.json()) as SkillsResponse
  skillsClientCache.set(key, { data, timestamp: Date.now() })
  return data
}

export async function fetchMCPServers(): Promise<MCPResponse> {
  const res = await fetch(`${getBaseURL()}/api/mcp`)
  if (res.status === 503) {
    return { servers: [], available: false }
  }
  if (!res.ok) {
    throw new Error('Falha ao carregar servidores MCP')
  }
  return res.json()
}

export async function toggleMCPServer(name: string, enable: boolean): Promise<{ name: string; status: string; server?: MCPServer }> {
  const res = await fetch(`${getBaseURL()}/api/mcp/${encodeURIComponent(name)}/toggle`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enable })
  })
  if (!res.ok) {
    const err = await res.text()
    throw new Error(err || 'Falha ao alterar estado do servidor MCP')
  }
  return res.json()
}

export async function searchConversations(query: string): Promise<SearchResult[]> {
  const trimmed = query.trim()
  if (!trimmed) return []
  const res = await fetch(`${getBaseURL()}/api/search?q=${encodeURIComponent(trimmed)}`)
  if (!res.ok) {
    throw new Error('Falha ao pesquisar conversas')
  }
  const data = await res.json()
  return data.results || []
}
