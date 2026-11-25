import React, { useState, useRef, useEffect } from 'react'
import { ChevronDownIcon } from './icons'
import { ModesInfo } from '../types'

interface ModeSelectProps {
  modes?: ModesInfo | null
  selectedModeId?: string
  onSelectMode: (modeId: string) => void
  disabled?: boolean
}

export const ModeSelect: React.FC<ModeSelectProps> = ({
  modes,
  selectedModeId,
  onSelectMode,
  disabled
}) => {
  const [isOpen, setIsOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handleOutsideClick = (event: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setIsOpen(false)
      }
    }
    document.addEventListener('mousedown', handleOutsideClick)
    return () => document.removeEventListener('mousedown', handleOutsideClick)
  }, [])

  const available = modes?.availableModes || modes?.available || []
  if (!modes || available.length === 0) {
    return null
  }

  const activeId = selectedModeId || modes.currentModeId || modes.current
  const currentMode = available.find((m) => m.id === activeId) || available[0]
  const displayName = currentMode?.name || currentMode?.id || 'Modo'

  return (
    <div ref={containerRef} style={{ position: 'relative', display: 'inline-block' }}>
      <button
        onClick={() => !disabled && setIsOpen(!isOpen)}
        disabled={disabled}
        title={currentMode?.description || currentMode?.name || 'Selecionar modo de operação'}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          padding: '6px 12px',
          borderRadius: 16,
          backgroundColor: isOpen ? 'var(--bg-pill-hover)' : 'transparent',
          color: 'var(--text-secondary)',
          fontSize: 13,
          fontWeight: 500,
          cursor: disabled ? 'not-allowed' : 'pointer',
          transition: 'background 0.15s, color 0.15s'
        }}
        onMouseEnter={(e) => {
          if (!disabled) e.currentTarget.style.color = 'var(--text-primary)'
        }}
        onMouseLeave={(e) => {
          if (!disabled) e.currentTarget.style.color = 'var(--text-secondary)'
        }}
      >
        <span>{displayName}</span>
        <ChevronDownIcon size={14} />
      </button>

      {isOpen && (
        <div
          style={{
            position: 'absolute',
            bottom: '100%',
            left: 0,
            marginBottom: 8,
            minWidth: 220,
            maxWidth: 320,
            backgroundColor: 'var(--bg-dropdown)',
            borderRadius: 12,
            border: '1px solid var(--border-card)',
            boxShadow: 'var(--shadow-dropdown)',
            padding: 4,
            zIndex: 1000,
            display: 'flex',
            flexDirection: 'column',
            gap: 2
          }}
        >
          {available.map((mode) => {
            const isSelected = mode.id === currentMode?.id
            return (
              <button
                key={mode.id}
                title={mode.description || mode.name}
                onClick={() => {
                  onSelectMode(mode.id)
                  setIsOpen(false)
                }}
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: 'flex-start',
                  padding: '8px 12px',
                  borderRadius: 8,
                  backgroundColor: isSelected ? 'var(--bg-sidebar-active)' : 'transparent',
                  color: isSelected ? 'var(--text-primary)' : 'var(--text-secondary)',
                  textAlign: 'left',
                  width: '100%',
                  cursor: 'pointer',
                  transition: 'background 0.1s'
                }}
                onMouseEnter={(e) => {
                  if (!isSelected) e.currentTarget.style.backgroundColor = 'var(--bg-sidebar-hover)'
                }}
                onMouseLeave={(e) => {
                  if (!isSelected) e.currentTarget.style.backgroundColor = 'transparent'
                }}
              >
                <span style={{ fontSize: 13, fontWeight: isSelected ? 600 : 500 }}>
                  {mode.name || mode.id}
                </span>
                {mode.description && (
                  <span
                    style={{
                      fontSize: 11,
                      color: 'var(--text-muted)',
                      marginTop: 2,
                      lineHeight: 1.3
                    }}
                  >
                    {mode.description}
                  </span>
                )}
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}
