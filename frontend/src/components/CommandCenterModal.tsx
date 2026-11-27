import React, { useState, useEffect, useRef } from 'react'
import { SearchIcon, CloseIcon, SpinnerIcon, PlusIcon, SettingsIcon, SunIcon, MoonIcon, ToolIcon, ZapIcon, GlobeIcon } from './icons'
import { SearchResult } from '../types'
import { searchConversations } from '../api'

interface CommandCenterModalProps {
  isOpen: boolean
  onClose: () => void
  onNewConversation: () => void
  onToggleTheme: () => void
  onOpenSettings: (tab?: 'general' | 'mcp' | 'skills' | 'prompts' | 'browser') => void
  onSelectConversation: (id: string) => void
  theme: 'dark' | 'light'
}

type CommandItemType = 'command' | 'conversation'

interface PaletteItem {
  id: string
  type: CommandItemType
  title: string
  subtitle?: string
  icon: React.ReactNode
  onExecute: () => void
}

export const CommandCenterModal: React.FC<CommandCenterModalProps> = ({
  isOpen,
  onClose,
  onNewConversation,
  onToggleTheme,
  onOpenSettings,
  onSelectConversation,
  theme
}) => {
  const [query, setQuery] = useState('')
  const [searchResults, setSearchResults] = useState<SearchResult[]>([])
  const [isSearching, setIsSearching] = useState(false)
  const [selectedIndex, setSelectedIndex] = useState(0)

  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLDivElement>(null)

  // Auto-foco ao abrir
  useEffect(() => {
    if (isOpen) {
      setQuery('')
      setSearchResults([])
      setSelectedIndex(0)
      setTimeout(() => {
        inputRef.current?.focus()
      }, 50)
    }
  }, [isOpen])

  // Debounce de 250ms na busca
  useEffect(() => {
    if (!query.trim()) {
      setSearchResults([])
      setIsSearching(false)
      return
    }

    setIsSearching(true)
    const timer = setTimeout(() => {
      searchConversations(query)
        .then((res) => {
          setSearchResults(res)
          setIsSearching(false)
        })
        .catch(() => {
          setSearchResults([])
          setIsSearching(false)
        })
    }, 250)

    return () => clearTimeout(timer)
  }, [query])

  // Comandos nativos do aplicativo
  const baseCommands: PaletteItem[] = [
    {
      id: 'cmd-new',
      type: 'command',
      title: 'Nova Conversa',
      subtitle: 'Iniciar uma nova conversa com workspace ou temporária',
      icon: <PlusIcon size={16} />,
      onExecute: () => {
        onClose()
        onNewConversation()
      }
    },
    {
      id: 'cmd-theme',
      type: 'command',
      title: theme === 'dark' ? 'Alternar para Tema Claro' : 'Alternar para Tema Escuro',
      subtitle: 'Mudar a aparência do aplicativo',
      icon: theme === 'dark' ? <SunIcon size={16} /> : <MoonIcon size={16} />,
      onExecute: () => {
        onClose()
        onToggleTheme()
      }
    },
    {
      id: 'cmd-settings',
      type: 'command',
      title: 'Configurações',
      subtitle: 'Porta, agente ACP e preferências gerais',
      icon: <SettingsIcon size={16} />,
      onExecute: () => {
        onClose()
        onOpenSettings('general')
      }
    },
    {
      id: 'cmd-mcp',
      type: 'command',
      title: 'MCP Servers',
      subtitle: 'Gerenciar servidores Model Context Protocol ativos e inativos',
      icon: <ToolIcon size={16} />,
      onExecute: () => {
        onClose()
        onOpenSettings('mcp')
      }
    },
    {
      id: 'cmd-skills',
      type: 'command',
      title: 'Skills do Antigravity',
      subtitle: 'Varredura de skills globais e do workspace (digite / ou $ no chat)',
      icon: <ZapIcon size={16} color="var(--accent-blue)" />,
      onExecute: () => {
        onClose()
      }
    },
    {
      id: 'cmd-browser',
      type: 'command',
      title: 'Integração do Navegador (Browser Bridge)',
      subtitle: 'Instalação da extensão, diagnóstico e estado da ponte de automação',
      icon: <GlobeIcon size={16} color="var(--accent-blue)" />,
      onExecute: () => {
        onClose()
        onOpenSettings('browser')
      }
    }
  ]

  // Filtra comandos pela query
  const qLower = query.toLowerCase().trim()
  const filteredCommands = baseCommands.filter(
    (c) =>
      c.title.toLowerCase().includes(qLower) ||
      (c.subtitle && c.subtitle.toLowerCase().includes(qLower))
  )

  // Mapeia conversas da busca em PaletteItem
  const conversationItems: PaletteItem[] = searchResults.map((r) => ({
    id: `conv-${r.conversationId}`,
    type: 'conversation',
    title: r.title || 'Conversa sem título',
    subtitle: r.snippet,
    icon: <SearchIcon size={16} />,
    onExecute: () => {
      onClose()
      onSelectConversation(r.conversationId)
    }
  }))

  // Lista unificada para navegação por teclado
  const displayItems = [...filteredCommands, ...conversationItems]

  // Reset do índice quando muda a lista
  useEffect(() => {
    setSelectedIndex(0)
  }, [query, searchResults])

  // Navegação por teclado (↑, ↓, Enter, Esc)
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      if (displayItems.length > 0) {
        setSelectedIndex((prev) => (prev + 1) % displayItems.length)
      }
      return
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault()
      if (displayItems.length > 0) {
        setSelectedIndex((prev) => (prev - 1 + displayItems.length) % displayItems.length)
      }
      return
    }
    if (e.key === 'Enter') {
      e.preventDefault()
      if (displayItems.length > 0 && displayItems[selectedIndex]) {
        displayItems[selectedIndex].onExecute()
      }
      return
    }
    if (e.key === 'Escape') {
      e.preventDefault()
      onClose()
    }
  }

  if (!isOpen) return null

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0, 0, 0, 0.65)',
        display: 'flex',
        alignItems: 'flex-start',
        justifyContent: 'center',
        paddingTop: '12vh',
        zIndex: 9999,
        backdropFilter: 'blur(3px)'
      }}
      onClick={onClose}
    >
      <div
        style={{
          width: 580,
          maxWidth: '92vw',
          maxHeight: '75vh',
          backgroundColor: 'var(--bg-modal)',
          borderRadius: 16,
          border: '1px solid var(--border-card)',
          boxShadow: '0 16px 40px rgba(0, 0, 0, 0.5)',
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden'
        }}
        onClick={(e) => e.stopPropagation()}
        onKeyDown={handleKeyDown}
      >
        {/* Barra de entrada de pesquisa */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 12,
            padding: '14px 18px',
            borderBottom: '1px solid var(--border-subtle)'
          }}
        >
          <SearchIcon size={18} color="var(--text-muted)" />
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Digite um comando ou busque nas conversas..."
            style={{
              flex: 1,
              background: 'none',
              border: 'none',
              outline: 'none',
              fontSize: 15,
              color: 'var(--text-primary)',
              fontFamily: 'inherit'
            }}
          />
          {isSearching && <SpinnerIcon size={16} />}
          <button
            type="button"
            onClick={onClose}
            style={{
              padding: 4,
              borderRadius: '50%',
              color: 'var(--text-muted)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              cursor: 'pointer'
            }}
          >
            <CloseIcon size={16} />
          </button>
        </div>

        {/* Lista de Comandos e Resultados */}
        <div
          ref={listRef}
          style={{
            padding: '8px',
            display: 'flex',
            flexDirection: 'column',
            gap: 2,
            overflowY: 'auto',
            maxHeight: 380
          }}
        >
          {displayItems.length === 0 ? (
            <div
              style={{
                padding: '24px 16px',
                textAlign: 'center',
                color: 'var(--text-muted)',
                fontSize: 13
              }}
            >
              Nenhum comando ou conversa encontrada
            </div>
          ) : (
            displayItems.map((item, idx) => {
              const isSelected = idx === selectedIndex
              const isCommand = item.type === 'command'

              return (
                <div
                  key={item.id}
                  onClick={item.onExecute}
                  onMouseEnter={() => setSelectedIndex(idx)}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    gap: 12,
                    padding: '8px 12px',
                    borderRadius: 8,
                    cursor: 'pointer',
                    backgroundColor: isSelected ? 'var(--bg-pill-hover)' : 'transparent',
                    transition: 'background-color 0.1s'
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0, flex: 1 }}>
                    <div style={{ color: isSelected ? 'var(--accent-blue)' : 'var(--text-muted)', flexShrink: 0 }}>
                      {item.icon}
                    </div>

                    <div style={{ display: 'flex', flexDirection: 'column', minWidth: 0, flex: 1 }}>
                      <span
                        style={{
                          fontSize: 13,
                          fontWeight: 500,
                          color: 'var(--text-primary)',
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap'
                        }}
                      >
                        {item.title}
                      </span>
                      {item.subtitle && (
                        <span
                          style={{
                            fontSize: 11,
                            color: 'var(--text-muted)',
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap',
                            marginTop: 1
                          }}
                          title={item.subtitle}
                        >
                          {item.subtitle}
                        </span>
                      )}
                    </div>
                  </div>

                  {/* Badge do tipo */}
                  <span
                    style={{
                      fontSize: 10,
                      fontWeight: 600,
                      textTransform: 'uppercase',
                      padding: '2px 6px',
                      borderRadius: 4,
                      backgroundColor: isCommand ? 'rgba(66, 133, 244, 0.12)' : 'var(--bg-pill)',
                      color: isCommand ? 'var(--accent-blue)' : 'var(--text-muted)',
                      flexShrink: 0
                    }}
                  >
                    {isCommand ? 'Comando' : 'Conversa'}
                  </span>
                </div>
              )
            })
          )}
        </div>

        {/* Dica de Teclado no Rodapé */}
        <div
          style={{
            padding: '8px 14px',
            borderTop: '1px solid var(--border-subtle)',
            backgroundColor: 'var(--bg-sidebar)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            fontSize: 11,
            color: 'var(--text-muted)'
          }}
        >
          <span>Navegue com ↑ / ↓ e pressione Enter para executar</span>
          <span>Esc para fechar</span>
        </div>
      </div>
    </div>
  )
}
