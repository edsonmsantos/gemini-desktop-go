import React, { useState } from 'react'
import { ShieldIcon, TerminalIcon } from './icons'
import { PermissionRequest } from '../types'
import { DiffViewer } from './DiffViewer'

interface PermissionDialogProps {
  request: PermissionRequest | null
  onRespond: (callId: string, allow: boolean) => void
  onRejectWithFeedback?: (callId: string, feedback: string) => void
  disabled?: boolean
}

export const PermissionDialog: React.FC<PermissionDialogProps> = ({
  request,
  onRespond,
  onRejectWithFeedback,
  disabled
}) => {
  const [isFeedbackOpen, setIsFeedbackOpen] = useState(false)
  const [feedbackText, setFeedbackText] = useState('')

  if (!request) return null

  const { toolCall, options } = request
  const rawInput = toolCall.rawInput || {}
  const kind = (toolCall.kind || 'tool').toLowerCase()

  // Extração de dados para o preview inteligente
  const oldString = typeof rawInput.old_string === 'string' ? rawInput.old_string : undefined
  const newString = typeof rawInput.new_string === 'string' ? rawInput.new_string : undefined
  const content = typeof rawInput.content === 'string' ? rawInput.content : undefined
  const filePath = (rawInput.path || rawInput.filePath || rawInput.file) as string | undefined
  const command = (rawInput.command || rawInput.cmd) as string | undefined

  const hasDiff = oldString !== undefined || newString !== undefined || (kind === 'edit' && content !== undefined)
  const hasCommand = command !== undefined || kind === 'execute' || kind === 'command'

  const handleConfirmFeedback = () => {
    if (onRejectWithFeedback) {
      onRejectWithFeedback(request.callId, feedbackText)
    } else {
      onRespond(request.callId, false)
    }
    setIsFeedbackOpen(false)
    setFeedbackText('')
  }

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0, 0, 0, 0.7)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 9999,
        backdropFilter: 'blur(3px)'
      }}
      // Clique fora NÃO fecha (decisão pendente)
      onClick={(e) => e.stopPropagation()}
    >
      <div
        style={{
          width: 540,
          maxWidth: '92vw',
          maxHeight: '85vh',
          backgroundColor: 'var(--bg-modal)',
          borderRadius: 16,
          border: '1px solid var(--accent-blue)',
          boxShadow: '0 16px 40px rgba(0, 0, 0, 0.6), 0 0 20px rgba(66, 133, 244, 0.2)',
          padding: 24,
          display: 'flex',
          flexDirection: 'column',
          gap: 16,
          overflow: 'hidden'
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header do Dialog */}
        <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12 }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: 36,
              height: 36,
              borderRadius: 10,
              backgroundColor: 'rgba(66, 133, 244, 0.15)',
              color: 'var(--accent-blue)',
              flexShrink: 0
            }}
          >
            <ShieldIcon size={20} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4, flex: 1, minWidth: 0 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
              <h2 style={{ fontSize: 16, fontWeight: 600, color: 'var(--text-primary)', margin: 0 }}>
                {toolCall.title || 'Ação do Sistema'}
              </h2>
              <span
                style={{
                  fontSize: 11,
                  textTransform: 'uppercase',
                  fontWeight: 600,
                  padding: '2px 8px',
                  borderRadius: 6,
                  backgroundColor: 'var(--bg-pill)',
                  color: 'var(--text-muted)'
                }}
              >
                {kind}
              </span>
            </div>
            <p style={{ fontSize: 13, color: 'var(--text-muted)', margin: 0 }}>
              O agente solicitou autorização para executar esta ação:
            </p>
          </div>
        </div>

        {/* Preview Embutido Inteligente */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6, minHeight: 0 }}>
          {hasDiff ? (
            <DiffViewer
              oldString={oldString || ''}
              newString={newString || content || ''}
              filePath={filePath}
              maxHeight={260}
            />
          ) : hasCommand ? (
            <div
              style={{
                borderRadius: 8,
                backgroundColor: 'var(--bg-code)',
                border: '1px solid var(--border-subtle)',
                padding: '12px 14px',
                fontFamily: 'var(--font-mono)',
                fontSize: 13,
                color: 'var(--text-primary)',
                overflowX: 'auto',
                display: 'flex',
                alignItems: 'center',
                gap: 8
              }}
            >
              <TerminalIcon size={16} color="var(--accent-blue)" />
              <span style={{ color: 'var(--accent-blue)', fontWeight: 600 }}>$</span>
              <span style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                {command || JSON.stringify(rawInput)}
              </span>
            </div>
          ) : (
            <div
              style={{
                borderRadius: 8,
                backgroundColor: 'var(--bg-code)',
                border: '1px solid var(--border-subtle)',
                padding: '10px 12px',
                fontFamily: 'var(--font-mono)',
                fontSize: 12,
                color: 'var(--text-primary)',
                maxHeight: 220,
                overflowY: 'auto'
              }}
            >
              <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>
                {JSON.stringify(rawInput, null, 2)}
              </pre>
            </div>
          )}
        </div>

        {/* Textarea para "Negar e responder com texto" */}
        {isFeedbackOpen && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginTop: 4 }}>
            <label style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-muted)' }}>
              Orientações ou motivo da recusa para o agente:
            </label>
            <textarea
              autoFocus
              value={feedbackText}
              onChange={(e) => setFeedbackText(e.target.value)}
              placeholder="Explique o que o agente deve corrigir ou fazer em vez disso..."
              rows={3}
              style={{
                width: '100%',
                padding: '8px 12px',
                borderRadius: 8,
                backgroundColor: 'var(--bg-sidebar)',
                border: '1px solid var(--border-card)',
                color: 'var(--text-primary)',
                fontSize: 13,
                resize: 'none',
                fontFamily: 'inherit'
              }}
            />
          </div>
        )}

        {/* Rodapé de Ações */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 10,
            marginTop: 6
          }}
        >
          {isFeedbackOpen ? (
            <>
              <button
                type="button"
                onClick={() => setIsFeedbackOpen(false)}
                disabled={disabled}
                style={{
                  padding: '8px 16px',
                  borderRadius: 18,
                  backgroundColor: 'transparent',
                  border: '1px solid var(--border-card)',
                  color: 'var(--text-muted)',
                  fontSize: 13,
                  cursor: 'pointer'
                }}
              >
                Voltar
              </button>
              <button
                type="button"
                onClick={handleConfirmFeedback}
                disabled={disabled || !feedbackText.trim()}
                style={{
                  padding: '8px 20px',
                  borderRadius: 18,
                  backgroundColor: 'var(--status-failed)',
                  color: '#ffffff',
                  fontSize: 13,
                  fontWeight: 500,
                  border: 'none',
                  cursor: disabled || !feedbackText.trim() ? 'not-allowed' : 'pointer',
                  opacity: disabled || !feedbackText.trim() ? 0.6 : 1
                }}
              >
                Confirmar recusa com feedback
              </button>
            </>
          ) : (
            <>
              <button
                type="button"
                onClick={() => setIsFeedbackOpen(true)}
                disabled={disabled}
                style={{
                  padding: '8px 14px',
                  borderRadius: 18,
                  backgroundColor: 'transparent',
                  border: '1px solid var(--border-card)',
                  color: 'var(--text-secondary)',
                  fontSize: 12,
                  cursor: 'pointer'
                }}
              >
                Negar e responder...
              </button>

              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                {options && options.length > 0 ? (
                  options.map((opt) => {
                    const isAllow =
                      opt.kind.includes('allow') ||
                      opt.name.toLowerCase().includes('permitir') ||
                      opt.name.toLowerCase().includes('allow')

                    return (
                      <button
                        key={opt.optionId}
                        type="button"
                        onClick={() => onRespond(request.callId, isAllow)}
                        disabled={disabled}
                        style={{
                          padding: isAllow ? '8px 20px' : '8px 16px',
                          borderRadius: 18,
                          backgroundColor: isAllow ? 'var(--accent-blue)' : 'transparent',
                          border: isAllow ? 'none' : '1px solid var(--border-card)',
                          color: isAllow ? '#ffffff' : 'var(--status-failed)',
                          fontSize: 13,
                          fontWeight: 500,
                          cursor: 'pointer'
                        }}
                      >
                        {opt.name || (isAllow ? 'Permitir' : 'Negar')}
                      </button>
                    )
                  })
                ) : (
                  <>
                    <button
                      type="button"
                      onClick={() => onRespond(request.callId, false)}
                      disabled={disabled}
                      style={{
                        padding: '8px 16px',
                        borderRadius: 18,
                        backgroundColor: 'transparent',
                        border: '1px solid var(--border-card)',
                        color: 'var(--status-failed)',
                        fontSize: 13,
                        fontWeight: 500,
                        cursor: 'pointer'
                      }}
                    >
                      Negar
                    </button>
                    <button
                      type="button"
                      onClick={() => onRespond(request.callId, true)}
                      disabled={disabled}
                      style={{
                        padding: '8px 20px',
                        borderRadius: 18,
                        backgroundColor: 'var(--accent-blue)',
                        border: 'none',
                        color: '#ffffff',
                        fontSize: 13,
                        fontWeight: 500,
                        cursor: 'pointer'
                      }}
                    >
                      Permitir
                    </button>
                  </>
                )}
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
