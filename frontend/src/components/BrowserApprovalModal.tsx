import React from 'react'
import { BrowserApproval } from '../types'
import { GlobeIcon, ShieldIcon } from './icons'

interface BrowserApprovalModalProps {
  approval: BrowserApproval | null
  onRespond: (id: string, approve: boolean) => void
  disabled?: boolean
}

export const BrowserApprovalModal: React.FC<BrowserApprovalModalProps> = ({
  approval,
  onRespond,
  disabled = false
}) => {
  if (!approval) return null

  const toolName = approval.tool || approval.name || 'Ação do Navegador'
  const args = approval.arguments || approval.args || {}

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
        {/* Header */}
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
                {toolName}
              </h2>
              <span
                style={{
                  fontSize: 11,
                  textTransform: 'uppercase',
                  fontWeight: 600,
                  padding: '2px 8px',
                  borderRadius: 6,
                  backgroundColor: 'var(--bg-pill)',
                  color: 'var(--accent-blue)',
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 4
                }}
              >
                <GlobeIcon size={12} />
                browser-bridge
              </span>
            </div>
            <p style={{ fontSize: 13, color: 'var(--text-muted)', margin: 0 }}>
              A extensão do navegador solicita autorização para executar esta ação (modo ask):
            </p>
          </div>
        </div>

        {/* Resumo dos Argumentos (JSON pretty, scroll) */}
        <div style={{ flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column', gap: 6 }}>
          <span style={{ fontSize: 11, fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
            Parâmetros da Solicitação
          </span>
          <pre
            style={{
              margin: 0,
              padding: '12px 14px',
              borderRadius: 8,
              backgroundColor: 'var(--bg-code)',
              color: 'var(--text-primary)',
              fontFamily: 'var(--font-mono)',
              fontSize: 12,
              lineHeight: 1.5,
              overflowY: 'auto',
              maxHeight: '40vh',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
              border: '1px solid var(--border-subtle)'
            }}
          >
            {JSON.stringify(args, null, 2)}
          </pre>
        </div>

        {/* Botões de Ação: Permitir e Negar */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 10, paddingTop: 8 }}>
          <button
            type="button"
            onClick={() => onRespond(approval.id, false)}
            disabled={disabled}
            style={{
              padding: '8px 18px',
              borderRadius: 18,
              backgroundColor: 'transparent',
              border: '1px solid var(--border-card)',
              color: 'var(--status-failed)',
              fontSize: 13,
              fontWeight: 500,
              cursor: 'pointer',
              transition: 'background-color 0.15s ease'
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = 'rgba(234, 67, 53, 0.1)'
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = 'transparent'
            }}
          >
            Negar
          </button>
          <button
            type="button"
            onClick={() => onRespond(approval.id, true)}
            disabled={disabled}
            style={{
              padding: '8px 22px',
              borderRadius: 18,
              backgroundColor: 'var(--accent-blue)',
              border: 'none',
              color: '#ffffff',
              fontSize: 13,
              fontWeight: 500,
              cursor: 'pointer',
              boxShadow: '0 2px 8px rgba(66, 133, 244, 0.3)'
            }}
          >
            Permitir
          </button>
        </div>
      </div>
    </div>
  )
}
