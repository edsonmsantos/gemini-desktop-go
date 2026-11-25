import React, { useRef, useEffect, useState } from 'react'
import { MicIcon, PlusIcon, SendIcon, StopIcon, FileIcon, CloseIcon } from './icons'
import { ModelSelect } from './ModelSelect'
import { ModeSelect } from './ModeSelect'
import { Model, ModesInfo } from '../types'
import { selectFilesViaWails } from '../api'

interface PromptBarProps {
  prompt: string
  onChangePrompt: (val: string) => void
  onSend: () => void
  onCancel: () => void
  isStreaming: boolean
  models: Model[]
  selectedModelId: string
  onSelectModel: (id: string) => void
  modes?: ModesInfo | null
  selectedModeId?: string
  onSelectMode?: (id: string) => void
  files?: string[]
  onAddFiles?: (files: string[]) => void
  onRemoveFile?: (index: number) => void
  placeholder?: string
}

function getBasename(path: string): string {
  const parts = path.split(/[/\\]/)
  return parts[parts.length - 1] || path
}

export const PromptBar: React.FC<PromptBarProps> = ({
  prompt,
  onChangePrompt,
  onSend,
  onCancel,
  isStreaming,
  models,
  selectedModelId,
  onSelectModel,
  modes,
  selectedModeId,
  onSelectMode,
  files = [],
  onAddFiles,
  onRemoveFile,
  placeholder = 'Peça ao Gemini'
}) => {
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const [hasNativePicker, setHasNativePicker] = useState<boolean>(false)

  useEffect(() => {
    setHasNativePicker(Boolean(window.go?.main?.App?.SelectFiles))
  }, [])

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
      textareaRef.current.style.height = `${Math.min(textareaRef.current.scrollHeight, 200)}px`
    }
  }, [prompt])

  const canSend = (prompt.trim().length > 0 || files.length > 0) && !isStreaming

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      if (canSend) {
        onSend()
      }
    }
  }

  const handlePaste = (e: React.ClipboardEvent<HTMLTextAreaElement>) => {
    const uriList = e.clipboardData.getData('text/uri-list')
    const gnomeList = e.clipboardData.getData('x-special/gnome-copied-files')
    const plainText = e.clipboardData.getData('text/plain')
    const raw = uriList || (gnomeList && gnomeList.includes('file://') ? gnomeList : '') || (plainText && plainText.includes('file://') ? plainText : '')

    if (raw && raw.includes('file://')) {
      const lines = raw.split(/[\r\n]+/)
      const extracted: string[] = []
      for (const line of lines) {
        const trimmed = line.trim()
        if (trimmed.startsWith('file://')) {
          try {
            const path = decodeURIComponent(trimmed.replace(/^file:\/\//, ''))
            if (path && !extracted.includes(path)) {
              extracted.push(path)
            }
          } catch {
            // ignore
          }
        }
      }
      if (extracted.length > 0) {
        e.preventDefault()
        onAddFiles?.(extracted)
      }
    }
  }

  const handleSelectFilesClick = async () => {
    if (!hasNativePicker || !onAddFiles) return
    const chosen = await selectFilesViaWails()
    if (chosen && chosen.length > 0) {
      onAddFiles(chosen)
    }
  }

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        width: '100%',
        maxWidth: 780,
        margin: '0 auto',
        backgroundColor: 'var(--bg-prompt)',
        border: '1px solid var(--border-card)',
        borderRadius: 24,
        boxShadow: 'var(--shadow-prompt)',
        padding: '10px 14px 8px 14px',
        transition: 'border-color 0.2s, box-shadow 0.2s'
      }}
    >
      {/* Chips de arquivos anexados */}
      {files.length > 0 && (
        <div
          style={{
            display: 'flex',
            flexWrap: 'wrap',
            gap: 6,
            marginBottom: 8,
            paddingLeft: 38
          }}
        >
          {files.map((filePath, index) => {
            const name = getBasename(filePath)
            return (
              <div
                key={`${filePath}-${index}`}
                title={filePath}
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 6,
                  padding: '4px 10px',
                  borderRadius: 14,
                  backgroundColor: 'var(--bg-pill)',
                  border: '1px solid var(--border-card)',
                  fontSize: 12,
                  color: 'var(--text-primary)',
                  maxWidth: 240,
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis'
                }}
              >
                <FileIcon size={14} color="var(--accent-blue)" />
                <span
                  style={{
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                    maxWidth: 160
                  }}
                >
                  {name}
                </span>
                {onRemoveFile && (
                  <button
                    onClick={() => onRemoveFile(index)}
                    title="Remover arquivo"
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      padding: 2,
                      borderRadius: '50%',
                      color: 'var(--text-muted)',
                      cursor: 'pointer',
                      border: 'none',
                      backgroundColor: 'transparent'
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.color = 'var(--status-failed)'
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.color = 'var(--text-muted)'
                    }}
                  >
                    <CloseIcon size={12} />
                  </button>
                )}
              </div>
            )
          })}
        </div>
      )}

      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 10 }}>
        {/* Botão + à esquerda (seletor nativo Wails ou desabilitado) */}
        <button
          onClick={handleSelectFilesClick}
          disabled={!hasNativePicker}
          title={hasNativePicker ? 'Anexar arquivos' : 'Disponível no app desktop'}
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: 32,
            height: 32,
            borderRadius: '50%',
            color: hasNativePicker ? 'var(--text-primary)' : 'var(--text-disabled)',
            backgroundColor: hasNativePicker ? 'var(--bg-pill)' : 'transparent',
            marginTop: 2,
            flexShrink: 0,
            cursor: hasNativePicker ? 'pointer' : 'default',
            transition: 'background 0.15s, color 0.15s'
          }}
          onMouseEnter={(e) => {
            if (hasNativePicker) {
              e.currentTarget.style.backgroundColor = 'var(--bg-pill-hover)'
            }
          }}
          onMouseLeave={(e) => {
            if (hasNativePicker) {
              e.currentTarget.style.backgroundColor = 'var(--bg-pill)'
            }
          }}
        >
          <PlusIcon size={18} />
        </button>

        {/* Textarea auto-expansível */}
        <textarea
          ref={textareaRef}
          value={prompt}
          onChange={(e) => onChangePrompt(e.target.value)}
          onKeyDown={handleKeyDown}
          onPaste={handlePaste}
          placeholder={placeholder}
          rows={1}
          style={{
            flex: 1,
            backgroundColor: 'transparent',
            resize: 'none',
            fontSize: 15,
            lineHeight: 1.5,
            maxHeight: 200,
            padding: '6px 2px',
            color: 'var(--text-primary)'
          }}
        />
      </div>

      {/* Barra inferior interna: Model dropdown + mic + send/stop */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          marginTop: 6,
          paddingLeft: 38
        }}
      >
        {/* Dropdowns de Modelo e Modo */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <ModelSelect
            models={models}
            selectedModelId={selectedModelId}
            onSelectModel={onSelectModel}
            disabled={isStreaming}
          />
          {modes && onSelectMode && (
            <ModeSelect
              modes={modes}
              selectedModeId={selectedModeId}
              onSelectMode={onSelectMode}
              disabled={isStreaming}
            />
          )}
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          {/* Microfone desabilitado */}
          <button
            disabled
            title="Entrada por voz (em breve)"
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: 34,
              height: 34,
              borderRadius: '50%',
              color: 'var(--text-disabled)'
            }}
          >
            <MicIcon size={18} />
          </button>

          {/* Botão de Enviar ou Parar */}
          {isStreaming ? (
            <button
              onClick={onCancel}
              title="Interromper geração"
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                width: 34,
                height: 34,
                borderRadius: '50%',
                backgroundColor: 'var(--status-failed)',
                color: '#ffffff',
                transition: 'transform 0.15s'
              }}
            >
              <StopIcon size={14} color="#ffffff" />
            </button>
          ) : (
            <button
              onClick={onSend}
              disabled={!canSend}
              title="Enviar mensagem (Enter)"
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                width: 34,
                height: 34,
                borderRadius: '50%',
                backgroundColor: canSend ? 'var(--accent-blue)' : 'var(--bg-pill)',
                color: canSend ? '#ffffff' : 'var(--text-disabled)',
                transition: 'background 0.2s, color 0.2s, transform 0.15s'
              }}
            >
              <SendIcon size={16} />
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
