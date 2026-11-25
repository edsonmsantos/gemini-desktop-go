import React from 'react'
import { CloseIcon } from './icons'

interface SettingsModalProps {
  isOpen: boolean
  onClose: () => void
  port: number
  agentCmd: string
  theme: 'dark' | 'light'
  onToggleTheme: () => void
}

export const SettingsModal: React.FC<SettingsModalProps> = ({
  isOpen,
  onClose,
  port,
  agentCmd,
  theme,
  onToggleTheme
}) => {
  if (!isOpen) return null

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0, 0, 0, 0.65)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 9999,
        backdropFilter: 'blur(3px)'
      }}
      onClick={onClose}
    >
      <div
        style={{
          width: 480,
          maxWidth: '90vw',
          backgroundColor: 'var(--bg-modal)',
          borderRadius: 16,
          border: '1px solid var(--border-card)',
          boxShadow: '0 16px 40px rgba(0,0,0,0.5)',
          padding: 24,
          display: 'flex',
          flexDirection: 'column',
          gap: 20
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <h2 style={{ fontSize: 18, fontWeight: 500, color: 'var(--text-primary)' }}>Configurações</h2>
          <button
            onClick={onClose}
            style={{
              padding: 6,
              borderRadius: '50%',
              color: 'var(--text-secondary)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center'
            }}
          >
            <CloseIcon size={18} />
          </button>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <div>
            <label style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
              Agente ACP Configurado
            </label>
            <div
              style={{
                marginTop: 6,
                padding: '10px 12px',
                borderRadius: 8,
                backgroundColor: 'var(--bg-sidebar)',
                border: '1px solid var(--border-subtle)',
                fontFamily: 'var(--font-mono)',
                fontSize: 13,
                color: 'var(--text-primary)'
              }}
            >
              {agentCmd || 'antigravity-cli (padrão)'}
            </div>
          </div>

          <div>
            <label style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
              Porta do Servidor Local
            </label>
            <div
              style={{
                marginTop: 6,
                padding: '10px 12px',
                borderRadius: 8,
                backgroundColor: 'var(--bg-sidebar)',
                border: '1px solid var(--border-subtle)',
                fontFamily: 'var(--font-mono)',
                fontSize: 13,
                color: 'var(--text-primary)'
              }}
            >
              127.0.0.1:{port || 8399}
            </div>
          </div>

          <div>
            <label style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
              Tema da Interface
            </label>
            <div style={{ marginTop: 8, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <span style={{ fontSize: 14, color: 'var(--text-primary)' }}>
                Modo {theme === 'dark' ? 'Escuro' : 'Claro'}
              </span>
              <button
                onClick={onToggleTheme}
                style={{
                  padding: '8px 16px',
                  borderRadius: 20,
                  backgroundColor: 'var(--bg-pill)',
                  border: '1px solid var(--border-card)',
                  color: 'var(--text-primary)',
                  fontSize: 13,
                  fontWeight: 500
                }}
              >
                Alternar para {theme === 'dark' ? 'Claro' : 'Escuro'}
              </button>
            </div>
          </div>
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 10 }}>
          <button
            onClick={onClose}
            style={{
              padding: '8px 20px',
              borderRadius: 20,
              backgroundColor: 'var(--accent-blue)',
              color: '#ffffff',
              fontSize: 14,
              fontWeight: 500
            }}
          >
            Concluir
          </button>
        </div>
      </div>
    </div>
  )
}
