import React, { useState, useEffect } from 'react'
import { CloseIcon, FolderIcon } from './icons'
import { Workspace } from '../types'
import { selectDirectoryViaWails } from '../api'

interface NewConversationModalProps {
  isOpen: boolean
  onClose: () => void
  workspaces: Workspace[]
  onStartConversation: (cwd?: string) => void
}

export const NewConversationModal: React.FC<NewConversationModalProps> = ({
  isOpen,
  onClose,
  workspaces,
  onStartConversation
}) => {
  const [selectedType, setSelectedType] = useState<'temp' | 'recent' | 'custom'>('temp')
  const [selectedRecentCwd, setSelectedRecentCwd] = useState<string>(workspaces[0]?.cwd || '')
  const [customPath, setCustomPath] = useState<string>('')
  const [hasNativeDialog, setHasNativeDialog] = useState<boolean>(false)

  useEffect(() => {
    setHasNativeDialog(Boolean(window.go?.main?.App?.SelectDirectory))
  }, [])

  useEffect(() => {
    if (workspaces.length > 0 && !selectedRecentCwd) {
      setSelectedRecentCwd(workspaces[0].cwd)
    }
  }, [workspaces, selectedRecentCwd])

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      }
    }
    if (isOpen) {
      document.addEventListener('keydown', handleKeyDown)
    }
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [isOpen, onClose])

  if (!isOpen) return null

  const handleBrowseNative = async () => {
    const dir = await selectDirectoryViaWails()
    if (dir) {
      setCustomPath(dir)
      setSelectedType('custom')
    }
  }

  const handleConfirm = () => {
    if (selectedType === 'temp') {
      onStartConversation(undefined)
    } else if (selectedType === 'recent') {
      onStartConversation(selectedRecentCwd || undefined)
    } else {
      const trimmed = customPath.trim()
      onStartConversation(trimmed || undefined)
    }
    onClose()
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
          width: 540,
          maxWidth: '92vw',
          maxHeight: '88vh',
          backgroundColor: 'var(--bg-modal)',
          borderRadius: 16,
          border: '1px solid var(--border-card)',
          boxShadow: '0 16px 40px rgba(0,0,0,0.5)',
          padding: 24,
          display: 'flex',
          flexDirection: 'column',
          gap: 18,
          overflowY: 'auto'
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div>
            <h2 style={{ fontSize: 18, fontWeight: 500, color: 'var(--text-primary)' }}>Nova Conversa</h2>
            <p style={{ fontSize: 13, color: 'var(--text-muted)', marginTop: 2 }}>
              Escolha a pasta de trabalho ou inicie uma conversa temporária
            </p>
          </div>
          <button
            onClick={onClose}
            style={{
              padding: 6,
              borderRadius: '50%',
              color: 'var(--text-secondary)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center'
            }}
          >
            <CloseIcon size={18} />
          </button>
        </div>

        {/* Opção 1: Sem workspace (temporária) */}
        <div
          onClick={() => setSelectedType('temp')}
          style={{
            padding: '12px 14px',
            borderRadius: 12,
            border: `2px solid ${selectedType === 'temp' ? 'var(--accent-blue)' : 'var(--border-subtle)'}`,
            backgroundColor: selectedType === 'temp' ? 'var(--bg-sidebar-active)' : 'var(--bg-card)',
            cursor: 'pointer',
            transition: 'border-color 0.15s, background 0.15s'
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <div
              style={{
                width: 16,
                height: 16,
                borderRadius: '50%',
                border: `2px solid ${selectedType === 'temp' ? 'var(--accent-blue)' : 'var(--border-card)'}`,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center'
              }}
            >
              {selectedType === 'temp' && (
                <div style={{ width: 8, height: 8, borderRadius: '50%', backgroundColor: 'var(--accent-blue)' }} />
              )}
            </div>
            <div>
              <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--text-primary)' }}>
                Sem workspace (temporária)
              </span>
              <p style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 2 }}>
                Cria uma pasta temporária isolada para esta conversa
              </p>
            </div>
          </div>
        </div>

        {/* Opção 2: Workspaces Recentes */}
        {workspaces.length > 0 && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <label style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
              Workspaces Recentes
            </label>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6, maxHeight: 160, overflowY: 'auto' }}>
              {workspaces.map((ws) => {
                const isSelected = selectedType === 'recent' && selectedRecentCwd === ws.cwd
                const basename = ws.cwd.split(/[/\\]/).filter(Boolean).pop() || ws.cwd

                return (
                  <div
                    key={ws.cwd}
                    onClick={() => {
                      setSelectedRecentCwd(ws.cwd)
                      setSelectedType('recent')
                    }}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 10,
                      padding: '8px 12px',
                      borderRadius: 8,
                      border: `1px solid ${isSelected ? 'var(--accent-blue)' : 'var(--border-subtle)'}`,
                      backgroundColor: isSelected ? 'var(--bg-sidebar-active)' : 'transparent',
                      cursor: 'pointer',
                      transition: 'background 0.15s'
                    }}
                  >
                    <FolderIcon size={16} color={isSelected ? 'var(--accent-blue)' : 'var(--text-muted)'} />
                    <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
                      <span style={{ fontSize: 13, fontWeight: 500, color: 'var(--text-primary)' }}>
                        {basename}
                      </span>
                      <span
                        style={{
                          fontSize: 11,
                          color: 'var(--text-muted)',
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap'
                        }}
                      >
                        {ws.cwd}
                      </span>
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        )}

        {/* Opção 3: Outro diretório */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <label style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
            Outro Diretório
          </label>
          <div style={{ display: 'flex', gap: 8 }}>
            <input
              type="text"
              placeholder="/caminho/absoluto/do/projeto"
              value={customPath}
              onChange={(e) => {
                setCustomPath(e.target.value)
                setSelectedType('custom')
              }}
              onFocus={() => setSelectedType('custom')}
              style={{
                flex: 1,
                padding: '9px 12px',
                borderRadius: 8,
                backgroundColor: 'var(--bg-card)',
                border: `1px solid ${selectedType === 'custom' ? 'var(--accent-blue)' : 'var(--border-card)'}`,
                fontSize: 13,
                color: 'var(--text-primary)'
              }}
            />
            {hasNativeDialog && (
              <button
                onClick={handleBrowseNative}
                style={{
                  padding: '9px 14px',
                  borderRadius: 8,
                  backgroundColor: 'var(--bg-pill)',
                  border: '1px solid var(--border-card)',
                  fontSize: 13,
                  fontWeight: 500,
                  color: 'var(--text-primary)',
                  whiteSpace: 'nowrap',
                  transition: 'background 0.15s'
                }}
              >
                Procurar...
              </button>
            )}
          </div>
        </div>

        {/* Rodapé do Modal */}
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10, marginTop: 10 }}>
          <button
            onClick={onClose}
            style={{
              padding: '8px 16px',
              borderRadius: 20,
              backgroundColor: 'transparent',
              border: '1px solid var(--border-card)',
              color: 'var(--text-secondary)',
              fontSize: 13,
              fontWeight: 500
            }}
          >
            Cancelar
          </button>
          <button
            onClick={handleConfirm}
            style={{
              padding: '8px 22px',
              borderRadius: 20,
              backgroundColor: 'var(--accent-blue)',
              color: '#ffffff',
              fontSize: 13,
              fontWeight: 500,
              boxShadow: '0 2px 6px rgba(66, 133, 244, 0.3)'
            }}
          >
            Iniciar Conversa
          </button>
        </div>
      </div>
    </div>
  )
}
