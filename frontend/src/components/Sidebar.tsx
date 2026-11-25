import React, { useState } from 'react'
import { GeminiSparkle, PenIcon, SearchIcon, SidebarCollapseIcon } from './icons'
import { WorkspaceList } from './WorkspaceList'
import { ConversationList } from './ConversationList'
import { AgentFooter } from './AgentFooter'
import { AgentInfo, Conversation, Workspace } from '../types'

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
  onOpenSettings
}) => {
  const [isCollapsed, setIsCollapsed] = useState(false)
  const [filterText, setFilterText] = useState('')
  const [showSearchInput, setShowSearchInput] = useState(false)

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

      {/* Conteúdo rolável: Workspaces + Histórico */}
      <div style={{ flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 6 }}>
        <WorkspaceList
          workspaces={workspaces}
          selectedCwd={selectedCwd}
          onSelectCwd={onSelectCwd}
        />
        <ConversationList
          conversations={conversations}
          activeConversationId={activeConversationId}
          processingConversationIds={processingConversationIds}
          filterText={filterText}
          onSelectConversation={onSelectConversation}
          onDeleteConversation={onDeleteConversation}
          onRenameConversation={onRenameConversation}
        />
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
