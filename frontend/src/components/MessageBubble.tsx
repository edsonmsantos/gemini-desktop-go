import React, { useState } from 'react'
import { GeminiSparkle, FileIcon } from './icons'
import { MarkdownRenderer } from './MarkdownRenderer'
import { ThinkingBlock } from './ThinkingBlock'
import { ToolCallCard } from './ToolCallCard'
import { PermissionCard } from './PermissionCard'
import { PlanCard } from './PlanCard'
import { Message, PermissionRequest, Plan, ToolCall } from '../types'
import { exportResponsePdf, openExternalURL } from '../api'

interface SourceLink {
  text: string
  url: string
}

// extractSources coleta links da resposta: markdown [texto](url) e URLs soltas
function extractSources(content: string): SourceLink[] {
  const seen = new Set<string>()
  const out: SourceLink[] = []
  const add = (text: string, url: string) => {
    const clean = url.replace(/[.,;:]+$/, '')
    if (!/^https?:\/\//.test(clean) || seen.has(clean) || out.length >= 20) return
    seen.add(clean)
    const label = text.trim() || clean.replace(/^https?:\/\//, '').split('/')[0]
    out.push({ text: label, url: clean })
  }
  const mdLink = /\[([^\]]{1,150})\]\((https?:\/\/[^)\s]+)[^)]*\)/g
  let m: RegExpExecArray | null
  while ((m = mdLink.exec(content)) !== null) {
    add(m[1], m[2])
  }
  const rest = content.replace(/\[([^\]]{1,150})\]\((https?:\/\/[^)\s]+)[^)]*\)/g, ' ')
  const bare = /https?:\/\/[^\s<>"')\]]+/g
  while ((m = bare.exec(rest)) !== null) {
    add('', m[0])
  }
  return out
}

interface MessageBubbleProps {
  message: Message
  onRespondPermission?: (callId: string, allow: boolean) => void
  onRegenerate?: () => void
  onEdit?: (newText: string) => void
  showRegenerate?: boolean
  showEdit?: boolean
  conversationTitle?: string
}

// friendlyError traduz mensagens técnicas do backend para linguagem comum
function friendlyError(raw: string): { title: string; hint?: string } {
  const m = (raw || '').toLowerCase()
  if (m.includes('encerrou inesperadamente') || m.includes('agent exit')) {
    return {
      title: 'O assistente caiu inesperadamente durante a resposta.',
      hint: 'Clique em Tentar novamente para reenviar sua mensagem.'
    }
  }
  if (m.includes('timeout') || m.includes('context deadline') || m.includes('tempo expirou')) {
    return {
      title: 'A resposta demorou demais e o tempo expirou.',
      hint: 'Tente novamente — se persistir, divida o pedido em partes menores.'
    }
  }
  if (m.includes('connection refused') || m.includes('falha ao inicializar sessão') || m.includes('session not found')) {
    return {
      title: 'Não foi possível falar com o agente de IA.',
      hint: 'Verifique se o agente (agy/gemini) está instalado. Reabra o app para tentar de novo.'
    }
  }
  if (m.includes('autentic') || m.includes('unauthenticated') || m.includes('permissão negada')) {
    return {
      title: 'A sessão do agente precisa de autenticação.',
      hint: 'Reabra o app para autenticar sua conta Google.'
    }
  }
  if (m.includes('imagem inválida') || m.includes('arquivo de imagem inválido')) {
    return {
      title: 'Um dos anexos de imagem é inválido.',
      hint: 'Tente salvar a imagem novamente e anexá-la outra vez.'
    }
  }
  return { title: raw }
}

export const MessageBubble: React.FC<MessageBubbleProps> = ({
  message,
  onRespondPermission,
  onRegenerate,
  onEdit,
  showRegenerate,
  showEdit,
  conversationTitle
}) => {
  const isUser = message.role === 'user'
  const [editing, setEditing] = useState<boolean>(false)
  const [editText, setEditText] = useState<string>(message.content)
  const [copied, setCopied] = useState<boolean>(false)
  const [savingPdf, setSavingPdf] = useState<boolean>(false)
  const [savedPath, setSavedPath] = useState<string | null>(null)
  const [showSources, setShowSources] = useState<boolean>(false)

  const isAssistant = message.role === 'assistant'
  const sources = isAssistant ? extractSources(message.content) : []

  const handleCopy = async () => {
    // Clipboard async pode ficar PENDENTE (permissão do navegador) e nunca
    // resolver — corre com timeout e cai no fallback execCommand.
    const fallbackCopy = () => {
      const ta = document.createElement('textarea')
      ta.value = message.content
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      try {
        document.execCommand('copy')
      } catch {
        // ignore
      }
      ta.remove()
    }
    try {
      await Promise.race([
        navigator.clipboard.writeText(message.content),
        new Promise((_, reject) => setTimeout(() => reject(new Error('clipboard timeout')), 800))
      ])
    } catch {
      fallbackCopy()
    }
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleSavePdf = async () => {
    if (savingPdf) return
    setSavingPdf(true)
    try {
      // Modo desktop: diálogo nativo para o usuário escolher onde salvar
      const saveViaDialog = window.go?.main?.App?.SavePdfDialog
      if (saveViaDialog) {
        const chosen = await saveViaDialog(conversationTitle || 'Resposta do Gemini', message.content)
        if (chosen) {
          setSavedPath(chosen)
          setTimeout(() => setSavedPath(null), 8000)
        }
        return // cancelado pelo usuário: sem feedback
      }
      // Modo web (serve): salva direto em ~/Downloads/Gemini Desktop
      const res = await exportResponsePdf(conversationTitle || 'Resposta do Gemini', message.content)
      if (res) {
        setSavedPath(res.displayPath)
        setTimeout(() => setSavedPath(null), 8000)
      }
    } finally {
      setSavingPdf(false)
    }
  }

  const handleOpenSource = (url: string) => {
    void openExternalURL(url)
  }

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
    if (editing) {
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
              width: '75%',
              padding: 12,
              borderRadius: 20,
              backgroundColor: 'var(--bg-user-bubble)',
              border: '1px solid var(--accent-blue)'
            }}
          >
            <textarea
              value={editText}
              onChange={(e) => setEditText(e.target.value)}
              autoFocus
              rows={Math.min(8, editText.split('\n').length + 1)}
              style={{
                width: '100%',
                background: 'transparent',
                border: 'none',
                outline: 'none',
                resize: 'vertical',
                color: 'var(--text-primary)',
                fontSize: 15,
                lineHeight: 1.5,
                fontFamily: 'inherit'
              }}
            />
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 8 }}>
              <button
                type="button"
                onClick={() => {
                  setEditText(message.content)
                  setEditing(false)
                }}
                style={{
                  padding: '6px 14px',
                  borderRadius: 14,
                  backgroundColor: 'transparent',
                  border: '1px solid var(--border-card)',
                  color: 'var(--text-secondary)',
                  fontSize: 13,
                  cursor: 'pointer'
                }}
              >
                Cancelar
              </button>
              <button
                type="button"
                disabled={!editText.trim()}
                onClick={() => {
                  setEditing(false)
                  onEdit?.(editText.trim())
                }}
                style={{
                  padding: '6px 14px',
                  borderRadius: 14,
                  backgroundColor: 'var(--accent-blue)',
                  border: 'none',
                  color: '#ffffff',
                  fontSize: 13,
                  fontWeight: 500,
                  cursor: editText.trim() ? 'pointer' : 'default',
                  opacity: editText.trim() ? 1 : 0.5
                }}
              >
                Salvar e reenviar
              </button>
            </div>
          </div>
        </div>
      )
    }

    return (
      <div
        className="msg-row"
        style={{
          display: 'flex',
          justifyContent: 'flex-end',
          margin: '16px 0',
          width: '100%'
        }}
      >
        {/* Botão Editar no hover (mensagem do usuário) */}
        {showEdit && (
          <button
            type="button"
            onClick={() => {
              setEditText(message.content)
              setEditing(true)
            }}
            title="Editar e reenviar esta mensagem"
            className="msg-action-btn"
            style={{
              alignSelf: 'flex-start',
              margin: '14px 8px 0 0',
              padding: '4px 10px',
              borderRadius: 12,
              backgroundColor: 'transparent',
              border: '1px solid var(--border-card)',
              color: 'var(--text-muted)',
              fontSize: 12,
              cursor: 'pointer',
              opacity: 0
            }}
          >
            ✏️ Editar
          </button>
        )}
        <div
          className="selectable"
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

  const isErr = message.role === 'error'
  const errInfo = isErr ? friendlyError(message.content) : null

  return (
    <div
      className="msg-row"
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
        {isErr && errInfo ? (
          <div
            className="selectable"
            style={{
              padding: '10px 14px',
              borderRadius: 8,
              backgroundColor: 'rgba(234, 67, 53, 0.1)',
              border: '1px solid var(--status-failed)',
              color: 'var(--status-failed)',
              fontSize: 14,
              lineHeight: 1.5
            }}
          >
            <div style={{ fontWeight: 500 }}>{errInfo.title}</div>
            {errInfo.hint && (
              <div style={{ marginTop: 4, fontSize: 13, color: 'var(--text-secondary)' }}>{errInfo.hint}</div>
            )}
            {onRegenerate && (
              <button
                type="button"
                onClick={onRegenerate}
                style={{
                  marginTop: 8,
                  padding: '5px 14px',
                  borderRadius: 14,
                  backgroundColor: 'var(--status-failed)',
                  border: 'none',
                  color: '#ffffff',
                  fontSize: 13,
                  fontWeight: 500,
                  cursor: 'pointer'
                }}
              >
                🔁 Tentar novamente
              </button>
            )}
          </div>
        ) : (
          <div className="selectable">
            <MarkdownRenderer content={message.content} />
          </div>
        )}

        {/* Ações da resposta: copiar, PDF, fontes e regenerar (última) */}
        {isAssistant && (
          <>
            <div
              className="msg-action-row"
              style={{ display: 'flex', gap: 6, marginTop: 8, flexWrap: 'wrap', opacity: showRegenerate ? 1 : undefined }}
            >
              <ActionButton onClick={handleCopy} label={copied ? '✓ Copiado' : '📋 Copiar'} />
              <ActionButton
                onClick={handleSavePdf}
                label={savingPdf ? '⏳ Gerando…' : savedPath ? '✓ Salvo' : '📄 Salvar PDF'}
                title="Salvar esta resposta como PDF"
                disabled={savingPdf}
              />
              {sources.length > 0 && (
                <ActionButton
                  onClick={() => setShowSources((prev) => !prev)}
                  label={showSources ? '▲ Fontes' : `🔗 Fontes (${sources.length})`}
                />
              )}
              {showRegenerate && onRegenerate && (
                <ActionButton onClick={onRegenerate} label="🔁 Regenerar" title="Descartar esta resposta e gerar outra" />
              )}
            </div>
            {savedPath && (
              <div style={{ marginTop: 6, fontSize: 12, color: 'var(--status-completed)' }}>
                ✓ PDF salvo em {savedPath}
              </div>
            )}
            {showSources && sources.length > 0 && (
              <div
                className="selectable"
                style={{
                  marginTop: 8,
                  padding: '10px 12px',
                  borderRadius: 10,
                  backgroundColor: 'var(--bg-sidebar)',
                  border: '1px solid var(--border-subtle)'
                }}
              >
                <div
                  style={{
                    fontSize: 11,
                    textTransform: 'uppercase',
                    color: 'var(--text-muted)',
                    fontWeight: 600,
                    marginBottom: 6
                  }}
                >
                  Fontes mencionadas na resposta
                </div>
                {sources.map((s, i) => (
                  <div key={s.url} style={{ display: 'flex', gap: 8, padding: '4px 0', fontSize: 13 }}>
                    <span style={{ color: 'var(--text-muted)', flexShrink: 0, minWidth: 16 }}>{i + 1}.</span>
                    <button
                      type="button"
                      onClick={() => handleOpenSource(s.url)}
                      title={s.url}
                      style={{
                        background: 'none',
                        border: 'none',
                        padding: 0,
                        color: 'var(--accent-blue)',
                        cursor: 'pointer',
                        textAlign: 'left',
                        fontSize: 13,
                        lineHeight: 1.4,
                        wordBreak: 'break-word'
                      }}
                    >
                      {s.text} ↗
                    </button>
                  </div>
                ))}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}

const ActionButton: React.FC<{
  onClick: () => void
  label: string
  title?: string
  disabled?: boolean
}> = ({ onClick, label, title, disabled }) => (
  <button
    type="button"
    onClick={onClick}
    title={title}
    disabled={disabled}
    style={{
      padding: '4px 10px',
      borderRadius: 12,
      backgroundColor: 'transparent',
      border: '1px solid var(--border-card)',
      color: 'var(--text-muted)',
      fontSize: 12,
      cursor: disabled ? 'wait' : 'pointer',
      transition: 'color 0.15s, border-color 0.15s'
    }}
  >
    {label}
  </button>
)
