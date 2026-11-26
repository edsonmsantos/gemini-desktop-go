import React, { useState, useEffect } from 'react'
import { ToolCall } from '../types'
import { GlobeIcon, ChevronDownIcon, ChevronRightIcon, SpinnerIcon } from './icons'
import { fetchFileContent } from '../api'
import { ArtifactViewerModal } from './ArtifactViewerModal'

interface BrowserToolCardProps {
  toolCall: ToolCall
}

function humanizeBrowserTitle(toolName: string, args: Record<string, unknown> = {}): string {
  switch (toolName) {
    case 'browser_snapshot':
      return 'Ler página'
    case 'browser_act': {
      const steps = Array.isArray(args.steps) ? args.steps : []
      const count = steps.length
      const firstStep = steps.length > 0 && typeof steps[0] === 'object' && steps[0] !== null ? (steps[0] as Record<string, unknown>) : null
      const firstOp = firstStep ? String(firstStep.action || firstStep.op || '') : ''
      if (firstOp) {
        return `Agir (${count} ${count === 1 ? 'passo' : 'passos'}: ${firstOp})`
      }
      return `Agir (${count} ${count === 1 ? 'passo' : 'passos'})`
    }
    case 'browser_watch':
      return 'Aguardar condição'
    case 'browser_screenshot':
      return 'Capturar tela'
    case 'browser_som_screenshot':
      return 'Capturar tela com marcações'
    case 'browser_extract':
      return 'Extrair dados'
    case 'browser_navigate': {
      const targetUrl = String(args.url || args.target || '')
      if (targetUrl) {
        try {
          const u = new URL(targetUrl)
          const shortUrl = u.hostname + (u.pathname.length > 24 ? u.pathname.slice(0, 24) + '…' : u.pathname)
          return `Abrir ${shortUrl}`
        } catch {
          const short = targetUrl.length > 30 ? targetUrl.slice(0, 30) + '…' : targetUrl
          return `Abrir ${short}`
        }
      }
      return 'Abrir página'
    }
    case 'browser_click': {
      const targetText = String(args.text || args.selector || args.label || '')
      if (targetText && targetText.length <= 25) {
        return `Clicar em "${targetText}"`
      }
      return 'Clicar'
    }
    case 'browser_type': {
      const text = String(args.text || '')
      if (text && text.length <= 20) {
        return `Digitar "${text}"`
      }
      return 'Digitar'
    }
    case 'browser_save':
      return 'Salvar página'
    case 'browser_tab':
      return 'Gerenciar abas'
    case 'browser_cache':
      return 'Consultar cache'
    case 'browser_status':
      return 'Verificar ponte'
    case 'browser_get_content':
      return 'Ler página (legado)'
    case 'browser_get_interactives':
      return 'Interativos da página (legado)'
    case 'browser_get_html':
      return 'HTML da página (legado)'
    default:
      if (toolName.startsWith('browser_')) {
        const clean = toolName.replace(/^browser_/, '').replace(/_/g, ' ')
        return `Navegador: ${clean}`
      }
      return 'Navegador'
  }
}

export const BrowserToolCard: React.FC<BrowserToolCardProps> = ({ toolCall }) => {
  const [isExpanded, setIsExpanded] = useState(false)
  const [imageSrc, setImageSrc] = useState<string | null>(null)
  const [loadingImg, setLoadingImg] = useState(false)
  const [showViewer, setShowViewer] = useState(false)

  const rawInput = (toolCall.rawInput || {}) as Record<string, unknown>
  const toolName = String(rawInput.ToolName || toolCall.title || '')
  const toolArgs = (rawInput.Arguments || {}) as Record<string, unknown>
  const rawOutput = toolCall.rawOutput || ''

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'in_progress':
        return 'var(--status-progress)'
      case 'completed':
        return 'var(--status-completed)'
      case 'failed':
        return 'var(--status-failed)'
      default:
        return 'var(--status-pending)'
    }
  }

  const getStatusLabel = (status: string) => {
    switch (status) {
      case 'in_progress':
        return 'Em execução...'
      case 'completed':
        return 'Concluído'
      case 'failed':
        return 'Falhou'
      default:
        return 'Pendente'
    }
  }

  const statusColor = getStatusColor(toolCall.status)
  const isPulsing = toolCall.status === 'in_progress'
  const title = humanizeBrowserTitle(toolName, toolArgs)

  // Badges
  const isTrusted =
    rawOutput.includes('cdp-trusted') ||
    rawOutput.includes('via:"cdp-trusted"') ||
    Boolean(toolArgs.humanize) ||
    Boolean(rawInput.humanize)

  const hasModal =
    rawOutput.toLowerCase().includes('modal') ||
    rawOutput.toLowerCase().includes('overlay') ||
    Boolean(toolArgs.modal)

  const isFailed = toolCall.status === 'failed'

  // Carrega screenshot se disponível
  useEffect(() => {
    let active = true
    if (!toolCall.screenshot) {
      setImageSrc(null)
      return
    }

    setLoadingImg(true)
    fetchFileContent(toolCall.screenshot)
      .then((fc) => {
        if (!active) return
        if (fc && fc.kind === 'image' && fc.content) {
          if (fc.content.startsWith('data:')) {
            setImageSrc(fc.content)
          } else {
            setImageSrc(`data:${fc.mime || 'image/png'};base64,${fc.content}`)
          }
        }
      })
      .catch(() => {
        // falha silenciosa para screenshot
      })
      .finally(() => {
        if (active) setLoadingImg(false)
      })

    return () => {
      active = false
    }
  }, [toolCall.screenshot])

  const hasArgs = Object.keys(toolArgs).length > 0
  const hasOutput = Boolean(rawOutput && toolCall.status === 'completed')

  return (
    <div
      style={{
        margin: '6px 0',
        borderRadius: 12,
        backgroundColor: 'var(--bg-card)',
        border: '1px solid var(--border-card)',
        overflow: 'hidden',
        boxShadow: '0 1px 3px rgba(0, 0, 0, 0.1)',
        fontSize: 13
      }}
    >
      {/* Cabeçalho */}
      <div
        onClick={() => setIsExpanded((prev) => !prev)}
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '10px 14px',
          cursor: 'pointer',
          userSelect: 'none',
          gap: 10,
          backgroundColor: isExpanded ? 'var(--bg-pill)' : 'transparent',
          transition: 'background-color 0.15s ease'
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0, flex: 1 }}>
          <div
            style={{
              width: 8,
              height: 8,
              borderRadius: '50%',
              backgroundColor: statusColor,
              flexShrink: 0,
              boxShadow: isPulsing ? `0 0 6px ${statusColor}` : 'none'
            }}
          />
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: 20,
              height: 20,
              borderRadius: 4,
              backgroundColor: 'rgba(66, 133, 244, 0.15)',
              color: 'var(--accent-blue)',
              flexShrink: 0
            }}
          >
            <GlobeIcon size={14} />
          </div>
          <span
            style={{
              fontWeight: 500,
              color: 'var(--text-primary)',
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis'
            }}
          >
            {title}
          </span>

          {/* Badges */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexShrink: 0 }}>
            {isTrusted && (
              <span
                style={{
                  fontSize: 10,
                  fontWeight: 600,
                  textTransform: 'uppercase',
                  padding: '1px 6px',
                  borderRadius: 4,
                  backgroundColor: 'rgba(52, 168, 83, 0.15)',
                  color: 'var(--status-completed)',
                  border: '1px solid rgba(52, 168, 83, 0.3)'
                }}
              >
                trusted
              </span>
            )}

            {hasModal && (
              <span
                style={{
                  fontSize: 10,
                  fontWeight: 600,
                  textTransform: 'uppercase',
                  padding: '1px 6px',
                  borderRadius: 4,
                  backgroundColor: 'rgba(249, 171, 0, 0.15)',
                  color: 'var(--status-progress)',
                  border: '1px solid rgba(249, 171, 0, 0.3)'
                }}
              >
                modal
              </span>
            )}

            {isFailed && (
              <span
                style={{
                  fontSize: 10,
                  fontWeight: 600,
                  textTransform: 'uppercase',
                  padding: '1px 6px',
                  borderRadius: 4,
                  backgroundColor: 'rgba(234, 67, 53, 0.15)',
                  color: 'var(--status-failed)',
                  border: '1px solid rgba(234, 67, 53, 0.3)'
                }}
              >
                falhou
              </span>
            )}
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexShrink: 0 }}>
          <span style={{ fontSize: 11, color: statusColor, fontWeight: 500 }}>
            {getStatusLabel(toolCall.status)}
          </span>
          <span style={{ color: 'var(--text-muted)' }}>
            {isExpanded ? <ChevronDownIcon size={16} /> : <ChevronRightIcon size={16} />}
          </span>
        </div>
      </div>

      {/* Screenshot inline (se disponível) */}
      {toolCall.screenshot && (
        <div style={{ padding: '8px 14px 12px 14px', borderTop: '1px solid var(--border-subtle)' }}>
          {loadingImg ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--text-muted)' }}>
              <SpinnerIcon size={14} className="animate-spin" />
              <span>Carregando captura de tela...</span>
            </div>
          ) : imageSrc ? (
            <div style={{ display: 'inline-block', cursor: 'pointer' }} onClick={() => setShowViewer(true)} title="Clique para ampliar">
              <img
                src={imageSrc}
                alt="Captura de tela do navegador"
                style={{
                  maxHeight: 220,
                  maxWidth: '100%',
                  borderRadius: 8,
                  border: '1px solid var(--border-subtle)',
                  boxShadow: '0 2px 8px rgba(0, 0, 0, 0.2)',
                  display: 'block'
                }}
              />
            </div>
          ) : null}
        </div>
      )}

      {/* Conteúdo colapsável (Args e Saída) */}
      {isExpanded && (
        <div style={{ padding: '10px 14px', borderTop: '1px solid var(--border-subtle)', display: 'flex', flexDirection: 'column', gap: 10 }}>
          {hasArgs && (
            <div>
              <div style={{ fontSize: 11, fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase', marginBottom: 4 }}>
                Argumentos
              </div>
              <pre
                style={{
                  margin: 0,
                  padding: '8px 12px',
                  borderRadius: 6,
                  backgroundColor: 'var(--bg-code)',
                  color: 'var(--text-primary)',
                  fontFamily: 'var(--font-mono)',
                  fontSize: 11,
                  lineHeight: 1.4,
                  overflowX: 'auto',
                  maxHeight: 180,
                  overflowY: 'auto'
                }}
              >
                {JSON.stringify(toolArgs, null, 2)}
              </pre>
            </div>
          )}

          {hasOutput && (
            <div>
              <div style={{ fontSize: 11, fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase', marginBottom: 4 }}>
                Saída
              </div>
              <pre
                style={{
                  margin: 0,
                  padding: '8px 12px',
                  borderRadius: 6,
                  backgroundColor: 'var(--bg-code)',
                  color: 'var(--text-secondary)',
                  fontFamily: 'var(--font-mono)',
                  fontSize: 11,
                  lineHeight: 1.4,
                  overflowX: 'auto',
                  maxHeight: 200,
                  overflowY: 'auto',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word'
                }}
              >
                {rawOutput}
              </pre>
            </div>
          )}
        </div>
      )}

      {/* Modal de ampliação do screenshot */}
      {showViewer && toolCall.screenshot && (
        <ArtifactViewerModal
          artifact={{
            path: toolCall.screenshot,
            name: toolCall.screenshot.split('/').pop() || 'screenshot.png',
            size: 0,
            modified: Date.now()
          }}
          onClose={() => setShowViewer(false)}
        />
      )}
    </div>
  )
}
