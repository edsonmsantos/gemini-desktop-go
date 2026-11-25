import React, { useState, useEffect, useRef, useMemo } from 'react'
import { GeminiSparkle, MoreVerticalIcon, PenIcon, SearchIcon, SidebarCollapseIcon, SpinnerIcon, TrashIcon } from './icons'
import { WorkspaceList } from './WorkspaceList'
import { AgentFooter } from './AgentFooter'
import { AgentInfo, Conversation, Workspace } from '../types'
import { deleteFolder, fetchConversations, setConversationFolder } from '../api'

interface SidebarProps {
  conversations: Conversation[]
  activeConversationId: string | null
  processingConversationIds: Set<string>
  workspaces: Workspace[]
  selectedCwd: string
  agentInfo: AgentInfo | null
  theme: 'dark' | 'light'
  onToggleTheme: () => void
  onNewConversation: () => void
  onSelectConversation: (id: string) => void
  onDeleteConversation: (id: string) => void
  onRenameConversation: (id: string, newTitle: string) => Promise<void>
  onSelectCwd: (cwd: string) => void
  onOpenSettings: () => void
  onMoveConversation?: (id: string, folder: string) => void
}

export const Sidebar: React.FC<SidebarProps> = ({
  conversations,
  activeConversationId,
  processingConversationIds,
  workspaces,
  selectedCwd,
  agentInfo,
  theme,
  onToggleTheme,
  onNewConversation,
  onSelectConversation,
  onDeleteConversation,
  onRenameConversation,
  onSelectCwd,
  onOpenSettings,
  onMoveConversation
}) => {
  const [isCollapsed, setIsCollapsed] = useState(false)
  const [filterText, setFilterText] = useState('')
  const [showSearchInput, setShowSearchInput] = useState(false)

  // Sincronização e controle local de conversas para atualização otimista
  const [localConversations, setLocalConversations] = useState<Conversation[]>(conversations)

  useEffect(() => {
    setLocalConversations(conversations)
  }, [conversations])

  // Estado das pastas colapsadas persistido em localStorage
  const [collapsedFolders, setCollapsedFolders] = useState<Record<string, boolean>>(() => {
    try {
      const saved = localStorage.getItem('gemini_desktop_collapsed_folders')
      return saved ? JSON.parse(saved) : {}
    } catch {
      return {}
    }
  })

  const toggleFolder = (name: string) => {
    setCollapsedFolders((prev) => {
      const next = { ...prev, [name]: !prev[name] }
      try {
        localStorage.setItem('gemini_desktop_collapsed_folders', JSON.stringify(next))
      } catch {}
      return next
    })
  }

  // Estados de edição / ações de conversas e pastas
  const [hoveredConvId, setHoveredConvId] = useState<string | null>(null)
  const [openMenuId, setOpenMenuId] = useState<string | null>(null)
  const [editingConvId, setEditingConvId] = useState<string | null>(null)
  const [editConvTitle, setEditConvTitle] = useState<string>('')

  const [movingConvId, setMovingConvId] = useState<string | null>(null)
  const [showNewFolderInput, setShowNewFolderInput] = useState(false)
  const [newFolderName, setNewFolderName] = useState('')

  const [editingFolderName, setEditingFolderName] = useState<string | null>(null)
  const [newFolderTitle, setNewFolderTitle] = useState<string>('')
  const [deletingFolderName, setDeletingFolderName] = useState<string | null>(null)

  const [toastMessage, setToastMessage] = useState<string | null>(null)

  const menuRef = useRef<HTMLDivElement>(null)
  const moveMenuRef = useRef<HTMLDivElement>(null)
  const convInputRef = useRef<HTMLInputElement>(null)
  const folderInputRef = useRef<HTMLInputElement>(null)
  const newFolderInputRef = useRef<HTMLInputElement>(null)

  // Foco em inputs de renomeação
  useEffect(() => {
    if (editingConvId && convInputRef.current) {
      convInputRef.current.focus()
      convInputRef.current.select()
    }
  }, [editingConvId])

  useEffect(() => {
    if (editingFolderName && folderInputRef.current) {
      folderInputRef.current.focus()
      folderInputRef.current.select()
    }
  }, [editingFolderName])

  useEffect(() => {
    if (showNewFolderInput && newFolderInputRef.current) {
      newFolderInputRef.current.focus()
    }
  }, [showNewFolderInput])

  // Fechar menus de contexto com clique fora ou Esc
  useEffect(() => {
    const handleGlobalClick = (e: MouseEvent) => {
      const target = e.target as Node
      if (menuRef.current && !menuRef.current.contains(target)) {
        setOpenMenuId(null)
      }
      if (moveMenuRef.current && !moveMenuRef.current.contains(target)) {
        setMovingConvId(null)
        setShowNewFolderInput(false)
        setNewFolderName('')
      }
    }
    const handleGlobalKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpenMenuId(null)
        setMovingConvId(null)
        setShowNewFolderInput(false)
        setNewFolderName('')
        if (editingConvId) setEditingConvId(null)
        if (editingFolderName) setEditingFolderName(null)
        if (deletingFolderName) setDeletingFolderName(null)
      }
    }

    document.addEventListener('mousedown', handleGlobalClick)
    document.addEventListener('keydown', handleGlobalKeyDown)
    return () => {
      document.removeEventListener('mousedown', handleGlobalClick)
      document.removeEventListener('keydown', handleGlobalKeyDown)
    }
  }, [editingConvId, editingFolderName, deletingFolderName])

  // Ações de Conversa
  const handleSaveConvRename = async (id: string) => {
    const trimmed = editConvTitle.trim()
    const targetConv = localConversations.find((c) => c.id === id)
    if (!trimmed || trimmed === (targetConv?.title?.trim() || 'Nova conversa')) {
      setEditingConvId(null)
      return
    }

    setEditingConvId(null)
    try {
      await onRenameConversation(id, trimmed)
      setLocalConversations((prev) =>
        prev.map((c) => (c.id === id ? { ...c, title: trimmed } : c))
      )
    } catch {
      setToastMessage('Falha ao renomear conversa')
      setTimeout(() => setToastMessage(null), 3000)
    }
  }

  const handleMoveConversation = async (convId: string, targetFolder: string) => {
    const trimmed = targetFolder.trim()
    setMovingConvId(null)
    setShowNewFolderInput(false)
    setNewFolderName('')

    // Atualização otimista
    setLocalConversations((prev) =>
      prev.map((c) => (c.id === convId ? { ...c, folder: trimmed } : c))
    )
    if (onMoveConversation) {
      onMoveConversation(convId, trimmed)
    }

    try {
      await setConversationFolder(convId, trimmed)
      const updated = await fetchConversations()
      if (updated) {
        setLocalConversations(updated)
      }
    } catch {
      setToastMessage('Falha ao mover conversa')
      setTimeout(() => setToastMessage(null), 3000)
      try {
        const updated = await fetchConversations()
        if (updated) setLocalConversations(updated)
      } catch {}
    }
  }

  // Ações de Pastas
  const handleRenameFolder = async (oldName: string, newName: string) => {
    const trimmed = newName.trim()
    setEditingFolderName(null)
    if (!trimmed || trimmed === oldName) return

    // Atualização otimista
    setLocalConversations((prev) =>
      prev.map((c) => (c.folder === oldName ? { ...c, folder: trimmed } : c))
    )

    try {
      const convsToUpdate = localConversations.filter((c) => c.folder === oldName)
      await Promise.all(convsToUpdate.map((c) => setConversationFolder(c.id, trimmed)))
      const updated = await fetchConversations()
      if (updated) {
        setLocalConversations(updated)
      }
    } catch {
      setToastMessage('Falha ao renomear pasta')
      setTimeout(() => setToastMessage(null), 3000)
      try {
        const updated = await fetchConversations()
        if (updated) setLocalConversations(updated)
      } catch {}
    }
  }

  const handleDeleteFolder = async (folderName: string) => {
    setDeletingFolderName(null)

    // Atualização otimista: conversas voltam a não ter pasta
    setLocalConversations((prev) =>
      prev.map((c) => (c.folder === folderName ? { ...c, folder: '' } : c))
    )

    try {
      await deleteFolder(folderName)
      const updated = await fetchConversations()
      if (updated) {
        setLocalConversations(updated)
      }
    } catch {
      setToastMessage('Falha ao excluir pasta')
      setTimeout(() => setToastMessage(null), 3000)
      try {
        const updated = await fetchConversations()
        if (updated) setLocalConversations(updated)
      } catch {}
    }
  }

  // Agrupamento por pasta
  const foldersMap = useMemo(() => {
    const map: Record<string, Conversation[]> = {}
    for (const conv of localConversations) {
      const f = conv.folder?.trim()
      if (f) {
        if (!map[f]) map[f] = []
        map[f].push(conv)
      }
    }
    return map
  }, [localConversations])

  const sortedFolderNames = useMemo(() => {
    return Object.keys(foldersMap).sort((a, b) => a.localeCompare(b))
  }, [foldersMap])

  const unfiledConversations = useMemo(() => {
    return localConversations.filter((c) => !c.folder || !c.folder.trim())
  }, [localConversations])

  // Filtro de texto
  const matchFilter = (c: Conversation) => {
    if (!filterText.trim()) return true
    const q = filterText.toLowerCase()
    return (c.title || 'Nova conversa').toLowerCase().includes(q)
  }

  // Renderiza um item de conversa
  const renderConversationItem = (conv: Conversation, isInFolder: boolean) => {
    const isActive = activeConversationId === conv.id
    const isHovered = hoveredConvId === conv.id
    const isMenuOpen = openMenuId === conv.id
    const isMoving = movingConvId === conv.id
    const isEditing = editingConvId === conv.id
    const isProcessing = processingConversationIds.has(conv.id)
    const title = conv.title?.trim() || 'Nova conversa'

    return (
      <div
        key={conv.id}
        onMouseEnter={() => setHoveredConvId(conv.id)}
        onMouseLeave={() => setHoveredConvId(null)}
        style={{
          position: 'relative',
          display: 'flex',
          alignItems: 'center',
          width: '100%',
          borderRadius: 8,
          backgroundColor: isActive
            ? 'var(--bg-sidebar-active)'
            : isHovered || isMenuOpen || isMoving
            ? 'var(--bg-sidebar-hover)'
            : 'transparent',
          transition: 'background 0.15s'
        }}
      >
        {isEditing ? (
          <div style={{ flex: 1, padding: '4px 6px', display: 'flex' }}>
            <input
              ref={convInputRef}
              type="text"
              maxLength={80}
              value={editConvTitle}
              onChange={(e) => setEditConvTitle(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault()
                  handleSaveConvRename(conv.id)
                } else if (e.key === 'Escape') {
                  e.preventDefault()
                  setEditingConvId(null)
                }
              }}
              onBlur={() => handleSaveConvRename(conv.id)}
              onClick={(e) => e.stopPropagation()}
              style={{
                width: '100%',
                fontSize: 13,
                padding: '5px 8px',
                borderRadius: 6,
                backgroundColor: 'var(--bg-app)',
                color: 'var(--text-primary)',
                border: '1px solid var(--border-focus)',
                boxShadow: '0 0 0 1px var(--border-focus)'
              }}
            />
          </div>
        ) : (
          <>
            <button
              onClick={() => onSelectConversation(conv.id)}
              title={title}
              style={{
                flex: 1,
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                padding: '8px 10px',
                paddingRight: isHovered || isMenuOpen || isMoving ? (isInFolder ? 80 : 58) : 10,
                fontSize: 13,
                fontWeight: isActive ? 500 : 400,
                color: isActive ? 'var(--text-primary)' : 'var(--text-secondary)',
                textAlign: 'left',
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis'
              }}
            >
              {isProcessing && <SpinnerIcon size={14} color="var(--accent-blue)" />}
              <span
                style={{
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  opacity: isProcessing ? 0.75 : 1
                }}
              >
                {title}
              </span>
            </button>

            {/* Ações no hover do item */}
            {(isHovered || isMenuOpen || isMoving) && (
              <div
                style={{
                  position: 'absolute',
                  right: 4,
                  display: 'flex',
                  alignItems: 'center',
                  gap: 2
                }}
              >
                {/* Botão remover da pasta (apenas se estiver em pasta) */}
                {isInFolder && (
                  <button
                    onClick={(e) => {
                      e.stopPropagation()
                      handleMoveConversation(conv.id, '')
                    }}
                    title="Remover da pasta"
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      width: 22,
                      height: 22,
                      borderRadius: 4,
                      fontSize: 12,
                      color: 'var(--text-muted)',
                      backgroundColor: 'transparent',
                      transition: 'color 0.15s, background 0.15s'
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.color = 'var(--text-primary)'
                      e.currentTarget.style.backgroundColor = 'var(--bg-pill)'
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.color = 'var(--text-muted)'
                      e.currentTarget.style.backgroundColor = 'transparent'
                    }}
                  >
                    ↩
                  </button>
                )}

                {/* Botão Mover para pasta "📁→" */}
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    setOpenMenuId(null)
                    setMovingConvId(isMoving ? null : conv.id)
                    setShowNewFolderInput(false)
                    setNewFolderName('')
                  }}
                  title="Mover para pasta"
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    padding: '2px 4px',
                    height: 22,
                    borderRadius: 4,
                    fontSize: 11,
                    color: isMoving ? 'var(--text-primary)' : 'var(--text-muted)',
                    backgroundColor: isMoving ? 'var(--bg-sidebar-active)' : 'transparent',
                    transition: 'color 0.15s, background 0.15s'
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.color = 'var(--text-primary)'
                    e.currentTarget.style.backgroundColor = 'var(--bg-pill)'
                  }}
                  onMouseLeave={(e) => {
                    if (!isMoving) {
                      e.currentTarget.style.color = 'var(--text-muted)'
                      e.currentTarget.style.backgroundColor = 'transparent'
                    }
                  }}
                >
                  📁→
                </button>

                {/* Botão ⋮ */}
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    setMovingConvId(null)
                    setOpenMenuId(isMenuOpen ? null : conv.id)
                  }}
                  title="Opções da conversa"
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    width: 22,
                    height: 22,
                    borderRadius: 4,
                    color: isMenuOpen ? 'var(--text-primary)' : 'var(--text-muted)',
                    backgroundColor: isMenuOpen ? 'var(--bg-sidebar-active)' : 'transparent',
                    transition: 'color 0.15s, background 0.15s'
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.color = 'var(--text-primary)'
                    e.currentTarget.style.backgroundColor = 'var(--bg-pill)'
                  }}
                  onMouseLeave={(e) => {
                    if (!isMenuOpen) {
                      e.currentTarget.style.color = 'var(--text-muted)'
                      e.currentTarget.style.backgroundColor = 'transparent'
                    }
                  }}
                >
                  <MoreVerticalIcon size={14} />
                </button>
              </div>
            )}

            {/* Menu Popover "Mover para pasta" */}
            {isMoving && (
              <div
                ref={moveMenuRef}
                style={{
                  position: 'absolute',
                  right: 4,
                  top: 28,
                  zIndex: 110,
                  minWidth: 160,
                  backgroundColor: 'var(--bg-dropdown)',
                  border: '1px solid var(--border-card)',
                  borderRadius: 8,
                  boxShadow: 'var(--shadow-dropdown)',
                  padding: 4,
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 2
                }}
                onClick={(e) => e.stopPropagation()}
              >
                <div
                  style={{
                    fontSize: 11,
                    fontWeight: 600,
                    color: 'var(--text-muted)',
                    padding: '4px 8px',
                    textTransform: 'uppercase'
                  }}
                >
                  Mover para pasta
                </div>

                {sortedFolderNames.map((fName) => {
                  const isCurrent = conv.folder === fName
                  return (
                    <button
                      key={fName}
                      disabled={isCurrent}
                      onClick={() => handleMoveConversation(conv.id, fName)}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        padding: '6px 8px',
                        borderRadius: 6,
                        fontSize: 12,
                        color: isCurrent ? 'var(--text-muted)' : 'var(--text-primary)',
                        textAlign: 'left',
                        cursor: isCurrent ? 'default' : 'pointer',
                        opacity: isCurrent ? 0.6 : 1
                      }}
                      onMouseEnter={(e) => {
                        if (!isCurrent) e.currentTarget.style.backgroundColor = 'var(--bg-sidebar-hover)'
                      }}
                      onMouseLeave={(e) => {
                        e.currentTarget.style.backgroundColor = 'transparent'
                      }}
                    >
                      <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        📁 {fName}
                      </span>
                      {isCurrent && <span style={{ fontSize: 10 }}>atual</span>}
                    </button>
                  )
                })}

                {/* Opção Nova Pasta */}
                {!showNewFolderInput ? (
                  <button
                    onClick={() => setShowNewFolderInput(true)}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 6,
                      padding: '6px 8px',
                      borderRadius: 6,
                      fontSize: 12,
                      color: 'var(--accent-blue)',
                      textAlign: 'left',
                      borderTop: '1px solid var(--border-subtle)',
                      marginTop: 2
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.backgroundColor = 'var(--bg-sidebar-hover)'
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.backgroundColor = 'transparent'
                    }}
                  >
                    <span>+ Nova pasta…</span>
                  </button>
                ) : (
                  <div
                    style={{
                      padding: '4px 6px',
                      borderTop: '1px solid var(--border-subtle)',
                      display: 'flex',
                      flexDirection: 'column',
                      gap: 4
                    }}
                  >
                    <input
                      ref={newFolderInputRef}
                      type="text"
                      placeholder="Nome da pasta"
                      value={newFolderName}
                      onChange={(e) => setNewFolderName(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' && newFolderName.trim()) {
                          e.preventDefault()
                          handleMoveConversation(conv.id, newFolderName.trim())
                        } else if (e.key === 'Escape') {
                          setShowNewFolderInput(false)
                        }
                      }}
                      style={{
                        fontSize: 12,
                        padding: '4px 6px',
                        borderRadius: 4,
                        backgroundColor: 'var(--bg-app)',
                        color: 'var(--text-primary)',
                        border: '1px solid var(--border-focus)'
                      }}
                    />
                    <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 4 }}>
                      <button
                        onClick={() => setShowNewFolderInput(false)}
                        style={{ fontSize: 11, padding: '2px 6px', color: 'var(--text-muted)' }}
                      >
                        Cancelar
                      </button>
                      <button
                        onClick={() => {
                          if (newFolderName.trim()) {
                            handleMoveConversation(conv.id, newFolderName.trim())
                          }
                        }}
                        style={{
                          fontSize: 11,
                          padding: '2px 8px',
                          borderRadius: 4,
                          backgroundColor: 'var(--bg-pill)',
                          color: 'var(--text-primary)',
                          fontWeight: 500
                        }}
                      >
                        Mover
                      </button>
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Menu Dropdown de Contexto (Renomear / Excluir) */}
            {isMenuOpen && (
              <div
                ref={menuRef}
                style={{
                  position: 'absolute',
                  right: 4,
                  top: 28,
                  zIndex: 100,
                  minWidth: 124,
                  backgroundColor: 'var(--bg-dropdown)',
                  border: '1px solid var(--border-card)',
                  borderRadius: 8,
                  boxShadow: 'var(--shadow-dropdown)',
                  padding: 4,
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 2
                }}
                onClick={(e) => e.stopPropagation()}
              >
                <button
                  onClick={() => {
                    setOpenMenuId(null)
                    setEditingConvId(conv.id)
                    setEditConvTitle(conv.title || 'Nova conversa')
                  }}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 8,
                    padding: '6px 10px',
                    borderRadius: 6,
                    fontSize: 13,
                    color: 'var(--text-primary)',
                    textAlign: 'left',
                    width: '100%',
                    transition: 'background 0.15s'
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.backgroundColor = 'var(--bg-sidebar-hover)'
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.backgroundColor = 'transparent'
                  }}
                >
                  <PenIcon size={14} />
                  <span>Renomear</span>
                </button>

                <button
                  onClick={() => {
                    setOpenMenuId(null)
                    if (window.confirm(`Deseja excluir a conversa "${title}"?`)) {
                      onDeleteConversation(conv.id)
                    }
                  }}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 8,
                    padding: '6px 10px',
                    borderRadius: 6,
                    fontSize: 13,
                    color: 'var(--status-failed)',
                    textAlign: 'left',
                    width: '100%',
                    transition: 'background 0.15s'
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.backgroundColor = 'rgba(234, 67, 53, 0.12)'
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.backgroundColor = 'transparent'
                  }}
                >
                  <TrashIcon size={14} />
                  <span>Excluir</span>
                </button>
              </div>
            )}
          </>
        )}
      </div>
    )
  }

  if (isCollapsed) {
    return (
      <div
        style={{
          width: 64,
          height: '100vh',
          backgroundColor: 'var(--bg-sidebar)',
          borderRight: '1px solid var(--border-subtle)',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          padding: '16px 0',
          gap: 16,
          flexShrink: 0
        }}
      >
        <button
          onClick={() => setIsCollapsed(false)}
          title="Expandir menu"
          style={{
            padding: 8,
            borderRadius: '50%',
            color: 'var(--text-secondary)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}
        >
          <SidebarCollapseIcon size={20} />
        </button>

        <button
          onClick={onNewConversation}
          title="Nova conversa"
          style={{
            width: 42,
            height: 42,
            borderRadius: '50%',
            backgroundColor: 'var(--bg-pill)',
            color: 'var(--text-primary)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}
        >
          <PenIcon size={18} />
        </button>
      </div>
    )
  }

  const hasAnyConversation = localConversations.length > 0

  return (
    <aside
      style={{
        width: 280,
        height: '100vh',
        backgroundColor: 'var(--bg-sidebar)',
        borderRight: '1px solid var(--border-subtle)',
        display: 'flex',
        flexDirection: 'column',
        flexShrink: 0,
        overflow: 'hidden'
      }}
    >
      {/* Topo da Sidebar: Logo + Ações */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '16px 18px 12px 18px'
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <GeminiSparkle size={22} />
          <span style={{ fontSize: 18, fontWeight: 500, letterSpacing: '-0.02em', color: 'var(--text-primary)' }}>
            Gemini
          </span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
          <button
            onClick={() => setShowSearchInput(!showSearchInput)}
            title="Buscar conversas"
            style={{
              padding: 6,
              borderRadius: '50%',
              color: 'var(--text-secondary)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center'
            }}
          >
            <SearchIcon size={17} />
          </button>
          <button
            onClick={() => setIsCollapsed(true)}
            title="Recolher menu"
            style={{
              padding: 6,
              borderRadius: '50%',
              color: 'var(--text-secondary)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center'
            }}
          >
            <SidebarCollapseIcon size={17} />
          </button>
        </div>
      </div>

      {/* Botão Pill Destacado "Nova conversa" */}
      <div style={{ padding: '8px 14px' }}>
        <button
          onClick={onNewConversation}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 12,
            width: '100%',
            padding: '11px 18px',
            borderRadius: 24,
            backgroundColor: 'var(--bg-pill)',
            color: 'var(--text-primary)',
            fontSize: 14,
            fontWeight: 500,
            transition: 'background 0.2s',
            boxShadow: '0 1px 2px rgba(0,0,0,0.1)'
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.backgroundColor = 'var(--bg-pill-hover)'
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.backgroundColor = 'var(--bg-pill)'
          }}
        >
          <PenIcon size={16} />
          <span>Nova conversa</span>
        </button>
      </div>

      {/* Input de Busca de Conversas (expansível) */}
      {showSearchInput && (
        <div style={{ padding: '4px 14px 8px 14px' }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              padding: '6px 12px',
              borderRadius: 8,
              backgroundColor: 'var(--bg-app)',
              border: '1px solid var(--border-card)'
            }}
          >
            <SearchIcon size={14} color="var(--text-muted)" />
            <input
              type="text"
              placeholder="Filtrar histórico..."
              value={filterText}
              onChange={(e) => setFilterText(e.target.value)}
              autoFocus
              style={{
                flex: 1,
                fontSize: 13,
                backgroundColor: 'transparent',
                color: 'var(--text-primary)'
              }}
            />
            {filterText && (
              <button
                onClick={() => setFilterText('')}
                style={{ fontSize: 11, color: 'var(--text-muted)' }}
              >
                ✕
              </button>
            )}
          </div>
        </div>
      )}

      {/* Toast inline de erro */}
      {toastMessage && (
        <div
          style={{
            margin: '4px 14px',
            padding: '6px 10px',
            borderRadius: 6,
            backgroundColor: 'rgba(234, 67, 53, 0.12)',
            border: '1px solid var(--status-failed)',
            color: 'var(--status-failed)',
            fontSize: 12,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between'
          }}
        >
          <span>{toastMessage}</span>
          <button onClick={() => setToastMessage(null)} style={{ fontSize: 11, color: 'inherit' }}>
            ✕
          </button>
        </div>
      )}

      {/* Conteúdo rolável: Workspaces + Histórico agrupado */}
      <div style={{ flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 6 }}>
        <WorkspaceList
          workspaces={workspaces}
          selectedCwd={selectedCwd}
          onSelectCwd={onSelectCwd}
        />

        <div style={{ display: 'flex', flexDirection: 'column', gap: 4, padding: '4px 10px', flex: 1 }}>
          {/* Seções de Pastas */}
          {sortedFolderNames.map((folderName) => {
            const allFolderConvs = foldersMap[folderName] || []
            const visibleConvs = allFolderConvs.filter(matchFilter)
            const isFolderCollapsed = !!collapsedFolders[folderName]
            const isEditingThisFolder = editingFolderName === folderName
            const isDeletingThisFolder = deletingFolderName === folderName

            // Se busca ativa e não tem itens correspondentes, oculta pasta
            if (filterText.trim() && visibleConvs.length === 0) {
              return null
            }

            return (
              <div key={folderName} style={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                {/* Cabeçalho da Pasta */}
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    padding: '5px 8px',
                    borderRadius: 6,
                    backgroundColor: 'var(--bg-sidebar)',
                    transition: 'background 0.15s'
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.backgroundColor = 'var(--bg-sidebar-hover)'
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.backgroundColor = 'var(--bg-sidebar)'
                  }}
                >
                  {isEditingThisFolder ? (
                    <input
                      ref={folderInputRef}
                      type="text"
                      value={newFolderTitle}
                      onChange={(e) => setNewFolderTitle(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                          e.preventDefault()
                          handleRenameFolder(folderName, newFolderTitle)
                        } else if (e.key === 'Escape') {
                          e.preventDefault()
                          setEditingFolderName(null)
                        }
                      }}
                      onBlur={() => handleRenameFolder(folderName, newFolderTitle)}
                      onClick={(e) => e.stopPropagation()}
                      style={{
                        flex: 1,
                        fontSize: 12,
                        padding: '2px 6px',
                        borderRadius: 4,
                        backgroundColor: 'var(--bg-app)',
                        color: 'var(--text-primary)',
                        border: '1px solid var(--border-focus)'
                      }}
                    />
                  ) : (
                    <>
                      <button
                        onClick={() => toggleFolder(folderName)}
                        title={`Clique para ${isFolderCollapsed ? 'expandir' : 'recolher'} pasta`}
                        style={{
                          flex: 1,
                          display: 'flex',
                          alignItems: 'center',
                          gap: 6,
                          textAlign: 'left',
                          fontSize: 12,
                          fontWeight: 600,
                          color: 'var(--text-secondary)',
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap'
                        }}
                      >
                        <span style={{ fontSize: 9, color: 'var(--text-muted)' }}>
                          {isFolderCollapsed ? '▶' : '▼'}
                        </span>
                        <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          📁 {folderName} ({allFolderConvs.length})
                        </span>
                      </button>

                      {/* Ações do cabeçalho da pasta */}
                      <div style={{ display: 'flex', alignItems: 'center', gap: 2 }} onClick={(e) => e.stopPropagation()}>
                        {isDeletingThisFolder ? (
                          <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                            <span style={{ fontSize: 11, color: 'var(--status-failed)' }}>Confirmar?</span>
                            <button
                              onClick={() => handleDeleteFolder(folderName)}
                              style={{
                                fontSize: 11,
                                padding: '1px 5px',
                                borderRadius: 4,
                                backgroundColor: 'rgba(234, 67, 53, 0.2)',
                                color: 'var(--status-failed)',
                                fontWeight: 600
                              }}
                            >
                              Sim
                            </button>
                            <button
                              onClick={() => setDeletingFolderName(null)}
                              style={{
                                fontSize: 11,
                                padding: '1px 4px',
                                color: 'var(--text-muted)'
                              }}
                            >
                              Não
                            </button>
                          </div>
                        ) : (
                          <>
                            <button
                              onClick={() => {
                                setEditingFolderName(folderName)
                                setNewFolderTitle(folderName)
                              }}
                              title="Renomear pasta"
                              style={{
                                padding: 3,
                                borderRadius: 4,
                                color: 'var(--text-muted)',
                                display: 'flex',
                                alignItems: 'center',
                                justifyContent: 'center'
                              }}
                              onMouseEnter={(e) => {
                                e.currentTarget.style.color = 'var(--text-primary)'
                                e.currentTarget.style.backgroundColor = 'var(--bg-pill)'
                              }}
                              onMouseLeave={(e) => {
                                e.currentTarget.style.color = 'var(--text-muted)'
                                e.currentTarget.style.backgroundColor = 'transparent'
                              }}
                            >
                              <PenIcon size={12} />
                            </button>
                            <button
                              onClick={() => setDeletingFolderName(folderName)}
                              title="Excluir pasta"
                              style={{
                                padding: 3,
                                borderRadius: 4,
                                color: 'var(--text-muted)',
                                display: 'flex',
                                alignItems: 'center',
                                justifyContent: 'center'
                              }}
                              onMouseEnter={(e) => {
                                e.currentTarget.style.color = 'var(--status-failed)'
                                e.currentTarget.style.backgroundColor = 'rgba(234, 67, 53, 0.12)'
                              }}
                              onMouseLeave={(e) => {
                                e.currentTarget.style.color = 'var(--text-muted)'
                                e.currentTarget.style.backgroundColor = 'transparent'
                              }}
                            >
                              <TrashIcon size={12} />
                            </button>
                          </>
                        )}
                      </div>
                    </>
                  )}
                </div>

                {/* Conteúdo da Pasta (se não colapsada) */}
                {!isFolderCollapsed && (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 1, paddingLeft: 8 }}>
                    {visibleConvs.map((conv) => renderConversationItem(conv, true))}
                  </div>
                )}
              </div>
            )
          })}

          {/* Seção TODAS */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 1, marginTop: sortedFolderNames.length > 0 ? 4 : 0 }}>
            <div
              style={{
                fontSize: 12,
                fontWeight: 500,
                color: 'var(--text-muted)',
                padding: '6px 8px',
                textTransform: 'uppercase',
                letterSpacing: '0.04em'
              }}
            >
              Todas
            </div>

            {(() => {
              const visibleUnfiled = unfiledConversations.filter(matchFilter)
              if (visibleUnfiled.length === 0) {
                if (!hasAnyConversation) {
                  return (
                    <div style={{ fontSize: 13, color: 'var(--text-muted)', padding: '12px 10px', fontStyle: 'italic' }}>
                      Nenhuma conversa recente
                    </div>
                  )
                }
                if (filterText.trim()) {
                  return (
                    <div style={{ fontSize: 13, color: 'var(--text-muted)', padding: '12px 10px', fontStyle: 'italic' }}>
                      Nenhuma conversa encontrada
                    </div>
                  )
                }
                return (
                  <div style={{ fontSize: 12, color: 'var(--text-muted)', padding: '8px 10px', fontStyle: 'italic' }}>
                    Nenhuma conversa sem pasta
                  </div>
                )
              }
              return visibleUnfiled.map((conv) => renderConversationItem(conv, false))
            })()}
          </div>
        </div>
      </div>

      {/* Rodapé condicional: avatar de iniciais se email != null + engrenagem + tema */}
      <AgentFooter
        agentInfo={agentInfo}
        theme={theme}
        onToggleTheme={onToggleTheme}
        onOpenSettings={onOpenSettings}
      />
    </aside>
  )
}
