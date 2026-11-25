import React, { useState } from 'react'
import { CheckIcon, CopyIcon } from './icons'

interface CodeBlockProps {
  language: string
  value: string
}

export const CodeBlock: React.FC<CodeBlockProps> = ({ language, value }) => {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // fallback copy
    }
  }

  return (
    <div
      style={{
        margin: '12px 0',
        borderRadius: 8,
        border: '1px solid var(--border-card)',
        backgroundColor: 'var(--bg-code)',
        overflow: 'hidden'
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '6px 12px',
          backgroundColor: 'var(--bg-code-header)',
          borderBottom: '1px solid var(--border-card)',
          fontSize: 12,
          color: 'var(--text-muted)',
          fontFamily: 'var(--font-mono)'
        }}
      >
        <span>{language || 'text'}</span>
        <button
          onClick={handleCopy}
          title="Copiar código"
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 6,
            fontSize: 12,
            color: copied ? 'var(--status-completed)' : 'var(--text-secondary)',
            padding: '3px 8px',
            borderRadius: 4,
            transition: 'color 0.15s'
          }}
        >
          {copied ? <CheckIcon size={14} /> : <CopyIcon size={14} />}
          <span>{copied ? 'Copiado!' : 'Copiar'}</span>
        </button>
      </div>

      <pre
        style={{
          padding: '12px 14px',
          overflowX: 'auto',
          margin: 0,
          fontFamily: 'var(--font-mono)',
          fontSize: 13,
          lineHeight: 1.5,
          color: 'var(--text-primary)'
        }}
      >
        <code>{value}</code>
      </pre>
    </div>
  )
}
