import React, { useState, useEffect } from 'react'
import { BrowserStatus } from '../types'
import { fetchBrowserStatus } from '../api'

interface BrowserStatusPillProps {
  onStatusChange?: (status: BrowserStatus) => void
}

export const BrowserStatusPill: React.FC<BrowserStatusPillProps> = ({ onStatusChange }) => {
  const [status, setStatus] = useState<BrowserStatus | null>(null)

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

  let dotColor = 'var(--status-pending)' // cinza
  if (isFullyConnected) {
    dotColor = 'var(--status-completed)' // verde
  } else if (isPartiallyConnected) {
    dotColor = 'var(--status-progress)' // amarelo / progresso
  }

  let tooltip = 'Navegador desconectado\nAbra o navegador (Brave) — extensão Browser Bridge'
  if (isFullyConnected && status) {
    const daemonTag = status.daemon ? status.daemon.toUpperCase() : 'BRIDGE'
    tooltip = `Navegador conectado (${daemonTag}) • Porta ${status.port}`
    if (status.mode) {
      tooltip += ` • Modo: ${status.mode}`
    }
    if (status.approvalsPending > 0) {
      tooltip += ` • ${status.approvalsPending} aprovação(ões) pendente(s)`
    }
  } else if (isPartiallyConnected && status) {
    tooltip = `Ponte ativa na porta ${status.port}, aguardando extensão Browser Bridge.\nAbra o navegador (Brave) — extensão Browser Bridge.`
  }

  return (
    <div
      title={tooltip}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 6,
        padding: '3px 8px',
        borderRadius: 12,
        backgroundColor: 'var(--bg-pill)',
        fontSize: 11,
        color: isFullyConnected ? 'var(--text-primary)' : 'var(--text-muted)',
        cursor: 'default',
        userSelect: 'none',
        transition: 'background-color 0.2s, color 0.2s',
        border: '1px solid var(--border-subtle)',
        whiteSpace: 'nowrap'
      }}
    >
      <span
        style={{
          width: 7,
          height: 7,
          borderRadius: '50%',
          backgroundColor: dotColor,
          display: 'inline-block',
          boxShadow: isFullyConnected ? `0 0 5px ${dotColor}` : 'none',
          flexShrink: 0
        }}
      />
      <span style={{ fontWeight: 500 }}>Navegador</span>
    </div>
  )
}
