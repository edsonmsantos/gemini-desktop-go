import React, { useState } from 'react'
import {
  ToolIcon,
  GlobeIcon,
  TerminalIcon,
  SearchIcon,
  ZapIcon,
  ExternalLinkIcon,
  ChevronDownIcon,
  ChevronRightIcon
} from './icons'
import { ToolCall } from '../types'
import { DiffViewer } from './DiffViewer'

interface ToolCallCardProps {
  toolCall: ToolCall
}

export const ToolCallCard: React.FC<ToolCallCardProps> = ({ toolCall }) => {
  const [isExpanded, setIsExpanded] = useState(false)

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
  const kind = (toolCall.kind || '').toLowerCase()

  // Corpos volumosos (comando/saída, diff) ficam FECHADOS por padrão —
  // só aparecem quando o usuário clica no cabeçalho (igual ao thinking)
  const isCollapsible = kind === 'execute' || kind === 'command' || kind === 'edit' 
  const rawInput = toolCall.rawInput || {}
  const rawOutput = toolCall.rawOutput || (rawInput.output as string | undefined)

  // Extrações específicas por kind
  const command = (rawInput.command || rawInput.cmd) as string | undefined
  const url = (rawInput.url || rawInput.uri) as string | undefined
  const query = (rawInput.query || rawInput.q || rawInput.path || rawInput.filePath) as string | undefined
  const oldString = typeof rawInput.old_string === 'string' ? rawInput.old_string : undefined
  const newString = typeof rawInput.new_string === 'string' ? rawInput.new_string : undefined
  const content = typeof rawInput.content === 'string' ? rawInput.content : undefined
  const filePath = (rawInput.path || rawInput.filePath || rawInput.file) as string | undefined

  // Registry de ícone por kind
  const renderKindIcon = () => {
    switch (kind) {
      case 'execute':
      case 'command':
        return <TerminalIcon size={16} />
      case 'edit':
        return <ToolIcon size={16} />
      case 'fetch':
      case 'web':
        return <GlobeIcon size={16} />
      case 'read':
      case 'search':
        return <SearchIcon size={16} />
      case 'skill':
        return <ZapIcon size={16} color="var(--accent-blue)" />
      default:
        return <ToolIcon size={16} />
    }
  }

  // Renderizador específico por kind
  const renderKindBody = () => {
    switch (kind) {
      case 'edit': {
        return (
          <div style={{ marginTop: 6 }}>
            <DiffViewer
              oldString={oldString || ''}
              newString={newString || content || ''}
              filePath={filePath}
              maxHeight={200}
            />
          </div>
        )
      }

      case 'execute':
      case 'command': {
        return (
          <div
            className="selectable"
            style={{
              marginTop: 6,
              borderRadius: 8,
              backgroundColor: 'var(--bg-code)',
              border: '1px solid var(--border-subtle)',
              fontFamily: 'var(--font-mono)',
              fontSize: 12,
              color: 'var(--text-primary)',
              overflow: 'hidden'
            }}
          >
            <div
              style={{
                padding: '8px 12px',
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                overflowX: 'auto',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-all'
              }}
            >
              <span style={{ color: 'var(--accent-blue)', fontWeight: 600 }}>$</span>
              <span>{command || toolCall.title}</span>
            </div>

            {toolCall.status === 'completed' && rawOutput && (
              <div
                style={{
                  maxHeight: 160,
                  overflowY: 'auto',
                  padding: '8px 12px',
                  backgroundColor: 'rgba(0, 0, 0, 0.2)',
                  borderTop: '1px solid var(--border-subtle)',
                  color: 'var(--text-secondary)',
                  whiteSpace: 'pre-wrap',
                  lineHeight: 1.4
                }}
              >
                {rawOutput}
              </div>
            )}
          </div>
        )
      }

      case 'fetch':
      case 'web': {
        const targetUrl = url || toolCall.title
        return (
          <div
            style={{
              marginTop: 6,
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              padding: '6px 10px',
              borderRadius: 6,
              backgroundColor: 'var(--bg-pill)',
              fontSize: 12
            }}
          >
            <GlobeIcon size={14} color="var(--accent-blue)" />
            {targetUrl.startsWith('http') ? (
              <a
                href={targetUrl}
                target="_blank"
                rel="noopener noreferrer"
                style={{
                  color: 'var(--accent-blue)',
                  textDecoration: 'none',
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 4,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap'
                }}
              >
                <span>{targetUrl}</span>
                <ExternalLinkIcon size={12} />
              </a>
            ) : (
              <span style={{ color: 'var(--text-primary)' }}>{targetUrl}</span>
            )}
          </div>
        )
      }

      case 'read':
      case 'search': {
        return (
          <div
            style={{
              marginTop: 6,
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              padding: '6px 10px',
              borderRadius: 6,
              backgroundColor: 'var(--bg-pill)',
              fontSize: 12,
              fontFamily: 'var(--font-mono)',
              color: 'var(--text-secondary)',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap'
            }}
            title={query}
          >
            <SearchIcon size={14} />
            <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {query || toolCall.title}
            </span>
          </div>
        )
      }

      case 'skill': {
        const skillSlug = (rawInput.slug || rawInput.skill || toolCall.title) as string
        return (
          <div
            style={{
              marginTop: 6,
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              padding: '6px 10px',
              borderRadius: 6,
              backgroundColor: 'var(--bg-pill)',
              fontSize: 12
            }}
          >
            <ZapIcon size={14} color="var(--accent-blue)" />
            <span style={{ fontWeight: 600, color: 'var(--text-primary)' }}>
              /{skillSlug}
            </span>
            {Boolean(rawInput.args) && (
              <span style={{ color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
                {JSON.stringify(rawInput.args)}
              </span>
            )}
          </div>
        )
      }

      default: {
        // Fallback para outros tipos de ferramentas
        const hasParams = Object.keys(rawInput).length > 0
        if (!hasParams) return null

        return (
          <div style={{ marginTop: 6, fontSize: 12 }}>
            <button
              type="button"
              onClick={() => setIsExpanded((prev) => !prev)}
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: 4,
                cursor: 'pointer',
                color: 'var(--text-muted)',
                userSelect: 'none',
                padding: '2px 0',
                border: 'none',
                background: 'none'
              }}
            >
              {isExpanded ? <ChevronDownIcon size={14} /> : <ChevronRightIcon size={14} />}
              <span>{isExpanded ? 'Ocultar parâmetros' : 'Ver parâmetros de entrada'}</span>
            </button>
            {isExpanded && (
              <pre
                style={{
                  marginTop: 6,
                  padding: '8px 10px',
                  borderRadius: 6,
                  backgroundColor: 'var(--bg-code)',
                  color: 'var(--text-secondary)',
                  fontFamily: 'var(--font-mono)',
                  fontSize: 11,
                  overflowX: 'auto',
                  whiteSpace: 'pre-wrap'
                }}
              >
                {JSON.stringify(rawInput, null, 2)}
              </pre>
            )}
          </div>
        )
      }
    }
  }

  return (
    <div
      style={{
        margin: '10px 0',
        borderRadius: 10,
        border: '1px solid var(--border-card)',
        backgroundColor: 'var(--bg-card)',
        padding: '10px 14px',
        display: 'flex',
        flexDirection: 'column',
        gap: 6
      }}
    >
      {/* Cabeçalho do Card (clicável quando o corpo é colapsável) */}
      <button
        type="button"
        onClick={isCollapsible ? () => setIsExpanded((prev) => !prev) : undefined}
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 10,
          width: '100%',
          padding: 0,
          border: 'none',
          background: 'none',
          cursor: isCollapsible ? 'pointer' : 'default',
          textAlign: 'left'
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0, flex: 1 }}>
          {isCollapsible && (
            <span
              style={{
                transform: isExpanded ? 'rotate(90deg)' : 'none',
                transition: 'transform 0.15s',
                display: 'flex',
                alignItems: 'center',
                color: 'var(--text-muted)',
                flexShrink: 0
              }}
            >
              <ChevronRightIcon size={14} />
            </span>
          )}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: 28,
              height: 28,
              borderRadius: 6,
              backgroundColor: 'var(--bg-pill)',
              color: 'var(--text-secondary)',
              flexShrink: 0
            }}
          >
            {renderKindIcon()}
          </div>
          <span
            style={{
              fontSize: 13,
              fontWeight: 500,
              color: 'var(--text-primary)',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap'
            }}
            title={toolCall.title}
          >
            {toolCall.title || 'Chamada de Ferramenta'}
          </span>
          {kind && (
            <span
              style={{
                fontSize: 10,
                textTransform: 'uppercase',
                fontWeight: 600,
                padding: '1px 6px',
                borderRadius: 4,
                backgroundColor: 'var(--bg-pill)',
                color: 'var(--text-muted)',
                flexShrink: 0
              }}
            >
              {kind}
            </span>
          )}
        </div>

        {/* Status Indicador */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexShrink: 0 }}>
          <span
            className={isPulsing ? 'pulsing' : ''}
            style={{
              width: 8,
              height: 8,
              borderRadius: '50%',
              backgroundColor: statusColor,
              display: 'inline-block'
            }}
          />
          <span style={{ fontSize: 12, fontWeight: 500, color: statusColor }}>
            {getStatusLabel(toolCall.status)}
          </span>
        </div>
      </button>

      {/* Corpo especializado por kind — colapsado por padrão em execute/edit */}
      {(!isCollapsible || isExpanded) && renderKindBody()}
    </div>
  )
}
