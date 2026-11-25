import React, { useState, useRef, useEffect } from 'react'
import { MoreVerticalIcon, PenIcon, SpinnerIcon, TrashIcon } from './icons'
import { Conversation } from '../types'

interface ConversationListProps {
  conversations: Conversation[]
  activeConversationId: string | null
  processingConversationIds: Set<string>
  filterText: string
  onSelectConversation: (id: string) => void
  onDeleteConversation: (id: string) => void
  onRenameConversation: (id: string, newTitle: string) => Promise<void>
}

export const ConversationList: React.FC<ConversationListProps> = ({
  conversations,
  activeConversationId,
  processingConversationIds,
  filterText,
  onSelectConversation,
  onDeleteConversation,
  onRenameConversation
}) => {
  const [hoveredId, setHoveredId] = useState<string | null>(null)
  const [openMenuId, setOpenMenuId] = useState<string | null>(null)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editTitle, setEditTitle] = useState<string>('')
  const [toastMessage, setToastMessage] = useState<string | null>(null)

  const menuRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  // Foco e seleção completa ao entrar no modo de edição
  useEffect(() => {
    if (editingId && inputRef.current) {
      inputRef.current.focus()
      inputRef.current.select()
    }
  }, [editingId])

  // Fechar menu com clique fora ou Esc
  useEffect(() => {
    const handleGlobalClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setOpenMenuId(null)
      }
    }
    const handleGlobalKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpenMenuId(null)
        if (editingId) {
          setEditingId(null)
        }
      }
    }

    document.addEventListener('mousedown', handleGlobalClick)
    document.addEventListener('keydown', handleGlobalKeyDown)
    return () => {
      document.removeEventListener('mousedown', handleGlobalClick)
      document.removeEventListener('keydown', handleGlobalKeyDown)
    }
  }, [editingId])

  const handleSaveRename = async (id: string) => {
    const trimmed = editTitle.trim()
    const targetConv = conversations.find((c) => c.id === id)
    if (!trimmed || trimmed === (targetConv?.title?.trim() || 'Nova conversa')) {
      setEditingId(null)
      return
    }

    setEditingId(null)
    try {
      await onRenameConversation(id, trimmed)
    } catch {
      setToastMessage('Falha ao renomear conversa')
      setTimeout(() => setToastMessage(null), 3000)
    }
  }

  const filtered = conversations.filter((c) => {
    if (!filterText.trim()) return true
    const q = filterText.toLowerCase()
    const titleMatch = (c.title || 'Nova conversa').toLowerCase().includes(q)
    return titleMatch
  })

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 1, padding: '4px 10px', flex: 1, overflowY: 'auto' }}>
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

      {/* Toast inline de erro */}
      {toastMessage && (
        <div
          style={{
            margin: '4px 6px 8px 6px',
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

      {filtered.length === 0 ? (
        <div style={{ fontSize: 13, color: 'var(--text-muted)', padding: '12px 10px', fontStyle: 'italic' }}>
          {conversations.length === 0 ? 'Nenhuma conversa recente' : 'Nenhuma conversa encontrada'}
        </div>
      ) : (
        filtered.map((conv) => {
          const isActive = activeConversationId === conv.id
          const isHovered = hoveredId === conv.id
          const isMenuOpen = openMenuId === conv.id
          const isEditing = editingId === conv.id
          const isProcessing = processingConversationIds.has(conv.id)
          const title = conv.title?.trim() || 'Nova conversa'

          return (
            <div
              key={conv.id}
              onMouseEnter={() => setHoveredId(conv.id)}
              onMouseLeave={() => setHoveredId(null)}
              style={{
                position: 'relative',
                display: 'flex',
                alignItems: 'center',
                width: '100%',
                borderRadius: 8,
                backgroundColor: isActive
                  ? 'var(--bg-sidebar-active)'
                  : isHovered || isMenuOpen
                  ? 'var(--bg-sidebar-hover)'
                  : 'transparent',
                transition: 'background 0.15s'
              }}
            >
              {isEditing ? (
                <div style={{ flex: 1, padding: '4px 6px', display: 'flex' }}>
                  <input
                    ref={inputRef}
                    type="text"
                    maxLength={80}
                    value={editTitle}
                    onChange={(e) => setEditTitle(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        e.preventDefault()
                        handleSaveRename(conv.id)
                      } else if (e.key === 'Escape') {
                        e.preventDefault()
                        setEditingId(null)
                      }
                    }}
                    onBlur={() => handleSaveRename(conv.id)}
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
                      padding: '9px 10px',
                      paddingRight: isHovered || isMenuOpen ? 34 : 10,
                      fontSize: 13,
                      fontWeight: isActive ? 500 : 400,
                      color: isActive ? 'var(--text-primary)' : 'var(--text-secondary)',
                      textAlign: 'left',
                      whiteSpace: 'nowrap',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis'
                    }}
                  >
                    {/* Spinner de processamento se a conversa tiver turno em andamento */}
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

                  {/* Botão ⋮ no hover do item */}
                  {(isHovered || isMenuOpen) && (
                    <button
                      onClick={(e) => {
                        e.stopPropagation()
                        setOpenMenuId(isMenuOpen ? null : conv.id)
                      }}
                      title="Opções da conversa"
                      style={{
                        position: 'absolute',
                        right: 6,
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        width: 24,
                        height: 24,
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
                      <MoreVerticalIcon size={15} />
                    </button>
                  )}

                  {/* Menu Dropdown de Contexto (Renomear / Excluir) */}
                  {isMenuOpen && (
                    <div
                      ref={menuRef}
                      style={{
                        position: 'absolute',
                        right: 6,
                        top: 30,
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
                          setEditingId(conv.id)
                          setEditTitle(conv.title || 'Nova conversa')
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
        })
      )}
    </div>
  )
}
