import React from 'react'
import { ShieldIcon } from './icons'
import { PermissionRequest } from '../types'

interface PermissionCardProps {
  request: PermissionRequest
  onRespond: (callId: string, allow: boolean) => void
  disabled?: boolean
}

export const PermissionCard: React.FC<PermissionCardProps> = ({
  request,
  onRespond,
  disabled
}) => {
  return (
    <div
      style={{
        margin: '12px 0',
        borderRadius: 12,
        border: '1px solid var(--accent-blue)',
        backgroundColor: 'var(--bg-card)',
        padding: '14px 16px',
        display: 'flex',
        flexDirection: 'column',
        gap: 12,
        boxShadow: '0 4px 16px rgba(66, 133, 244, 0.15)'
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: 32,
            height: 32,
            borderRadius: 8,
            backgroundColor: 'rgba(66, 133, 244, 0.12)',
            color: 'var(--accent-blue)'
          }}
        >
          <ShieldIcon size={18} />
        </div>
        <div style={{ display: 'flex', flexDirection: 'column' }}>
          <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--text-primary)' }}>
            Permissão Requerida
          </span>
          <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>
            O agente solicitou autorização para executar a seguinte ação:
          </span>
        </div>
      </div>

      <div
        style={{
          padding: '10px 12px',
          borderRadius: 8,
          backgroundColor: 'var(--bg-code)',
          border: '1px solid var(--border-subtle)',
          fontFamily: 'var(--font-mono)',
          fontSize: 12,
          color: 'var(--text-primary)',
          overflowX: 'auto'
        }}
      >
        <div style={{ fontWeight: 600, marginBottom: 4 }}>
          {request.toolCall.title || 'Ação do Sistema'}
        </div>
        {request.toolCall.rawInput && (
          <pre style={{ margin: 0 }}>
            {JSON.stringify(request.toolCall.rawInput, null, 2)}
          </pre>
        )}
      </div>

      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 10 }}>
        <button
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
            transition: 'background 0.15s'
          }}
        >
          Negar
        </button>
        <button
          onClick={() => onRespond(request.callId, true)}
          disabled={disabled}
          style={{
            padding: '8px 20px',
            borderRadius: 18,
            backgroundColor: 'var(--accent-blue)',
            color: '#ffffff',
            fontSize: 13,
            fontWeight: 500,
            boxShadow: '0 2px 6px rgba(66, 133, 244, 0.3)'
          }}
        >
          Permitir
        </button>
      </div>
    </div>
  )
}
