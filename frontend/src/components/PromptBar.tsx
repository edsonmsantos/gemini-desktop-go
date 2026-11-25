import React, { useRef, useEffect, useState, useCallback } from 'react'
import { MicIcon, PlusIcon, SendIcon, StopIcon, FileIcon, CloseIcon, ZapIcon, SpinnerIcon } from './icons'
import { ModelSelect } from './ModelSelect'
import { ModeSelect } from './ModeSelect'
import { Model, ModesInfo, Skill } from '../types'
import { selectFilesViaWails, fetchSkills } from '../api'

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
  cwd?: string
}

function getBasename(path: string): string {
  const parts = path.split(/[/\\]/)
  return parts[parts.length - 1] || path
}

interface SkillItemRowProps {
  skill: Skill
  isSelected: boolean
  onSelect: () => void
}

const SkillItemRow: React.FC<SkillItemRowProps> = ({ skill, isSelected, onSelect }) => {
  const rowRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (isSelected && rowRef.current) {
      rowRef.current.scrollIntoView({ block: 'nearest' })
    }
  }, [isSelected])

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
      <ZapIcon size={16} color={isSelected ? 'var(--accent-blue)' : 'var(--text-muted)'} />
      <div style={{ display: 'flex', flexDirection: 'column', minWidth: 0, flex: 1 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 6, minWidth: 0 }}>
          <span style={{ fontWeight: 600, fontSize: 13, color: 'var(--text-primary)', whiteSpace: 'nowrap' }}>
            /{skill.slug}
          </span>
          {skill.name && skill.name !== skill.slug && (
            <span
              style={{
                fontSize: 12,
                color: 'var(--text-muted)',
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis'
              }}
            >
              ({skill.name})
            </span>
          )}
        </div>
        {skill.description && (
          <span
            style={{
              fontSize: 12,
              color: 'var(--text-muted)',
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              marginTop: 1
            }}
            title={skill.description}
          >
            {skill.description}
          </span>
        )}
      </div>
    </button>
  )
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
  placeholder = 'Peça ao Gemini',
  cwd
}) => {
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const paletteRef = useRef<HTMLDivElement>(null)
  const isPastingRef = useRef<boolean>(false)
  const [hasNativePicker, setHasNativePicker] = useState<boolean>(false)

  // Estado da paleta de skills
  const [showSkills, setShowSkills] = useState<boolean>(false)
  const [tokenRange, setTokenRange] = useState<{ start: number; end: number } | null>(null)
  const [skillQuery, setSkillQuery] = useState<string>('')
  const [skills, setSkills] = useState<Skill[]>([])
  const [isLoadingSkills, setIsLoadingSkills] = useState<boolean>(false)
  const [skillsError, setSkillsError] = useState<string | null>(null)
  const [selectedIndex, setSelectedIndex] = useState<number>(0)

  useEffect(() => {
    setHasNativePicker(Boolean(window.go?.main?.App?.SelectFiles))
  }, [])

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
      textareaRef.current.style.height = `${Math.min(textareaRef.current.scrollHeight, 200)}px`
    }
  }, [prompt])

  // Verificação e ativação da paleta de skills
  const checkSkillTrigger = useCallback(
    (text: string, cursorPos: number) => {
      if (isPastingRef.current || cursorPos < 1) {
        if (showSkills) setShowSkills(false)
        return
      }

      const textBefore = text.slice(0, cursorPos)
      const lastBoundary = Math.max(
        textBefore.lastIndexOf(' '),
        textBefore.lastIndexOf('\n'),
        textBefore.lastIndexOf('\t')
      )
      const tokenStart = lastBoundary === -1 ? 0 : lastBoundary + 1
      const candidate = textBefore.slice(tokenStart)

      if (candidate.startsWith('/') || candidate.startsWith('$')) {
        const query = candidate.slice(1)
        setTokenRange({ start: tokenStart, end: cursorPos })
        setSkillQuery(query)

        if (!showSkills) {
          setShowSkills(true)
          setIsLoadingSkills(true)
          setSkillsError(null)
          fetchSkills(cwd)
            .then((res) => {
              setSkills(res.skills)
              setIsLoadingSkills(false)
            })
            .catch(() => {
              setSkillsError('Erro ao carregar skills')
              setIsLoadingSkills(false)
            })
        }
      } else {
        if (showSkills) setShowSkills(false)
      }
    },
    [showSkills, cwd]
  )

  // Filtragem e organização por escopo
  const q = skillQuery.toLowerCase().trim()
  const filteredSkills = skills.filter((s) => {
    if (!q) return true
    return (
      s.slug.toLowerCase().includes(q) ||
      s.name.toLowerCase().includes(q) ||
      (s.description && s.description.toLowerCase().includes(q))
    )
  })

  const wsSkills = filteredSkills.filter((s) => s.scope === 'workspace')
  const userSkills = filteredSkills.filter((s) => s.scope === 'user')
  const hasBothScopes = wsSkills.length > 0 && userSkills.length > 0
  const displayList = hasBothScopes ? [...wsSkills, ...userSkills] : filteredSkills

  useEffect(() => {
    setSelectedIndex(0)
  }, [skillQuery, skills])

  // Fechar paleta em clique-fora
  useEffect(() => {
    if (!showSkills) return
    const handleClickOutside = (e: MouseEvent) => {
      if (
        paletteRef.current &&
        !paletteRef.current.contains(e.target as Node) &&
        textareaRef.current &&
        !textareaRef.current.contains(e.target as Node)
      ) {
        setShowSkills(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [showSkills])

  const handleSelectSkill = (skill: Skill) => {
    if (!tokenRange) return
    const before = prompt.slice(0, tokenRange.start)
    const after = prompt.slice(tokenRange.end)
    const replacement = `/${skill.slug} `
    const newPrompt = `${before}${replacement}${after}`
    onChangePrompt(newPrompt)
    setShowSkills(false)

    requestAnimationFrame(() => {
      if (textareaRef.current) {
        const newPos = before.length + replacement.length
        textareaRef.current.selectionStart = newPos
        textareaRef.current.selectionEnd = newPos
        textareaRef.current.focus()
      }
    })
  }

  const canSend = (prompt.trim().length > 0 || files.length > 0) && !isStreaming

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (showSkills) {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        if (displayList.length > 0) {
          setSelectedIndex((prev) => (prev + 1) % displayList.length)
        }
        return
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        if (displayList.length > 0) {
          setSelectedIndex((prev) => (prev - 1 + displayList.length) % displayList.length)
        }
        return
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        if (displayList.length > 0 && displayList[selectedIndex]) {
          e.preventDefault()
          handleSelectSkill(displayList[selectedIndex])
          return
        } else {
          setShowSkills(false)
          if (e.key === 'Tab') {
            e.preventDefault()
            return
          }
        }
      }
      if (e.key === 'Escape') {
        e.preventDefault()
        setShowSkills(false)
        return
      }
    }

    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      if (canSend) {
        onSend()
      }
    }
  }

  const handlePaste = (e: React.ClipboardEvent<HTMLTextAreaElement>) => {
    isPastingRef.current = true
    setShowSkills(false)
    setTimeout(() => {
      isPastingRef.current = false
    }, 150)

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
        position: 'relative',
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
      {/* Painel flutuante de Skills (estilo ZCode) */}
      {showSkills && (
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
          {isLoadingSkills && (
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
              <SpinnerIcon size={14} /> Carregando skills...
            </div>
          )}

          {skillsError && (
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
              <span>{skillsError}</span>
              <button
                type="button"
                onClick={() => setShowSkills(false)}
                style={{ color: 'var(--text-muted)', cursor: 'pointer', padding: 2 }}
              >
                <CloseIcon size={14} />
              </button>
            </div>
          )}

          {!isLoadingSkills && !skillsError && displayList.length === 0 && (
            <div
              style={{
                padding: '12px 14px',
                fontSize: 13,
                color: 'var(--text-muted)',
                textAlign: 'center'
              }}
            >
              Nenhuma skill encontrada
            </div>
          )}

          {!isLoadingSkills && !skillsError && displayList.length > 0 && (
            hasBothScopes ? (
              <>
                {wsSkills.length > 0 && (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                    <div
                      style={{
                        fontSize: 11,
                        fontWeight: 600,
                        textTransform: 'uppercase',
                        letterSpacing: '0.05em',
                        color: 'var(--text-muted)',
                        padding: '6px 12px 2px 12px'
                      }}
                    >
                      Workspace
                    </div>
                    {wsSkills.map((sk) => {
                      const idx = displayList.indexOf(sk)
                      const isSelected = idx === selectedIndex
                      return (
                        <SkillItemRow
                          key={`ws-${sk.slug}`}
                          skill={sk}
                          isSelected={isSelected}
                          onSelect={() => handleSelectSkill(sk)}
                        />
                      )
                    })}
                  </div>
                )}
                {userSkills.length > 0 && (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 2, marginTop: wsSkills.length > 0 ? 4 : 0 }}>
                    <div
                      style={{
                        fontSize: 11,
                        fontWeight: 600,
                        textTransform: 'uppercase',
                        letterSpacing: '0.05em',
                        color: 'var(--text-muted)',
                        padding: '6px 12px 2px 12px'
                      }}
                    >
                      Global
                    </div>
                    {userSkills.map((sk) => {
                      const idx = displayList.indexOf(sk)
                      const isSelected = idx === selectedIndex
                      return (
                        <SkillItemRow
                          key={`usr-${sk.slug}`}
                          skill={sk}
                          isSelected={isSelected}
                          onSelect={() => handleSelectSkill(sk)}
                        />
                      )
                    })}
                  </div>
                )}
              </>
            ) : (
              displayList.map((sk, idx) => {
                const isSelected = idx === selectedIndex
                return (
                  <SkillItemRow
                    key={`${sk.scope}-${sk.slug}`}
                    skill={sk}
                    isSelected={isSelected}
                    onSelect={() => handleSelectSkill(sk)}
                  />
                )
              })
            )
          )}
        </div>
      )}

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
                    type="button"
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
          type="button"
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
          onChange={(e) => {
            onChangePrompt(e.target.value)
            if (!isPastingRef.current) {
              checkSkillTrigger(e.target.value, e.target.selectionStart)
            }
          }}
          onKeyUp={(e) => {
            if (
              e.key !== 'ArrowDown' &&
              e.key !== 'ArrowUp' &&
              e.key !== 'Enter' &&
              e.key !== 'Tab' &&
              e.key !== 'Escape'
            ) {
              const pos = textareaRef.current?.selectionStart ?? prompt.length
              checkSkillTrigger(prompt, pos)
            }
          }}
          onClick={() => {
            if (showSkills) {
              const pos = textareaRef.current?.selectionStart ?? prompt.length
              checkSkillTrigger(prompt, pos)
            }
          }}
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
            type="button"
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
              type="button"
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
              type="button"
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
