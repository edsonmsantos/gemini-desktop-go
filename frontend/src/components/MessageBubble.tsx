import React from 'react'
import { GeminiSparkle, FileIcon } from './icons'
import { MarkdownRenderer } from './MarkdownRenderer'
import { ThinkingBlock } from './ThinkingBlock'
import { ToolCallCard } from './ToolCallCard'
import { PermissionCard } from './PermissionCard'
import { PlanCard } from './PlanCard'
import { Message, PermissionRequest, Plan, ToolCall } from '../types'

interface MessageBubbleProps {
  message: Message
  onRespondPermission?: (callId: string, allow: boolean) => void
}

export const MessageBubble: React.FC<MessageBubbleProps> = ({
  message,
  onRespondPermission
}) => {
  const isUser = message.role === 'user'

  // Tratar mensagens com meta JSON
  let toolCallData: ToolCall | null = null
  let permissionData: PermissionRequest | null = null
  let planData: Plan | null = null
  let attachedFiles: string[] = []

  if (message.meta) {
    try {
      const parsed = JSON.parse(message.meta)
      if (message.role === 'tool_call') {
        toolCallData = parsed as ToolCall
      } else if (message.role === 'permission') {
        permissionData = parsed as PermissionRequest
      } else if (message.role === 'plan') {
        planData = parsed as Plan
      } else if (message.role === 'user' && Array.isArray(parsed)) {
        attachedFiles = parsed as string[]
      }
    } catch {
      // ignore
    }
  }

  if (isUser) {
    return (
      <div
        style={{
          display: 'flex',
          justifyContent: 'flex-end',
          margin: '16px 0',
          width: '100%'
        }}
      >
        <div
          style={{
            maxWidth: '75%',
            padding: '12px 18px',
            borderRadius: 20,
            backgroundColor: 'var(--bg-user-bubble)',
            color: 'var(--text-primary)',
            fontSize: 15,
            lineHeight: 1.5,
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
            boxShadow: '0 1px 2px rgba(0,0,0,0.1)'
          }}
        >
          {/* Arquivos anexados exibidos no topo da bolha do usuário */}
          {attachedFiles.length > 0 && (
            <div
              style={{
                display: 'flex',
                flexWrap: 'wrap',
                gap: 6,
                marginBottom: message.content ? 8 : 0
              }}
            >
              {attachedFiles.map((fileStr, i) => {
                const match = fileStr.match(/^(.*?)\s*\((.*?)\)$/)
                const name = match ? match[1] : fileStr.split(/[/\\]/).pop() || fileStr
                const fullPath = match ? match[2] : fileStr
                return (
                  <div
                    key={i}
                    title={fullPath}
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: 5,
                      padding: '3px 8px',
                      borderRadius: 6,
                      backgroundColor: 'rgba(255, 255, 255, 0.12)',
                      fontSize: 12,
                      color: 'inherit'
                    }}
                  >
                    <FileIcon size={12} />
                    <span>{name}</span>
                  </div>
                )
              })}
            </div>
          )}

          {message.content}
        </div>
      </div>
    )
  }

  if (message.role === 'thought') {
    return <ThinkingBlock thinking={message.content} />
  }

  if (message.role === 'tool_call' && toolCallData) {
    return <ToolCallCard toolCall={toolCallData} />
  }

  if (message.role === 'permission' && permissionData) {
    return (
      <PermissionCard
        request={permissionData}
        onRespond={(callId, allow) => onRespondPermission?.(callId, allow)}
      />
    )
  }

  if (message.role === 'plan' && planData) {
    return <PlanCard plan={planData} />
  }

  return (
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
        <GeminiSparkle size={24} />
      </div>

      <div style={{ flex: 1, minWidth: 0 }}>
        {message.role === 'error' ? (
          <div
            style={{
              padding: '10px 14px',
              borderRadius: 8,
              backgroundColor: 'rgba(234, 67, 53, 0.1)',
              border: '1px solid var(--status-failed)',
              color: 'var(--status-failed)',
              fontSize: 14
            }}
          >
            {message.content}
          </div>
        ) : (
          <MarkdownRenderer content={message.content} />
        )}
      </div>
    </div>
  )
}
