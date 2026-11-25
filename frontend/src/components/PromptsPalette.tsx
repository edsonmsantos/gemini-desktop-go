import React, { useRef, useEffect } from 'react'
import { SavedPrompt } from '../types'
import { ClipboardIcon, CloseIcon, SpinnerIcon } from './icons'

interface PromptItemRowProps {
  prompt: SavedPrompt
  isSelected: boolean
  onSelect: () => void
}

const PromptItemRow: React.FC<PromptItemRowProps> = ({ prompt, isSelected, onSelect }) => {
  const rowRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (isSelected && rowRef.current) {
      rowRef.current.scrollIntoView({ block: 'nearest' })
    }
  }, [isSelected])

  // Prévia do conteúdo em 1 linha (substituindo quebras de linha por espaços)
  const singleLinePreview = prompt.content.replace(/[\r\n]+/g, ' ').trim()

  return (
    <button
      ref={rowRef}
      type="button"
      onClick={onSelect}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 10,
        padding: '7px 12px',
        width: '100%',
        textAlign: 'left',
        backgroundColor: isSelected ? 'var(--bg-pill-hover)' : 'transparent',
        border: 'none',
        borderRadius: 8,
        cursor: 'pointer',
        transition: 'background-color 0.1s'
      }}
      onMouseEnter={(e) => {
        if (!isSelected) e.currentTarget.style.backgroundColor = 'var(--bg-sidebar-hover)'
      }}
      onMouseLeave={(e) => {
        if (!isSelected) e.currentTarget.style.backgroundColor = 'transparent'
      }}
    >
      <ClipboardIcon size={16} color={isSelected ? 'var(--accent-blue)' : 'var(--text-muted)'} />
      <div style={{ display: 'flex', flexDirection: 'column', minWidth: 0, flex: 1 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 6, minWidth: 0 }}>
          <span style={{ fontWeight: 600, fontSize: 13, color: 'var(--text-primary)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {prompt.title}
          </span>
        </div>
        {singleLinePreview && (
          <span
            style={{
              fontSize: 12,
              color: 'var(--text-muted)',
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              marginTop: 1
            }}
            title={prompt.content}
          >
            {singleLinePreview}
          </span>
        )}
      </div>
    </button>
  )
}

export interface PromptsPaletteProps {
  prompts: SavedPrompt[]
  isLoading?: boolean
  error?: string | null
  selectedIndex: number
  onSelectPrompt: (prompt: SavedPrompt) => void
  onClose: () => void
  paletteRef?: React.RefObject<HTMLDivElement | null>
}

export const PromptsPalette: React.FC<PromptsPaletteProps> = ({
  prompts,
  isLoading,
  error,
  selectedIndex,
  onSelectPrompt,
  onClose,
  paletteRef
}) => {
  return (
    <div
      ref={paletteRef}
      style={{
        position: 'absolute',
        bottom: 'calc(100% + 8px)',
        left: 14,
        width: 'calc(100% - 28px)',
        maxWidth: 380,
        maxHeight: 280,
        backgroundColor: 'var(--bg-dropdown)',
        border: '1px solid var(--border-card)',
        borderRadius: 14,
        boxShadow: 'var(--shadow-dropdown)',
        overflowY: 'auto',
        zIndex: 100,
        padding: '6px 8px',
        display: 'flex',
        flexDirection: 'column',
        gap: 2
      }}
    >
      {isLoading && (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 8,
            padding: '10px 12px',
            fontSize: 13,
            color: 'var(--text-muted)'
          }}
        >
          <SpinnerIcon size={14} /> Carregando prompts...
        </div>
      )}

      {error && (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '8px 12px',
            fontSize: 12,
            color: 'var(--status-failed)'
          }}
        >
          <span>{error}</span>
          <button
            type="button"
            onClick={onClose}
            style={{ color: 'var(--text-muted)', cursor: 'pointer', padding: 2 }}
          >
            <CloseIcon size={14} />
          </button>
        </div>
      )}

      {!isLoading && !error && prompts.length === 0 && (
        <div
          style={{
            padding: '12px 14px',
            fontSize: 13,
            color: 'var(--text-muted)',
            textAlign: 'center'
          }}
        >
          Nenhum prompt salvo encontrado
        </div>
      )}

      {!isLoading && !error && prompts.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          {prompts.map((p, idx) => (
            <PromptItemRow
              key={p.id}
              prompt={p}
              isSelected={idx === selectedIndex}
              onSelect={() => onSelectPrompt(p)}
            />
          ))}
        </div>
      )}
    </div>
  )
}
