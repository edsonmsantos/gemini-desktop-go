import React, { useState, useRef, useEffect } from 'react'
import { ChevronDownIcon } from './icons'
import { Model } from '../types'

interface ModelSelectProps {
  models: Model[]
  selectedModelId: string
  onSelectModel: (id: string) => void
  disabled?: boolean
}

export const ModelSelect: React.FC<ModelSelectProps> = ({
  models,
  selectedModelId,
  onSelectModel,
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

  const currentModel = models.find((m) => m.id === selectedModelId) || models[0]
  const displayName = currentModel?.name || currentModel?.id || 'Modelo'

  return (
    <div ref={containerRef} style={{ position: 'relative', display: 'inline-block' }}>
      <button
        onClick={() => !disabled && setIsOpen(!isOpen)}
        disabled={disabled || models.length === 0}
        title="Selecionar modelo de IA"
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
            right: 0,
            marginBottom: 8,
            minWidth: 200,
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
          {models.map((model) => {
            const isSelected = model.id === currentModel?.id
            return (
              <button
                key={model.id}
                onClick={() => {
                  onSelectModel(model.id)
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
                  transition: 'background 0.15s'
                }}
                onMouseEnter={(e) => {
                  if (!isSelected) e.currentTarget.style.backgroundColor = 'var(--bg-sidebar-hover)'
                }}
                onMouseLeave={(e) => {
                  if (!isSelected) e.currentTarget.style.backgroundColor = 'transparent'
                }}
              >
                <span style={{ fontSize: 13, fontWeight: isSelected ? 600 : 500 }}>
                  {model.name || model.id}
                </span>
                {model.description && (
                  <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>
                    {model.description}
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
