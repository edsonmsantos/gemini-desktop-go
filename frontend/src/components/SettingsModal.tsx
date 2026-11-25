import React, { useState, useEffect, useCallback } from 'react'
import { CloseIcon, RefreshIcon, SpinnerIcon } from './icons'
import { MCPServer, SavedPrompt, Skill } from '../types'
import { fetchMCPServers, fetchSkills, toggleMCPServer, fetchSavedPrompts, createSavedPrompt, updateSavedPrompt, deleteSavedPrompt } from '../api'

interface SettingsModalProps {
  isOpen: boolean
  onClose: () => void
  port: number
  agentCmd: string
  theme: 'dark' | 'light'
  onToggleTheme: () => void
  initialTab?: 'general' | 'mcp' | 'skills' | 'prompts'
  cwd?: string
}

export const SettingsModal: React.FC<SettingsModalProps> = ({
  isOpen,
  onClose,
  port,
  agentCmd,
  theme,
  onToggleTheme,
  initialTab = 'general',
  cwd
}) => {
  const [activeTab, setActiveTab] = useState<'general' | 'mcp' | 'skills' | 'prompts'>(initialTab)

  useEffect(() => {
    if (isOpen && initialTab) {
      setActiveTab(initialTab)
    }
  }, [isOpen, initialTab])

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

  // Estado da aba Skills
  const [skills, setSkills] = useState<Skill[]>([])
  const [skillsUserDir, setSkillsUserDir] = useState<string>('')
  const [skillsWorkspaceDir, setSkillsWorkspaceDir] = useState<string | null>(null)
  const [isLoadingSkills, setIsLoadingSkills] = useState<boolean>(false)
  const [skillsError, setSkillsError] = useState<string | null>(null)

  const loadSkills = useCallback(async () => {
    setIsLoadingSkills(true)
    setSkillsError(null)
    try {
      const res = await fetchSkills(cwd || undefined, true)
      setSkills(res.skills ?? [])
      setSkillsUserDir(res.userDir ?? '')
      setSkillsWorkspaceDir(res.workspaceDir ?? null)
    } catch (err: unknown) {
      setSkillsError(err instanceof Error ? err.message : 'Falha ao consultar skills')
    } finally {
      setIsLoadingSkills(false)
    }
  }, [cwd])

  useEffect(() => {
    if (isOpen && activeTab === 'skills') {
      loadSkills()
    }
  }, [isOpen, activeTab, loadSkills])

  // Estado da aba Prompts
  const [savedPrompts, setSavedPrompts] = useState<SavedPrompt[]>([])
  const [isLoadingPrompts, setIsLoadingPrompts] = useState<boolean>(false)
  const [promptsError, setPromptsError] = useState<string | null>(null)
  const [editingPromptId, setEditingPromptId] = useState<string | null>(null)
  const [formTitle, setFormTitle] = useState<string>('')
  const [formContent, setFormContent] = useState<string>('')
  const [isSavingPrompt, setIsSavingPrompt] = useState<boolean>(false)
  const [deletingPromptId, setDeletingPromptId] = useState<string | null>(null)

  const loadSavedPrompts = useCallback(async () => {
    setIsLoadingPrompts(true)
    setPromptsError(null)
    try {
      const res = await fetchSavedPrompts()
      setSavedPrompts(res ?? [])
    } catch (err: unknown) {
      setPromptsError(err instanceof Error ? err.message : 'Falha ao carregar prompts')
    } finally {
      setIsLoadingPrompts(false)
    }
  }, [])

  useEffect(() => {
    if (isOpen) {
      loadSavedPrompts()
    }
  }, [isOpen, loadSavedPrompts])

  const handleSavePrompt = async (e?: React.FormEvent) => {
    if (e) e.preventDefault()
    const title = formTitle.trim()
    const content = formContent.trim()
    if (!title || !content) return

    setIsSavingPrompt(true)
    setPromptsError(null)
    try {
      if (editingPromptId) {
        const updated = await updateSavedPrompt(editingPromptId, { title, content })
        if (updated) {
          setSavedPrompts((prev) => prev.map((p) => (p.id === editingPromptId ? updated : p)))
          setEditingPromptId(null)
          setFormTitle('')
          setFormContent('')
        }
      } else {
        const created = await createSavedPrompt(title, content)
        if (created) {
          setSavedPrompts((prev) => [created, ...prev])
          setFormTitle('')
          setFormContent('')
        }
      }
    } catch (err: unknown) {
      setPromptsError(err instanceof Error ? err.message : 'Erro ao salvar prompt')
    } finally {
      setIsSavingPrompt(false)
    }
  }

  const handleStartEdit = (p: SavedPrompt) => {
    setEditingPromptId(p.id)
    setFormTitle(p.title)
    setFormContent(p.content)
    setDeletingPromptId(null)
  }

  const handleCancelEdit = () => {
    setEditingPromptId(null)
    setFormTitle('')
    setFormContent('')
  }

  const handleDeletePrompt = async (id: string) => {
    if (deletingPromptId === id) {
      try {
        const ok = await deleteSavedPrompt(id)
        if (ok) {
          setSavedPrompts((prev) => prev.filter((p) => p.id !== id))
          if (editingPromptId === id) {
            handleCancelEdit()
          }
        }
      } catch (err: unknown) {
        setPromptsError(err instanceof Error ? err.message : 'Erro ao excluir prompt')
      } finally {
        setDeletingPromptId(null)
      }
    } else {
      setDeletingPromptId(id)
    }
  }

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
          width: activeTab === 'mcp' || activeTab === 'prompts' ? 580 : 480,
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
          <button
            type="button"
            onClick={() => setActiveTab('skills')}
            style={{
              padding: '6px 14px',
              borderRadius: 14,
              fontSize: 13,
              fontWeight: 500,
              backgroundColor: activeTab === 'skills' ? 'var(--bg-pill-hover)' : 'transparent',
              color: activeTab === 'skills' ? 'var(--text-primary)' : 'var(--text-muted)',
              border: 'none',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: 6,
              transition: 'background 0.15s, color 0.15s'
            }}
          >
            <span>Skills</span>
            {skills.length > 0 && (
              <span
                style={{
                  fontSize: 11,
                  padding: '1px 6px',
                  borderRadius: 10,
                  backgroundColor: 'var(--bg-pill)',
                  color: 'var(--text-secondary)'
                }}
              >
                {skills.length}
              </span>
            )}
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('prompts')}
            style={{
              padding: '6px 14px',
              borderRadius: 14,
              fontSize: 13,
              fontWeight: 500,
              backgroundColor: activeTab === 'prompts' ? 'var(--bg-pill-hover)' : 'transparent',
              color: activeTab === 'prompts' ? 'var(--text-primary)' : 'var(--text-muted)',
              border: 'none',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: 6,
              transition: 'background 0.15s, color 0.15s'
            }}
          >
            <span>Prompts</span>
            {savedPrompts.length > 0 && (
              <span
                style={{
                  fontSize: 11,
                  padding: '1px 6px',
                  borderRadius: 10,
                  backgroundColor: 'var(--bg-pill)',
                  color: 'var(--text-secondary)'
                }}
              >
                {savedPrompts.length}
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

        {/* Conteúdo da Aba Skills */}
        {activeTab === 'skills' && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12, flex: 1, minHeight: 0 }}>
            {/* Header da seção Skills: contador e botão refresh */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <span style={{ fontSize: 13, fontWeight: 500, color: 'var(--text-secondary)' }}>
                {skills.length} skill{skills.length === 1 ? '' : 's'} disponíve{skills.length === 1 ? 'l' : 'is'}
              </span>
              <button
                type="button"
                onClick={loadSkills}
                disabled={isLoadingSkills}
                title="Atualizar lista de skills"
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
                  cursor: isLoadingSkills ? 'wait' : 'pointer'
                }}
              >
                <RefreshIcon size={14} className={isLoadingSkills ? 'animate-spin' : ''} />
                <span>Atualizar</span>
              </button>
            </div>

            {/* Diretórios varridos */}
            <div
              style={{
                padding: '8px 12px',
                borderRadius: 8,
                backgroundColor: 'var(--bg-sidebar)',
                border: '1px solid var(--border-subtle)',
                fontSize: 11,
                color: 'var(--text-muted)',
                lineHeight: 1.6,
                fontFamily: 'var(--font-mono)'
              }}
            >
              <div title={skillsUserDir}>global: {skillsUserDir || '~/.agents/skills'}</div>
              <div title={skillsWorkspaceDir || undefined}>
                workspace: {skillsWorkspaceDir || (cwd ? `${cwd}/.agents/skills` : '— (sem workspace ativo)')}
              </div>
            </div>

            {/* Carregando */}
            {isLoadingSkills && skills.length === 0 && (
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
                <SpinnerIcon size={16} /> Carregando skills...
              </div>
            )}

            {/* Erro geral na consulta */}
            {skillsError && !isLoadingSkills && (
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
                {skillsError}
              </div>
            )}

            {/* Estado vazio com instruções */}
            {!isLoadingSkills && !skillsError && skills.length === 0 && (
              <div
                style={{
                  padding: '24px 16px',
                  textAlign: 'center',
                  color: 'var(--text-muted)',
                  fontSize: 13,
                  backgroundColor: 'var(--bg-sidebar)',
                  borderRadius: 10,
                  border: '1px solid var(--border-subtle)',
                  lineHeight: 1.6
                }}
              >
                Nenhuma skill encontrada.
                <br />
                <span style={{ fontSize: 12 }}>
                  Crie <code style={{ fontFamily: 'var(--font-mono)', color: 'var(--text-secondary)' }}>~/.agents/skills/&lt;slug&gt;/SKILL.md</code> com
                  frontmatter <code style={{ fontFamily: 'var(--font-mono)', color: 'var(--text-secondary)' }}>name</code> e{' '}
                  <code style={{ fontFamily: 'var(--font-mono)', color: 'var(--text-secondary)' }}>description</code>.
                </span>
              </div>
            )}

            {/* Lista de skills */}
            {skills.length > 0 && (
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
                {skills.map((skill) => (
                  <div
                    key={skill.slug}
                    title={skill.path}
                    style={{
                      padding: '10px 12px',
                      backgroundColor: 'var(--bg-sidebar)',
                      border: '1px solid var(--border-subtle)',
                      borderRadius: 10
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <span style={{ fontWeight: 600, fontSize: 13, color: 'var(--text-primary)' }}>{skill.name}</span>
                      <span
                        style={{
                          fontSize: 10,
                          fontFamily: 'var(--font-mono)',
                          padding: '2px 6px',
                          borderRadius: 4,
                          backgroundColor: 'var(--bg-pill)',
                          color: 'var(--text-muted)'
                        }}
                      >
                        /{skill.slug}
                      </span>
                      <span
                        style={{
                          fontSize: 10,
                          textTransform: 'uppercase',
                          fontWeight: 600,
                          padding: '2px 6px',
                          borderRadius: 4,
                          backgroundColor: 'var(--bg-pill)',
                          color: skill.scope === 'workspace' ? 'var(--accent-blue)' : 'var(--text-muted)'
                        }}
                      >
                        {skill.scope === 'workspace' ? 'workspace' : 'global'}
                      </span>
                    </div>
                    {skill.description && (
                      <div
                        style={{
                          marginTop: 4,
                          fontSize: 12,
                          color: 'var(--text-secondary)',
                          lineHeight: 1.4,
                          display: '-webkit-box',
                          WebkitLineClamp: 2,
                          WebkitBoxOrient: 'vertical',
                          overflow: 'hidden'
                        }}
                      >
                        {skill.description}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}

            {/* Dica de uso na conversa */}
            <div style={{ fontSize: 12, color: 'var(--text-muted)', lineHeight: 1.5, padding: '0 2px' }}>
              💡 Em uma conversa, digite <code style={{ fontFamily: 'var(--font-mono)' }}>/</code> ou{' '}
              <code style={{ fontFamily: 'var(--font-mono)' }}>$</code> no campo de prompt para inserir uma skill.
            </div>
          </div>
        )}

        {/* Conteúdo da Aba Prompts */}
        {activeTab === 'prompts' && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14, overflowY: 'auto', maxHeight: '60vh' }}>
            {/* Mensagem de erro se houver */}
            {promptsError && (
              <div
                style={{
                  padding: '8px 12px',
                  borderRadius: 8,
                  backgroundColor: 'rgba(239, 68, 68, 0.1)',
                  border: '1px solid var(--status-failed)',
                  color: 'var(--status-failed)',
                  fontSize: 13
                }}
              >
                {promptsError}
              </div>
            )}

            {/* Formulário de criação/edição no topo */}
            <form
              onSubmit={handleSavePrompt}
              style={{
                display: 'flex',
                flexDirection: 'column',
                gap: 8,
                padding: 12,
                borderRadius: 10,
                backgroundColor: 'var(--bg-sidebar)',
                border: '1px solid var(--border-subtle)'
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>
                  {editingPromptId ? 'Editar Prompt' : 'Novo Prompt'}
                </span>
                {editingPromptId && (
                  <button
                    type="button"
                    onClick={handleCancelEdit}
                    style={{
                      fontSize: 12,
                      color: 'var(--text-muted)',
                      border: 'none',
                      background: 'none',
                      cursor: 'pointer'
                    }}
                  >
                    Cancelar edição
                  </button>
                )}
              </div>

              <input
                type="text"
                placeholder="Título do prompt (ex: Revisão de código)"
                value={formTitle}
                onChange={(e) => setFormTitle(e.target.value)}
                style={{
                  padding: '7px 10px',
                  borderRadius: 6,
                  backgroundColor: 'var(--bg-modal)',
                  border: '1px solid var(--border-card)',
                  color: 'var(--text-primary)',
                  fontSize: 13,
                  outline: 'none'
                }}
              />

              <textarea
                placeholder="Conteúdo do prompt..."
                value={formContent}
                onChange={(e) => setFormContent(e.target.value)}
                rows={3}
                style={{
                  padding: '7px 10px',
                  borderRadius: 6,
                  backgroundColor: 'var(--bg-modal)',
                  border: '1px solid var(--border-card)',
                  color: 'var(--text-primary)',
                  fontSize: 13,
                  resize: 'vertical',
                  outline: 'none',
                  fontFamily: 'inherit'
                }}
              />

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 6, marginTop: 2 }}>
                <button
                  type="submit"
                  disabled={isSavingPrompt || !formTitle.trim() || !formContent.trim()}
                  style={{
                    padding: '6px 14px',
                    borderRadius: 6,
                    backgroundColor: 'var(--accent-blue)',
                    color: '#ffffff',
                    fontSize: 12,
                    fontWeight: 500,
                    border: 'none',
                    cursor: isSavingPrompt || !formTitle.trim() || !formContent.trim() ? 'not-allowed' : 'pointer',
                    opacity: isSavingPrompt || !formTitle.trim() || !formContent.trim() ? 0.6 : 1
                  }}
                >
                  {isSavingPrompt ? 'Salvando...' : editingPromptId ? 'Salvar Alterações' : 'Salvar Prompt'}
                </button>
              </div>
            </form>

            {/* Cabeçalho da Lista */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginTop: 4 }}>
              <span style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
                Prompts Salvos ({savedPrompts.length})
              </span>
              <button
                type="button"
                onClick={loadSavedPrompts}
                disabled={isLoadingPrompts}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 4,
                  fontSize: 12,
                  color: 'var(--text-muted)',
                  border: 'none',
                  background: 'none',
                  cursor: 'pointer'
                }}
              >
                <RefreshIcon size={12} />
                <span>Atualizar</span>
              </button>
            </div>

            {/* Lista de Prompts */}
            {isLoadingPrompts && savedPrompts.length === 0 ? (
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: 16, color: 'var(--text-muted)', fontSize: 13 }}>
                <SpinnerIcon size={14} /> Carregando prompts salvos...
              </div>
            ) : savedPrompts.length === 0 ? (
              <div style={{ padding: '24px 12px', textAlign: 'center', color: 'var(--text-muted)', fontSize: 13 }}>
                Nenhum prompt salvo ainda. Crie um acima para reutilizar com o gatilho <code style={{ fontFamily: 'var(--font-mono)' }}>@</code>!
              </div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {savedPrompts.map((p) => {
                  const singleLinePreview = p.content.replace(/[\r\n]+/g, ' ').trim()
                  const isConfirmingDelete = deletingPromptId === p.id

                  return (
                    <div
                      key={p.id}
                      style={{
                        padding: '10px 12px',
                        borderRadius: 8,
                        backgroundColor: 'var(--bg-sidebar)',
                        border: '1px solid var(--border-subtle)',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        gap: 12
                      }}
                    >
                      <div style={{ minWidth: 0, flex: 1 }}>
                        <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                          {p.title}
                        </div>
                        <div
                          style={{
                            marginTop: 3,
                            fontSize: 12,
                            color: 'var(--text-secondary)',
                            whiteSpace: 'nowrap',
                            overflow: 'hidden',
                            textOverflow: 'ellipsis'
                          }}
                          title={p.content}
                        >
                          {singleLinePreview}
                        </div>
                      </div>

                      <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexShrink: 0 }}>
                        <button
                          type="button"
                          onClick={() => handleStartEdit(p)}
                          style={{
                            padding: '4px 8px',
                            fontSize: 12,
                            borderRadius: 6,
                            border: '1px solid var(--border-card)',
                            backgroundColor: 'transparent',
                            color: 'var(--text-secondary)',
                            cursor: 'pointer'
                          }}
                        >
                          Editar
                        </button>
                        <button
                          type="button"
                          onClick={() => handleDeletePrompt(p.id)}
                          style={{
                            padding: '4px 8px',
                            fontSize: 12,
                            borderRadius: 6,
                            border: isConfirmingDelete ? '1px solid var(--status-failed)' : '1px solid var(--border-card)',
                            backgroundColor: isConfirmingDelete ? 'rgba(239, 68, 68, 0.15)' : 'transparent',
                            color: isConfirmingDelete ? 'var(--status-failed)' : 'var(--text-muted)',
                            fontWeight: isConfirmingDelete ? 600 : 400,
                            cursor: 'pointer',
                            transition: 'all 0.15s'
                          }}
                        >
                          {isConfirmingDelete ? 'Confirmar?' : 'Excluir'}
                        </button>
                      </div>
                    </div>
                  )
                })}
              </div>
            )}

            {/* Dica de uso na conversa */}
            <div style={{ fontSize: 12, color: 'var(--text-muted)', lineHeight: 1.5, padding: '0 2px' }}>
              💡 Em uma conversa, digite <code style={{ fontFamily: 'var(--font-mono)' }}>@</code> no campo de mensagem para inserir um prompt salvo.
            </div>
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
