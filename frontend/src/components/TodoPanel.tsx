import React, { useState } from 'react'
import { ListTodoIcon, ChevronDownIcon, ChevronRightIcon, SpinnerIcon } from './icons'
import { Plan } from '../types'

interface TodoPanelProps {
  plan: Plan | null
  isStreaming: boolean
}

export const TodoPanel: React.FC<TodoPanelProps> = ({ plan, isStreaming }) => {
  const [isOpen, setIsOpen] = useState(false)

  if (!plan || !plan.entries || plan.entries.length === 0) {
    return null
  }

  const completedCount = plan.entries.filter(
    (e) => e.status === 'completed' || e.status === 'done'
  ).length
  const totalCount = plan.entries.length
  const isAllCompleted = completedCount === totalCount

  // Painel inteiro some quando tudo estiver completado
  if (isAllCompleted) {
    return null
  }

  // Lista mostra SOMENTE itens pending / in_progress (completos somem)
  const visibleEntries = plan.entries.filter(
    (e) => e.status !== 'completed' && e.status !== 'done'
  )

  const activeEntry =
    plan.entries.find((e) => e.status === 'in_progress') || visibleEntries[0]

  return (
    <div
      style={{
        position: 'sticky',
        top: 0,
        zIndex: 20,
        width: '100%',
        maxWidth: 840,
        margin: '0 auto 12px auto',
        borderRadius: 12,
        backgroundColor: 'var(--bg-card)',
        border: '1px solid var(--border-card)',
        boxShadow: 'var(--shadow-prompt)',
        overflow: 'hidden',
        backdropFilter: 'blur(8px)'
      }}
    >
      {/* Header "Tarefas x/y" + chevron */}
      <button
        type="button"
        onClick={() => setIsOpen((prev) => !prev)}
        style={{
          width: '100%',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '10px 14px',
          background: 'none',
          border: 'none',
          cursor: 'pointer',
          textAlign: 'left'
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0, flex: 1 }}>
          <ListTodoIcon size={16} color="var(--accent-blue)" />
          <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>
            Tarefas
          </span>
          <span
            style={{
              fontSize: 12,
              fontFamily: 'var(--font-mono)',
              color: 'var(--text-muted)',
              backgroundColor: 'var(--bg-pill)',
              padding: '1px 6px',
              borderRadius: 8
            }}
          >
            {completedCount}/{totalCount}
          </span>

          {/* Tarefa ativa exibida resumidamente quando colapsado */}
          {!isOpen && activeEntry && (
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, minWidth: 0, flex: 1, marginLeft: 6 }}>
              {isStreaming && activeEntry.status === 'in_progress' && (
                <SpinnerIcon size={12} color="var(--status-progress)" />
              )}
              <span
                style={{
                  fontSize: 12,
                  color: isStreaming && activeEntry.status === 'in_progress' ? 'var(--text-primary)' : 'var(--text-secondary)',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap'
                }}
              >
                {activeEntry.content}
              </span>
            </div>
          )}
        </div>

        <div style={{ display: 'flex', alignItems: 'center', color: 'var(--text-muted)' }}>
          {isOpen ? <ChevronDownIcon size={16} /> : <ChevronRightIcon size={16} />}
        </div>
      </button>

      {/* Lista expandida (apenas itens pendentes/em execução) */}
      {isOpen && (
        <div
          style={{
            padding: '4px 14px 12px 14px',
            borderTop: '1px solid var(--border-subtle)',
            display: 'flex',
            flexDirection: 'column',
            gap: 8,
            maxHeight: 220,
            overflowY: 'auto'
          }}
        >
          {visibleEntries.map((entry, idx) => {
            const isInProgress = entry.status === 'in_progress'
            return (
              <div
                key={idx}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 10,
                  fontSize: 13,
                  padding: '4px 0'
                }}
              >
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    width: 16,
                    height: 16,
                    flexShrink: 0
                  }}
                >
                  {isInProgress ? (
                    <SpinnerIcon size={14} color="var(--status-progress)" />
                  ) : (
                    <div
                      style={{
                        width: 8,
                        height: 8,
                        borderRadius: '50%',
                        backgroundColor: 'var(--border-card)'
                      }}
                    />
                  )}
                </div>
                <span
                  style={{
                    color: isInProgress ? 'var(--text-primary)' : 'var(--text-secondary)',
                    fontWeight: isInProgress ? 500 : 400,
                    lineHeight: 1.4
                  }}
                >
                  {entry.content}
                </span>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
