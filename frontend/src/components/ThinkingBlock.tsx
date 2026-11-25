import React, { useState } from 'react'
import { BrainIcon, ChevronRightIcon } from './icons'

interface ThinkingBlockProps {
  thinking: string
  isStreaming?: boolean
}

export const ThinkingBlock: React.FC<ThinkingBlockProps> = ({ thinking, isStreaming }) => {
  const [isOpen, setIsOpen] = useState(false)

  if (!thinking.trim()) return null

  return (
    <div
      style={{
        margin: '8px 0',
        borderRadius: 8,
        border: '1px solid var(--border-card)',
        backgroundColor: 'var(--bg-card)',
        overflow: 'hidden'
      }}
    >
      <button
        onClick={() => setIsOpen(!isOpen)}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          width: '100%',
          padding: '8px 12px',
          color: 'var(--text-muted)',
          fontSize: 13,
          fontStyle: 'italic',
          textAlign: 'left'
        }}
      >
        <span
          style={{
            transform: isOpen ? 'rotate(90deg)' : 'none',
            transition: 'transform 0.15s',
            display: 'flex',
            alignItems: 'center'
          }}
        >
          <ChevronRightIcon size={14} />
        </span>
        <BrainIcon size={16} color="var(--accent-blue)" />
        <span>{isStreaming ? 'Pensando...' : 'Processo de raciocínio (thinking)'}</span>
      </button>

      {isOpen && (
        <div
          style={{
            padding: '10px 14px',
            borderTop: '1px solid var(--border-subtle)',
            fontSize: 13,
            lineHeight: 1.5,
            color: 'var(--text-secondary)',
            fontStyle: 'italic',
            whiteSpace: 'pre-wrap'
          }}
        >
          {thinking}
        </div>
      )}
    </div>
  )
}
