import React from 'react'
import { ToolIcon } from './icons'
import { ToolCall } from '../types'

interface ToolCallCardProps {
  toolCall: ToolCall
}

export const ToolCallCard: React.FC<ToolCallCardProps> = ({ toolCall }) => {
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
        gap: 8
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 10 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: 28,
              height: 28,
              borderRadius: 6,
              backgroundColor: 'var(--bg-pill)',
              color: 'var(--text-secondary)'
            }}
          >
            <ToolIcon size={16} />
          </div>
          <span style={{ fontSize: 13, fontWeight: 500, color: 'var(--text-primary)' }}>
            {toolCall.title || 'Chamada de Ferramenta'}
          </span>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
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
      </div>

      {toolCall.rawInput && Object.keys(toolCall.rawInput).length > 0 && (
        <details style={{ marginTop: 2, fontSize: 12 }}>
          <summary style={{ cursor: 'pointer', color: 'var(--text-muted)', userSelect: 'none' }}>
            Ver parâmetros de entrada
          </summary>
          <pre
            style={{
              marginTop: 6,
              padding: '8px 10px',
              borderRadius: 6,
              backgroundColor: 'var(--bg-code)',
              fontFamily: 'var(--font-mono)',
              fontSize: 12,
              overflowX: 'auto',
              color: 'var(--text-secondary)'
            }}
          >
            {JSON.stringify(toolCall.rawInput, null, 2)}
          </pre>
        </details>
      )}
    </div>
  )
}
