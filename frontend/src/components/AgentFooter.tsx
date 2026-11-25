import React from 'react'
import { SettingsIcon } from './icons'
import { ThemeToggle } from './ThemeToggle'
import { AgentInfo } from '../types'

interface AgentFooterProps {
  agentInfo: AgentInfo | null
  theme: 'dark' | 'light'
  onToggleTheme: () => void
  onOpenSettings: () => void
}

export const AgentFooter: React.FC<AgentFooterProps> = ({
  agentInfo,
  theme,
  onToggleTheme,
  onOpenSettings
}) => {
  const email = agentInfo?.email

  // Se email != null, extrai primeira letra em maiúsculo
  const initial = email ? email.charAt(0).toUpperCase() : null
  const displayName = email ? (email.includes('@') ? email.split('@')[0] : email) : null

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '12px 14px',
        borderTop: '1px solid var(--border-subtle)',
        marginTop: 'auto',
        gap: 8
      }}
    >
      {/* Bloco condicional de usuário: só renderiza se email != null */}
      {email && (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 10,
            overflow: 'hidden',
            flex: 1,
            cursor: 'default'
          }}
          title={email}
        >
          <div
            style={{
              width: 32,
              height: 32,
              borderRadius: '50%',
              backgroundColor: '#1a73e8',
              color: '#ffffff',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: 14,
              fontWeight: 600,
              flexShrink: 0,
              boxShadow: '0 1px 3px rgba(0,0,0,0.2)'
            }}
          >
            {initial}
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
            <span
              style={{
                fontSize: 13,
                fontWeight: 500,
                color: 'var(--text-primary)',
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis'
              }}
            >
              {displayName}
            </span>
            <span
              style={{
                fontSize: 11,
                color: 'var(--text-muted)',
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis'
              }}
            >
              {email}
            </span>
          </div>
        </div>
      )}

      {/* Botões de Ação do Rodapé: Tema + Configurações */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 4, marginLeft: email ? 0 : 'auto' }}>
        <ThemeToggle theme={theme} onToggle={onToggleTheme} />
        <button
          onClick={onOpenSettings}
          title="Configurações"
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: 34,
            height: 34,
            borderRadius: '50%',
            color: 'var(--text-secondary)',
            transition: 'background 0.2s, color 0.2s'
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.backgroundColor = 'var(--bg-sidebar-hover)'
            e.currentTarget.style.color = 'var(--text-primary)'
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.backgroundColor = 'transparent'
            e.currentTarget.style.color = 'var(--text-secondary)'
          }}
        >
          <SettingsIcon size={18} />
        </button>
      </div>
    </div>
  )
}
