import React, { useState, useEffect } from 'react'
import { BrowserStatus } from '../types'
import { fetchBrowserStatus } from '../api'

interface BrowserStatusPillProps {
  onStatusChange?: (status: BrowserStatus) => void
  onClick?: () => void
}

export const BrowserStatusPill: React.FC<BrowserStatusPillProps> = ({ onStatusChange, onClick }) => {
  const [status, setStatus] = useState<BrowserStatus | null>(null)
  const [isHovered, setIsHovered] = useState<boolean>(false)

  useEffect(() => {
    let active = true

    const poll = async () => {
      const res = await fetchBrowserStatus()
      if (!active) return
      setStatus(res)
      onStatusChange?.(res)
    }

    void poll()
    const interval = setInterval(poll, 15000)

    return () => {
      active = false
      clearInterval(interval)
    }
  }, [onStatusChange])

  const isFullyConnected = Boolean(status?.connected && status?.extensionConnected)
  const isPartiallyConnected = Boolean(status?.connected && !status?.extensionConnected)

  let dotColor = 'var(--status-pending)' // cinza/offline
  let badgeText = 'Offline'
  let badgeColor = 'rgba(234, 67, 53, 0.2)'
  let badgeTextColor = '#f28b82'

  if (isFullyConnected) {
    dotColor = 'var(--status-completed)' // verde
    badgeText = 'Conectado'
    badgeColor = 'rgba(52, 168, 83, 0.15)'
    badgeTextColor = 'var(--status-completed)'
  } else if (isPartiallyConnected) {
    dotColor = 'var(--status-progress)' // amarelo / progresso
    badgeText = 'Aguardando'
    badgeColor = 'rgba(249, 171, 0, 0.2)'
    badgeTextColor = 'var(--status-progress)'
  }

  let tooltip = 'Navegador desconectado • Clique para ver instruções de instalação'
  if (isFullyConnected && status) {
    const daemonTag = status.daemon ? status.daemon.toUpperCase() : 'BRIDGE'
    tooltip = `Navegador conectado (${daemonTag}) • Porta ${status.port} • Clique para ver status e opções`
    if (status.mode) {
      tooltip += ` • Modo: ${status.mode}`
    }
    if (status.approvalsPending > 0) {
      tooltip += ` • ${status.approvalsPending} aprovação(ões) pendente(s)`
    }
  } else if (isPartiallyConnected && status) {
    tooltip = `Ponte ativa na porta ${status.port}, aguardando extensão Browser Bridge.\nClique para instruções de instalação no navegador.`
  }

  const handleClick = () => {
    if (onClick) {
      onClick()
    } else {
      window.dispatchEvent(new CustomEvent('open-browser-settings'))
    }
  }

  return (
    <div
      title={tooltip}
      onClick={handleClick}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          handleClick()
        }
      }}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 6,
        padding: '3px 8px',
        borderRadius: 12,
        backgroundColor: isHovered ? 'var(--bg-pill-hover)' : 'var(--bg-pill)',
        fontSize: 11,
        color: isFullyConnected ? 'var(--text-primary)' : 'var(--text-muted)',
        cursor: 'pointer',
        userSelect: 'none',
        transition: 'background-color 0.2s, color 0.2s, border-color 0.2s, transform 0.1s',
        border: isHovered
          ? '1px solid var(--accent-blue)'
          : isFullyConnected
          ? '1px solid var(--border-subtle)'
          : isPartiallyConnected
          ? '1px solid rgba(249, 171, 0, 0.4)'
          : '1px solid rgba(234, 67, 53, 0.3)',
        whiteSpace: 'nowrap',
        transform: isHovered ? 'scale(1.02)' : 'none'
      }}
    >
      <span
        style={{
          width: 7,
          height: 7,
          borderRadius: '50%',
          backgroundColor: dotColor,
          display: 'inline-block',
          boxShadow: isFullyConnected ? `0 0 6px ${dotColor}` : isPartiallyConnected ? `0 0 5px ${dotColor}` : 'none',
          flexShrink: 0
        }}
      />
      <span style={{ fontWeight: 500 }}>Navegador</span>
      {!isFullyConnected && (
        <span
          style={{
            fontSize: 9,
            fontWeight: 600,
            padding: '1px 4px',
            borderRadius: 4,
            backgroundColor: badgeColor,
            color: badgeTextColor,
            textTransform: 'uppercase',
            letterSpacing: 0.3
          }}
        >
          {badgeText}
        </span>
      )}
    </div>
  )
}
