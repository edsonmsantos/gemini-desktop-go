import React from 'react'
import Markdown from 'react-markdown'
import { CodeBlock } from './CodeBlock'

interface MarkdownRendererProps {
  content: string
}

export const MarkdownRenderer: React.FC<MarkdownRendererProps> = ({ content }) => {
  return (
    <div style={{ lineHeight: 1.6, fontSize: 15, color: 'var(--text-primary)', wordBreak: 'break-word' }}>
      <Markdown
        components={{
          code(props) {
            const { className, children, ...rest } = props
            const match = /language-(\w+)/.exec(className || '')
            const isBlock = Boolean(match) || (typeof children === 'string' && children.includes('\n'))

            if (isBlock) {
              return (
                <CodeBlock
                  language={match ? match[1] : ''}
                  value={String(children).replace(/\n$/, '')}
                />
              )
            }

            return (
              <code
                {...rest}
                style={{
                  fontFamily: 'var(--font-mono)',
                  fontSize: '0.88em',
                  padding: '2px 5px',
                  borderRadius: 4,
                  backgroundColor: 'var(--bg-code)',
                  border: '1px solid var(--border-subtle)',
                  color: 'var(--text-primary)'
                }}
              >
                {children}
              </code>
            )
          },
          p({ children }) {
            return <p style={{ marginBottom: 12 }}>{children}</p>
          },
          ul({ children }) {
            return <ul style={{ paddingLeft: 24, marginBottom: 12 }}>{children}</ul>
          },
          ol({ children }) {
            return <ol style={{ paddingLeft: 24, marginBottom: 12 }}>{children}</ol>
          },
          li({ children }) {
            return <li style={{ marginBottom: 4 }}>{children}</li>
          },
          h1({ children }) {
            return <h1 style={{ fontSize: '1.4em', fontWeight: 600, margin: '16px 0 8px 0' }}>{children}</h1>
          },
          h2({ children }) {
            return <h2 style={{ fontSize: '1.25em', fontWeight: 600, margin: '14px 0 6px 0' }}>{children}</h2>
          },
          h3({ children }) {
            return <h3 style={{ fontSize: '1.1em', fontWeight: 600, margin: '12px 0 4px 0' }}>{children}</h3>
          },
          blockquote({ children }) {
            return (
              <blockquote
                style={{
                  borderLeft: '3px solid var(--accent-blue)',
                  paddingLeft: 12,
                  margin: '12px 0',
                  color: 'var(--text-muted)'
                }}
              >
                {children}
              </blockquote>
            )
          },
          a({ href, children }) {
            return (
              <a
                href={href}
                target="_blank"
                rel="noreferrer"
                style={{ color: 'var(--accent-blue)', textDecoration: 'underline' }}
              >
                {children}
              </a>
            )
          }
        }}
      >
        {content}
      </Markdown>
    </div>
  )
}
