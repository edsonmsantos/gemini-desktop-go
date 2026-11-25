import React, { useState, useEffect } from 'react'
import { Artifact, FileContent } from '../types'
import { fetchFileContent, openExternalURL } from '../api'
import { CloseIcon, ExternalLinkIcon, SpinnerIcon } from './icons'

interface ArtifactViewerModalProps {
  artifact: Artifact
  onClose: () => void
}

const formatSize = (bytes: number): string => {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

const artifactIcon = (name: string): string => {
  const ext = name.split('.').pop()?.toLowerCase() || ''
  if (['png', 'jpg', 'jpeg', 'webp', 'gif', 'bmp', 'svg'].includes(ext)) return '🖼️'
  if (ext === 'pdf') return '📕'
  if (['xls', 'xlsx', 'csv', 'ods'].includes(ext)) return '📊'
  if (['doc', 'docx', 'odt', 'rtf'].includes(ext)) return '📝'
  if (['mp4', 'avi', 'mkv', 'mov', 'webm'].includes(ext)) return '🎬'
  if (['mp3', 'wav', 'ogg', 'flac'].includes(ext)) return '🎵'
  if (['zip', '7z', 'tar', 'gz', 'rar'].includes(ext)) return '🗜️'
  if (['json', 'xml', 'yaml', 'yml', 'toml'].includes(ext)) return '⚙️'
  if (['py', 'js', 'ts', 'go', 'rs', 'java', 'c', 'cpp', 'h', 'sh', 'html', 'css', 'sql'].includes(ext)) return '💻'
  if (['md', 'txt', 'log', 'ini', 'conf'].includes(ext)) return '📄'
  return '📄'
}

export const ArtifactViewerModal: React.FC<ArtifactViewerModalProps> = ({ artifact, onClose }) => {
  const [content, setContent] = useState<FileContent | null>(null)
  const [loading, setLoading] = useState<boolean>(true)
  const [error, setError] = useState<boolean>(false)

  useEffect(() => {
    let active = true
    setLoading(true)
    setError(false)

    fetchFileContent(artifact.path)
      .then((res) => {
        if (!active) return
        if (!res) {
          setError(true)
        } else {
          setContent(res)
        }
        setLoading(false)
      })
      .catch(() => {
        if (!active) return
        setError(true)
        setLoading(false)
      })

    return () => {
      active = false
    }
  }, [artifact.path])

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [onClose])

  const handleOpenNative = () => {
    void openExternalURL(artifact.path)
  }

  const renderBody = () => {
    if (loading) {
      return (
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            padding: '48px 16px',
            gap: 12
          }}
        >
          <SpinnerIcon size={24} className="animate-spin" />
          <span style={{ fontSize: 13, color: 'var(--text-muted)' }}>Carregando visualização...</span>
        </div>
      )
    }

    if (error || !content) {
      return (
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            padding: '48px 16px',
            gap: 12,
            textAlign: 'center'
          }}
        >
          <span style={{ fontSize: 14, color: 'var(--text-secondary)' }}>
            Não foi possível carregar a pré-visualização deste arquivo.
          </span>
          <button
            type="button"
            onClick={handleOpenNative}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 6,
              padding: '6px 14px',
              borderRadius: 8,
              backgroundColor: 'var(--bg-pill)',
              border: '1px solid var(--border-card)',
              color: 'var(--text-primary)',
              fontSize: 13,
              cursor: 'pointer'
            }}
          >
            <ExternalLinkIcon size={14} />
            Abrir no app padrão
          </button>
        </div>
      )
    }

    if (content.kind === 'image') {
      const dataUrl = content.content.startsWith('data:')
        ? content.content
        : `data:${content.mime};base64,${content.content}`

      return (
        <div
          style={{
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            padding: 16,
            backgroundColor: 'var(--bg-card)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 8,
            overflow: 'auto',
            maxHeight: '60vh'
          }}
        >
          <img
            src={dataUrl}
            alt={artifact.name}
            style={{
              maxWidth: '100%',
              maxHeight: '55vh',
              objectFit: 'contain',
              borderRadius: 4
            }}
          />
        </div>
      )
    }

    // Texto
    return (
      <pre
        style={{
          fontFamily: 'var(--font-mono, monospace)',
          whiteSpace: 'pre-wrap',
          maxHeight: '55vh',
          overflow: 'auto',
          margin: 0,
          padding: 16,
          backgroundColor: 'var(--bg-card)',
          border: '1px solid var(--border-subtle)',
          borderRadius: 8,
          fontSize: 13,
          lineHeight: 1.5,
          color: 'var(--text-primary)'
        }}
      >
        {content.content}
      </pre>
    )
  }

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0, 0, 0, 0.65)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 9999,
        backdropFilter: 'blur(3px)'
      }}
      onClick={onClose}
    >
      <div
        style={{
          width: 760,
          maxWidth: '92vw',
          maxHeight: '88vh',
          backgroundColor: 'var(--bg-modal)',
          borderRadius: 16,
          border: '1px solid var(--border-card)',
          boxShadow: '0 16px 40px rgba(0,0,0,0.5)',
          padding: 20,
          display: 'flex',
          flexDirection: 'column',
          gap: 16,
          overflow: 'hidden'
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 12,
            borderBottom: '1px solid var(--border-subtle)',
            paddingBottom: 14
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 }}>
            <span style={{ fontSize: 20, flexShrink: 0 }}>{artifactIcon(artifact.name)}</span>
            <div style={{ display: 'flex', flexDirection: 'column', minWidth: 0 }}>
              <span
                style={{
                  fontSize: 15,
                  fontWeight: 600,
                  color: 'var(--text-primary)',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap'
                }}
                title={artifact.name}
              >
                {artifact.name}
              </span>
              <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>
                {formatSize(artifact.size)}
              </span>
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexShrink: 0 }}>
            <button
              type="button"
              onClick={handleOpenNative}
              title="Abrir no aplicativo padrão do sistema"
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: 6,
                padding: '6px 12px',
                borderRadius: 8,
                backgroundColor: 'var(--bg-pill)',
                border: '1px solid var(--border-card)',
                color: 'var(--text-secondary)',
                fontSize: 12,
                fontWeight: 500,
                cursor: 'pointer'
              }}
            >
              <ExternalLinkIcon size={14} />
              <span>Abrir no app padrão</span>
            </button>

            <button
              type="button"
              onClick={onClose}
              title="Fechar"
              style={{
                padding: 6,
                borderRadius: '50%',
                color: 'var(--text-secondary)',
                backgroundColor: 'transparent',
                border: 'none',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                cursor: 'pointer'
              }}
            >
              <CloseIcon size={18} />
            </button>
          </div>
        </div>

        {/* Corpo */}
        <div style={{ flex: 1, minHeight: 0, overflow: 'auto' }}>
          {renderBody()}
        </div>
      </div>
    </div>
  )
}
