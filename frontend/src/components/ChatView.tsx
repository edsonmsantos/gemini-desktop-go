import React, { useRef, useEffect, useState, useCallback } from 'react'
import { MessageBubble } from './MessageBubble'
import { PromptBar } from './PromptBar'
import { ThinkingBlock } from './ThinkingBlock'
import { ToolCallCard } from './ToolCallCard'
import { TodoPanel } from './TodoPanel'
import { MarkdownRenderer } from './MarkdownRenderer'
import { fetchArtifacts, openExternalURL } from '../api'
import { GeminiSparkle, RefreshIcon } from './icons'
import { Artifact, Message, Model, ModesInfo, PermissionRequest, Plan, ToolCall } from '../types'

interface ChatViewProps {
  messages: Message[]
  streamingText: string
  streamingThought: string
  activeToolCalls: ToolCall[]
  pendingPermission: PermissionRequest | null
  activePlan: Plan | null
  isStreaming: boolean
  prompt: string
  onChangePrompt: (val: string) => void
  onSend: () => void
  onCancel: () => void
  onRespondPermission: (callId: string, allow: boolean) => void
  models: Model[]
  selectedModelId: string
  onSelectModel: (id: string) => void
  modes?: ModesInfo | null
  selectedModeId?: string
  onSelectMode?: (id: string) => void
  files?: string[]
  onAddFiles?: (files: string[]) => void
  onRemoveFile?: (index: number) => void
  cwd?: string
  onRegenerate?: () => void
  onEditMessage?: (newText: string) => void
  conversationTitle?: string
  conversationId?: string
}

// WorkingIndicator: mostra que o agente está trabalhando enquanto nenhum
// conteúdo visível chegou (igual ao "Working 5s" do ZCode)
const WorkingIndicator: React.FC = () => {
  const [elapsed, setElapsed] = useState(0)
  useEffect(() => {
    const t = setInterval(() => setElapsed((s) => s + 1), 1000)
    return () => clearInterval(t)
  }, [])
  const label = elapsed < 60 ? `${elapsed}s` : `${Math.floor(elapsed / 60)}m ${elapsed % 60}s`
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 8, margin: '18px 0 6px 0' }}>
      <GeminiSparkle size={16} className="animate-spin" color="var(--accent-blue)" />
      <span style={{ fontSize: 13, fontWeight: 500, color: 'var(--text-primary)' }}>Trabalhando</span>
      <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>{label}</span>
    </div>
  )
}

export const ChatView: React.FC<ChatViewProps> = ({
  messages,
  streamingText,
  streamingThought,
  activeToolCalls,
  pendingPermission,
  activePlan,
  isStreaming,
  prompt,
  onChangePrompt,
  onSend,
  onCancel,
  onRespondPermission,
  models,
  selectedModelId,
  onSelectModel,
  modes,
  selectedModeId,
  onSelectMode,
  files = [],
  onAddFiles,
  onRemoveFile,
  cwd,
  onRegenerate,
  onEditMessage,
  conversationTitle,
  conversationId
}) => {
  // Índices das mensagens-alvo das ações: último prompt do usuário e última resposta/erro do assistente
  let lastUserIdx = -1
  let lastAssistantOrErrIdx = -1
  for (let i = messages.length - 1; i >= 0; i--) {
    if (lastUserIdx === -1 && messages[i].role === 'user') lastUserIdx = i
    if (lastAssistantOrErrIdx === -1 && (messages[i].role === 'assistant' || messages[i].role === 'error')) {
      lastAssistantOrErrIdx = i
    }
    if (lastUserIdx !== -1 && lastAssistantOrErrIdx !== -1) break
  }
  // ===== Artefatos da conversa =====
  const [artifacts, setArtifacts] = useState<Artifact[]>([])
  const [artifactsOpen, setArtifactsOpen] = useState<boolean>(false)
  const [loadingArtifacts, setLoadingArtifacts] = useState<boolean>(false)

  const loadArtifacts = useCallback(async () => {
    if (!conversationId) return
    setLoadingArtifacts(true)
    const list = await fetchArtifacts(conversationId)
    setArtifacts(list)
    setLoadingArtifacts(false)
  }, [conversationId])

  // Recarrega na abertura da conversa e ao terminar um turno (contagem do pill)
  useEffect(() => {
    if (!isStreaming) loadArtifacts()
  }, [isStreaming, conversationId, loadArtifacts])

  const handleToggleArtifacts = () => {
    const next = !artifactsOpen
    setArtifactsOpen(next)
    if (next) loadArtifacts()
  }

  const formatSize = (bytes: number): string => {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  }

  const formatModTime = (millis: number): string => {
    const d = new Date(millis)
    const now = new Date()
    const sameDay = d.toDateString() === now.toDateString()
    const hm = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
    return sameDay ? hm : `${String(d.getDate()).padStart(2, '0')}/${String(d.getMonth() + 1).padStart(2, '0')} ${hm}`
  }

  const artifactIcon = (name: string): string => {
    const ext = name.split('.').pop()?.toLowerCase() || ''
    if (['png', 'jpg', 'jpeg', 'webp', 'gif', 'bmp', 'svg'].includes(ext)) return '🖼️'
    if (ext === 'pdf') return '📕'
    if (['xls', 'xlsx', 'csv', 'ods'].includes(ext)) return '📊'
    if (['doc', 'docx', 'odt', 'rtf'].includes(ext)) return '📝'
    if (['mp4', 'avi', 'mkv', 'mov', 'webm'].includes(ext)) return '🎬'
    if (['mp3', 'wav', 'ogg', 'flac'].includes(ext)) return '🎵'
    if (['zip', '7z', 'tar', 'gz', 'rar'].includes(ext)) return '🗜️'
    if (['json', 'xml', 'yaml', 'yml', 'toml'].includes(ext)) return '⚙️'
    if (['py', 'js', 'ts', 'go', 'rs', 'java', 'c', 'cpp', 'sh', 'html', 'css', 'sql'].includes(ext)) return '💻'
    if (['md', 'txt', 'log'].includes(ext)) return '📄'
    return '📄'
  }

  const handleOpenArtifact = (path: string) => {
    void openExternalURL(path)
  }
  const scrollRef = useRef<HTMLDivElement>(null)

  // Scroll automático para a última mensagem
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [messages, streamingText, streamingThought, activeToolCalls, pendingPermission, activePlan])

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        flex: 1,
        height: '100%',
        position: 'relative',
        overflow: 'hidden'
      }}
    >
      {/* Pill de Artefatos (canto superior direito) */}
      {artifacts.length > 0 && (
        <div
          style={{
            position: 'absolute',
            top: 12,
            right: 20,
            zIndex: 20,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'flex-end'
          }}
        >
          <button
            type="button"
            onClick={handleToggleArtifacts}
            title="Arquivos criados/modificados nesta conversa"
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 6,
              padding: '5px 12px',
              borderRadius: 14,
              backgroundColor: 'var(--bg-pill)',
              border: '1px solid var(--border-card)',
              fontSize: 12,
              fontWeight: 500,
              color: 'var(--text-secondary)',
              cursor: 'pointer'
            }}
          >
            <span>📁 {artifacts.length} artefato{artifacts.length === 1 ? '' : 's'}</span>
            <span style={{ color: 'var(--text-muted)', fontSize: 10 }}>{artifactsOpen ? '▲' : '▼'}</span>
          </button>

          {artifactsOpen && (
            <div
              style={{
                marginTop: 6,
                width: 380,
                maxWidth: '85vw',
                borderRadius: 12,
                backgroundColor: 'var(--bg-modal)',
                border: '1px solid var(--border-card)',
                boxShadow: '0 10px 30px rgba(0,0,0,0.45)',
                overflow: 'hidden'
              }}
            >
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '8px 12px',
                  borderBottom: '1px solid var(--border-subtle)'
                }}
              >
                <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-secondary)' }}>
                  Artefatos da conversa
                </span>
                <button
                  type="button"
                  onClick={loadArtifacts}
                  title="Atualizar lista"
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: 4,
                    padding: '3px 8px',
                    borderRadius: 10,
                    backgroundColor: 'transparent',
                    border: '1px solid var(--border-card)',
                    fontSize: 11,
                    color: 'var(--text-muted)',
                    cursor: loadingArtifacts ? 'wait' : 'pointer'
                  }}
                >
                  <RefreshIcon size={12} className={loadingArtifacts ? 'animate-spin' : ''} />
                  Atualizar
                </button>
              </div>

              <div style={{ maxHeight: 300, overflowY: 'auto' }}>
                {artifacts.map((a) => (
                  <button
                    key={a.path}
                    type="button"
                    onClick={() => handleOpenArtifact(a.path)}
                    title={`${a.path}
Clique para abrir no app padrão`}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 8,
                      width: '100%',
                      padding: '7px 12px',
                      border: 'none',
                      borderBottom: '1px solid var(--border-subtle)',
                      backgroundColor: 'transparent',
                      color: 'var(--text-primary)',
                      fontSize: 12,
                      cursor: 'pointer',
                      textAlign: 'left'
                    }}
                  >
                    <span style={{ flexShrink: 0 }}>{artifactIcon(a.name)}</span>
                    <span style={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {a.name}
                    </span>
                    <span style={{ color: 'var(--text-muted)', flexShrink: 0, fontSize: 11 }}>{formatSize(a.size)}</span>
                    <span style={{ color: 'var(--text-muted)', flexShrink: 0, fontSize: 11 }}>{formatModTime(a.modified)}</span>
                  </button>
                ))}
              </div>

              <div style={{ padding: '6px 12px', fontSize: 11, color: 'var(--text-muted)' }}>
                Clique em um arquivo para abrir no app padrão do sistema
              </div>
            </div>
          )}
        </div>
      )}

      {/* Área de rolagem das mensagens */}
      <div
        ref={scrollRef}
        style={{
          flex: 1,
          overflowY: 'auto',
          padding: '24px 32px 140px 32px',
          display: 'flex',
          flexDirection: 'column',
          maxWidth: 880,
          width: '100%',
          margin: '0 auto'
        }}
      >
        {/* TodoPanel minimalista fixo no topo da área de mensagens */}
        {activePlan && <TodoPanel plan={activePlan} isStreaming={isStreaming} />}

        {messages.map((msg, idx) => (
          <MessageBubble
            key={msg.id || `${msg.role}-${msg.seq}`}
            message={msg}
            onRespondPermission={onRespondPermission}
            onRegenerate={!isStreaming && onRegenerate ? onRegenerate : undefined}
            onEdit={!isStreaming && onEditMessage ? onEditMessage : undefined}
            showRegenerate={idx === lastAssistantOrErrIdx}
            showEdit={idx === lastUserIdx}
            conversationTitle={conversationTitle}
          />
        ))}

        {/* Indicador de trabalho: enquanto o agente não produziu nada visível */}
        {isStreaming && !streamingText && !streamingThought && activeToolCalls.length === 0 && !activePlan && (
          <WorkingIndicator />
        )}

        {/* Blocos em tempo real durante streaming */}
        {streamingThought && (
          <ThinkingBlock thinking={streamingThought} isStreaming={isStreaming} />
        )}

        {activeToolCalls.map((tc) => (
          <ToolCallCard key={tc.id} toolCall={tc} />
        ))}

        {streamingText && (
          <div
            style={{
              display: 'flex',
              alignItems: 'flex-start',
              gap: 16,
              margin: '20px 0',
              width: '100%'
            }}
          >
            <div
              style={{
                width: 32,
                height: 32,
                borderRadius: '50%',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                flexShrink: 0,
                marginTop: 2
              }}
            >
              <GeminiSparkle size={24} className={isStreaming ? 'pulsing' : ''} />
            </div>
            <div style={{ flex: 1, minWidth: 0 }}>
              <MarkdownRenderer content={streamingText} />
            </div>
          </div>
        )}
      </div>

      {/* Barra de Prompt fixa no rodapé com gradiente suave */}
      <div
        style={{
          position: 'absolute',
          bottom: 0,
          left: 0,
          right: 0,
          padding: '16px 24px 24px 24px',
          background: 'linear-gradient(to top, var(--bg-app) 70%, transparent 100%)',
          display: 'flex',
          justifyContent: 'center',
          pointerEvents: 'none',
          zIndex: 10
        }}
      >
        <div style={{ width: '100%', maxWidth: 840, pointerEvents: 'auto' }}>
          <PromptBar
            prompt={prompt}
            onChangePrompt={onChangePrompt}
            onSend={onSend}
            onCancel={onCancel}
            isStreaming={isStreaming}
            models={models}
            selectedModelId={selectedModelId}
            onSelectModel={onSelectModel}
            modes={modes}
            selectedModeId={selectedModeId}
            onSelectMode={onSelectMode}
            files={files}
            onAddFiles={onAddFiles}
            onRemoveFile={onRemoveFile}
            cwd={cwd}
          />
        </div>
      </div>
    </div>
  )
}
