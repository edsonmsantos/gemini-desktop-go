import React, { useState, useEffect, useCallback } from 'react'
import { CloseIcon, RefreshIcon, SpinnerIcon } from './icons'
import { MCPServer } from '../types'
import { fetchMCPServers, toggleMCPServer } from '../api'

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
  const [activeTab, setActiveTab] = useState<'general' | 'mcp'>('general')

  // Estado da aba MCP Servers
  const [mcpServers, setMcpServers] = useState<MCPServer[]>([])
  const [mcpAvailable, setMcpAvailable] = useState<boolean>(true)
  const [isLoadingMcp, setIsLoadingMcp] = useState<boolean>(false)
  const [mcpError, setMcpError] = useState<string | null>(null)
  const [toastError, setToastError] = useState<string | null>(null)
  const [togglingServer, setTogglingServer] = useState<string | null>(null)

  const loadMCPServers = useCallback(async () => {
    setIsLoadingMcp(true)
    setMcpError(null)
    setToastError(null)
    try {
      const res = await fetchMCPServers()
      setMcpServers(res.servers)
      setMcpAvailable(res.available)
    } catch (err: unknown) {
      setMcpError(err instanceof Error ? err.message : 'Falha ao consultar MCP')
    } finally {
      setIsLoadingMcp(false)
    }
  }, [])

  useEffect(() => {
    if (isOpen && activeTab === 'mcp') {
      loadMCPServers()
    }
  }, [isOpen, activeTab, loadMCPServers])

  const handleToggle = async (server: MCPServer) => {
    const prevStatus = server.status
    const newEnable = prevStatus !== 'enabled'
    const newStatus = newEnable ? 'enabled' : 'disabled'

    // Atualização otimista na lista
    setMcpServers((prev) =>
      prev.map((s) => (s.name === server.name ? { ...s, status: newStatus } : s))
    )
    setTogglingServer(server.name)
    setToastError(null)

    try {
      const result = await toggleMCPServer(server.name, newEnable)
      setMcpServers((prev) =>
        prev.map((s) =>
          s.name === server.name
            ? { ...s, status: (result.status as 'enabled' | 'disabled' | 'other') || newStatus }
            : s
        )
      )
    } catch (err: unknown) {
      // Rollback em erro
      setMcpServers((prev) =>
        prev.map((s) => (s.name === server.name ? { ...s, status: prevStatus } : s))
      )
      setToastError(err instanceof Error ? err.message : 'Erro ao alterar servidor')
    } finally {
      setTogglingServer(null)
    }
  }

  if (!isOpen) return null

  const activeCount = mcpServers.filter((s) => s.status === 'enabled').length
  const totalCount = mcpServers.length

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
          width: activeTab === 'mcp' ? 580 : 480,
          maxWidth: '92vw',
          maxHeight: '85vh',
          backgroundColor: 'var(--bg-modal)',
          borderRadius: 16,
          border: '1px solid var(--border-card)',
          boxShadow: '0 16px 40px rgba(0,0,0,0.5)',
          padding: 24,
          display: 'flex',
          flexDirection: 'column',
          gap: 16,
          overflow: 'hidden'
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Cabeçalho do Modal */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <h2 style={{ fontSize: 18, fontWeight: 500, color: 'var(--text-primary)' }}>Configurações</h2>
          <button
            type="button"
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

        {/* Abas */}
        <div style={{ display: 'flex', gap: 6, borderBottom: '1px solid var(--border-subtle)', paddingBottom: 6 }}>
          <button
            type="button"
            onClick={() => setActiveTab('general')}
            style={{
              padding: '6px 14px',
              borderRadius: 14,
              fontSize: 13,
              fontWeight: 500,
              backgroundColor: activeTab === 'general' ? 'var(--bg-pill-hover)' : 'transparent',
              color: activeTab === 'general' ? 'var(--text-primary)' : 'var(--text-muted)',
              border: 'none',
              cursor: 'pointer',
              transition: 'background 0.15s, color 0.15s'
            }}
          >
            Geral
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('mcp')}
            style={{
              padding: '6px 14px',
              borderRadius: 14,
              fontSize: 13,
              fontWeight: 500,
              backgroundColor: activeTab === 'mcp' ? 'var(--bg-pill-hover)' : 'transparent',
              color: activeTab === 'mcp' ? 'var(--text-primary)' : 'var(--text-muted)',
              border: 'none',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: 6,
              transition: 'background 0.15s, color 0.15s'
            }}
          >
            <span>MCP Servers</span>
            {mcpServers.length > 0 && (
              <span
                style={{
                  fontSize: 11,
                  padding: '1px 6px',
                  borderRadius: 10,
                  backgroundColor: 'var(--bg-pill)',
                  color: 'var(--text-secondary)'
                }}
              >
                {activeCount}
              </span>
            )}
          </button>
        </div>

        {/* Conteúdo da Aba Geral */}
        {activeTab === 'general' && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14, overflowY: 'auto' }}>
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
                  type="button"
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
        )}

        {/* Conteúdo da Aba MCP Servers */}
        {activeTab === 'mcp' && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12, flex: 1, minHeight: 0 }}>
            {/* Header da seção MCP: contador e botão refresh */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <span style={{ fontSize: 13, fontWeight: 500, color: 'var(--text-secondary)' }}>
                {mcpAvailable ? `${activeCount} ativos / ${totalCount} servidores` : 'Gerenciamento MCP'}
              </span>
              <button
                type="button"
                onClick={loadMCPServers}
                disabled={isLoadingMcp}
                title="Atualizar lista de servidores MCP"
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 6,
                  padding: '5px 10px',
                  borderRadius: 12,
                  backgroundColor: 'var(--bg-pill)',
                  border: '1px solid var(--border-card)',
                  fontSize: 12,
                  color: 'var(--text-secondary)',
                  cursor: isLoadingMcp ? 'wait' : 'pointer'
                }}
              >
                <RefreshIcon size={14} className={isLoadingMcp ? 'animate-spin' : ''} />
                <span>Atualizar</span>
              </button>
            </div>

            {/* Toast inline de erro */}
            {toastError && (
              <div
                style={{
                  padding: '8px 12px',
                  borderRadius: 8,
                  backgroundColor: 'rgba(234, 67, 53, 0.12)',
                  border: '1px solid var(--status-failed)',
                  color: 'var(--status-failed)',
                  fontSize: 12,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: 8
                }}
              >
                <span>{toastError}</span>
                <button
                  type="button"
                  onClick={() => setToastError(null)}
                  style={{ color: 'inherit', cursor: 'pointer', padding: 2 }}
                >
                  <CloseIcon size={13} />
                </button>
              </div>
            )}

            {/* Carregando */}
            {isLoadingMcp && mcpServers.length === 0 && (
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gap: 8,
                  padding: '30px 16px',
                  fontSize: 13,
                  color: 'var(--text-muted)'
                }}
              >
                <SpinnerIcon size={16} /> Carregando servidores MCP...
              </div>
            )}

            {/* Erro geral na consulta */}
            {mcpError && !isLoadingMcp && (
              <div
                style={{
                  padding: '16px',
                  borderRadius: 8,
                  backgroundColor: 'rgba(234, 67, 53, 0.1)',
                  color: 'var(--status-failed)',
                  fontSize: 13,
                  textAlign: 'center'
                }}
              >
                {mcpError}
              </div>
            )}

            {/* Estado quando CLI agy não está disponível */}
            {!mcpAvailable && !isLoadingMcp && (
              <div
                style={{
                  padding: '24px 16px',
                  textAlign: 'center',
                  color: 'var(--text-muted)',
                  fontSize: 13,
                  backgroundColor: 'var(--bg-sidebar)',
                  borderRadius: 10,
                  border: '1px solid var(--border-subtle)',
                  lineHeight: 1.5
                }}
              >
                CLI agy não encontrado — gerenciamento MCP indisponível
              </div>
            )}

            {/* Lista estilo ZCode de servidores */}
            {mcpAvailable && !isLoadingMcp && mcpServers.length === 0 && !mcpError && (
              <div
                style={{
                  padding: '24px 16px',
                  textAlign: 'center',
                  color: 'var(--text-muted)',
                  fontSize: 13
                }}
              >
                Nenhum servidor MCP configurado no agy.
              </div>
            )}

            {mcpAvailable && mcpServers.length > 0 && (
              <div
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 6,
                  overflowY: 'auto',
                  maxHeight: 320,
                  paddingRight: 4
                }}
              >
                {mcpServers.map((server) => {
                  const isEnabled = server.status === 'enabled'
                  const isToggling = togglingServer === server.name
                  const dotColor =
                    server.status === 'enabled'
                      ? 'var(--status-completed)'
                      : server.status === 'disabled'
                      ? 'var(--text-muted)'
                      : 'var(--status-progress)'

                  return (
                    <div
                      key={server.name}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        gap: 12,
                        padding: '10px 12px',
                        backgroundColor: 'var(--bg-sidebar)',
                        border: '1px solid var(--border-subtle)',
                        borderRadius: 10,
                        transition: 'background-color 0.15s'
                      }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0, flex: 1 }}>
                        {/* Status dot colorido */}
                        <div
                          title={`Status: ${server.status}`}
                          style={{
                            width: 8,
                            height: 8,
                            borderRadius: '50%',
                            backgroundColor: dotColor,
                            flexShrink: 0
                          }}
                        />

                        {/* Nome e badge do type */}
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexShrink: 0 }}>
                          <span style={{ fontWeight: 600, fontSize: 13, color: 'var(--text-primary)' }}>
                            {server.name}
                          </span>
                          <span
                            style={{
                              fontSize: 10,
                              textTransform: 'uppercase',
                              fontWeight: 600,
                              padding: '2px 6px',
                              borderRadius: 4,
                              backgroundColor: 'var(--bg-pill)',
                              color: 'var(--text-muted)'
                            }}
                          >
                            {server.type}
                          </span>
                        </div>

                        {/* Comando truncado com title */}
                        <span
                          title={server.command}
                          style={{
                            fontSize: 12,
                            color: 'var(--text-muted)',
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap',
                            minWidth: 0,
                            flex: 1,
                            fontFamily: 'var(--font-mono)'
                          }}
                        >
                          {server.command}
                        </span>
                      </div>

                      {/* Switch por linha */}
                      <button
                        type="button"
                        role="switch"
                        aria-checked={isEnabled}
                        disabled={isToggling}
                        onClick={() => handleToggle(server)}
                        title={isEnabled ? 'Desativar servidor' : 'Ativar servidor'}
                        style={{
                          width: 36,
                          height: 20,
                          borderRadius: 10,
                          backgroundColor: isEnabled ? 'var(--accent-blue)' : 'var(--border-card)',
                          border: 'none',
                          padding: 2,
                          display: 'flex',
                          alignItems: 'center',
                          cursor: isToggling ? 'wait' : 'pointer',
                          position: 'relative',
                          transition: 'background-color 0.2s',
                          flexShrink: 0,
                          opacity: isToggling ? 0.6 : 1
                        }}
                      >
                        <div
                          style={{
                            width: 16,
                            height: 16,
                            borderRadius: '50%',
                            backgroundColor: '#ffffff',
                            transform: isEnabled ? 'translateX(16px)' : 'translateX(0px)',
                            transition: 'transform 0.2s',
                            boxShadow: '0 1px 3px rgba(0,0,0,0.3)'
                          }}
                        />
                      </button>
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        )}

        {/* Rodapé do Modal */}
        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 4 }}>
          <button
            type="button"
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
