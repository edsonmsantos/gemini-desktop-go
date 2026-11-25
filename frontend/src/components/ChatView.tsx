import React, { useRef, useEffect } from 'react'
import { MessageBubble } from './MessageBubble'
import { PromptBar } from './PromptBar'
import { ThinkingBlock } from './ThinkingBlock'
import { ToolCallCard } from './ToolCallCard'
import { PermissionCard } from './PermissionCard'
import { PlanCard } from './PlanCard'
import { MarkdownRenderer } from './MarkdownRenderer'
import { GeminiSparkle } from './icons'
import { Message, Model, PermissionRequest, Plan, ToolCall } from '../types'

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
  files?: string[]
  onAddFiles?: (files: string[]) => void
  onRemoveFile?: (index: number) => void
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
  files = [],
  onAddFiles,
  onRemoveFile
}) => {
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
        {messages.map((msg) => (
          <MessageBubble
            key={msg.id || `${msg.role}-${msg.seq}`}
            message={msg}
            onRespondPermission={onRespondPermission}
          />
        ))}

        {/* Blocos em tempo real durante streaming */}
        {streamingThought && (
          <ThinkingBlock thinking={streamingThought} isStreaming={isStreaming} />
        )}

        {activePlan && <PlanCard plan={activePlan} />}

        {activeToolCalls.map((tc) => (
          <ToolCallCard key={tc.id} toolCall={tc} />
        ))}

        {pendingPermission && (
          <PermissionCard
            request={pendingPermission}
            onRespond={onRespondPermission}
            disabled={!isStreaming}
          />
        )}

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
            files={files}
            onAddFiles={onAddFiles}
            onRemoveFile={onRemoveFile}
          />
        </div>
      </div>
    </div>
  )
}
