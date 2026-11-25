import React, { useRef, useEffect, useState, useCallback } from 'react'
import { MicIcon, PlusIcon, SendIcon, StopIcon, FileIcon, CloseIcon, ZapIcon, SpinnerIcon, ClipboardIcon } from './icons'
import { ModelSelect } from './ModelSelect'
import { ModeSelect } from './ModeSelect'
import { Model, ModesInfo, SavedPrompt, Skill } from '../types'
import { uploadAttachment, pasteClipboardImage } from '../api'
import { selectFilesViaWails, fetchSkills, fetchSavedPrompts } from '../api'
import { PromptsPalette } from './PromptsPalette'

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
  const [dragging, setDragging] = useState<boolean>(false)
  const [pastingImage, setPastingImage] = useState<boolean>(false)
  const pasteHandledRef = useRef<boolean>(false)
  const [tokenRange, setTokenRange] = useState<{ start: number; end: number } | null>(null)
  const [skillQuery, setSkillQuery] = useState<string>('')
  const [skills, setSkills] = useState<Skill[]>([])
  const [isLoadingSkills, setIsLoadingSkills] = useState<boolean>(false)
  const [skillsError, setSkillsError] = useState<string | null>(null)
  const [selectedIndex, setSelectedIndex] = useState<number>(0)

  // Estado da paleta de prompts salvos (@)
  const promptPaletteRef = useRef<HTMLDivElement>(null)
  const [showPrompts, setShowPrompts] = useState<boolean>(false)
  const [promptTokenRange, setPromptTokenRange] = useState<{ start: number; end: number } | null>(null)
  const [promptFilterQuery, setPromptFilterQuery] = useState<string>('')
  const [savedPrompts, setSavedPrompts] = useState<SavedPrompt[]>([])
  const [isLoadingPrompts, setIsLoadingPrompts] = useState<boolean>(false)
  const [promptsError, setPromptsError] = useState<string | null>(null)
  const [promptSelectedIndex, setPromptSelectedIndex] = useState<number>(0)

  useEffect(() => {
    setHasNativePicker(Boolean(window.go?.main?.App?.SelectFiles))
  }, [])

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
      textareaRef.current.style.height = `${Math.min(textareaRef.current.scrollHeight, 200)}px`
    }
  }, [prompt])

  // Verificação e ativação das paletas (skills: / ou $, prompts: @)
  const checkTriggers = useCallback(
    (text: string, cursorPos: number) => {
      if (isPastingRef.current || cursorPos < 1) {
        if (showSkills) setShowSkills(false)
        if (showPrompts) setShowPrompts(false)
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
        if (showPrompts) setShowPrompts(false)
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
      } else if (candidate.startsWith('@')) {
        if (showSkills) setShowSkills(false)
        const query = candidate.slice(1)
        setPromptTokenRange({ start: tokenStart, end: cursorPos })
        setPromptFilterQuery(query)

        if (!showPrompts) {
          setShowPrompts(true)
          setIsLoadingPrompts(true)
          setPromptsError(null)
          fetchSavedPrompts()
            .then((res) => {
              setSavedPrompts(res ?? [])
              setIsLoadingPrompts(false)
            })
            .catch(() => {
              setPromptsError('Erro ao carregar prompts')
              setIsLoadingPrompts(false)
            })
        }
      } else {
        if (showSkills) setShowSkills(false)
        if (showPrompts) setShowPrompts(false)
      }
    },
    [showSkills, showPrompts, cwd]
  )

  // Filtragem e organização de skills por escopo
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

  // Filtragem de prompts salvos
  const pq = promptFilterQuery.toLowerCase().trim()
  const filteredSavedPrompts = savedPrompts.filter((p) => {
    if (!pq) return true
    return (
      p.title.toLowerCase().includes(pq) ||
      p.content.toLowerCase().includes(pq)
    )
  })

  useEffect(() => {
    setPromptSelectedIndex(0)
  }, [promptFilterQuery, savedPrompts])

  // Fechar paleta de skills em clique-fora
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

  // Fechar paleta de prompts em clique-fora
  useEffect(() => {
    if (!showPrompts) return
    const handleClickOutside = (e: MouseEvent) => {
      if (
        promptPaletteRef.current &&
        !promptPaletteRef.current.contains(e.target as Node) &&
        textareaRef.current &&
        !textareaRef.current.contains(e.target as Node)
      ) {
        setShowPrompts(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [showPrompts])

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

  const handleSelectSavedPrompt = (sp: SavedPrompt) => {
    if (!promptTokenRange) return
    const before = prompt.slice(0, promptTokenRange.start)
    const after = prompt.slice(promptTokenRange.end)
    const cleanContent = sp.content.endsWith(' ') || sp.content.endsWith('\n') ? sp.content : `${sp.content} `
    const newPrompt = `${before}${cleanContent}${after}`
    onChangePrompt(newPrompt)
    setShowPrompts(false)

    requestAnimationFrame(() => {
      if (textareaRef.current) {
        const newPos = before.length + cleanContent.length
        textareaRef.current.selectionStart = newPos
        textareaRef.current.selectionEnd = newPos
        textareaRef.current.focus()
      }
    })
  }

  const canSend = (prompt.trim().length > 0 || files.length > 0) && !isStreaming

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    handlePasteKeyDown(e)
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

    if (showPrompts) {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        if (filteredSavedPrompts.length > 0) {
          setPromptSelectedIndex((prev) => (prev + 1) % filteredSavedPrompts.length)
        }
        return
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        if (filteredSavedPrompts.length > 0) {
          setPromptSelectedIndex((prev) => (prev - 1 + filteredSavedPrompts.length) % filteredSavedPrompts.length)
        }
        return
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        if (filteredSavedPrompts.length > 0 && filteredSavedPrompts[promptSelectedIndex]) {
          e.preventDefault()
          handleSelectSavedPrompt(filteredSavedPrompts[promptSelectedIndex])
          return
        } else {
          setShowPrompts(false)
          if (e.key === 'Tab') {
            e.preventDefault()
            return
          }
        }
      }
      if (e.key === 'Escape') {
        e.preventDefault()
        setShowPrompts(false)
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
    setShowPrompts(false)
    setTimeout(() => {
      isPastingRef.current = false
    }, 150)

    const uriList = e.clipboardData.getData('text/uri-list')
    const gnomeList = e.clipboardData.getData('x-special/gnome-copied-files')
    const plainText = e.clipboardData.getData('text/plain')
    const raw = uriList || (gnomeList && gnomeList.includes('file://') ? gnomeList : '') || (plainText && plainText.includes('file://') ? plainText : '')

    // Cola caminho absoluto puro como arquivo (ex.: "/home/user/doc.pdf" vindo
    // de gerenciadores que só expõem text/plain sem file://)
    if (!raw && plainText) {
      const candidate = plainText.trim()
      const lastSegment = candidate.split(/[/\\]/).pop() || ''
      if (
        candidate.startsWith('/') && candidate.indexOf('\n') === -1 &&
        !candidate.includes(' ') && lastSegment.includes('.')
      ) {
        e.preventDefault()
        onAddFiles?.([candidate])
        return
      }
    }
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
        pasteHandledRef.current = true
        onAddFiles?.(extracted)
        return
      }
    }

    // Imagens coladas direto da clipboard (ex.: screenshot Ctrl+V no Chromium)
    const items = Array.from(e.clipboardData?.items ?? [])
    const imageFiles: File[] = []
    for (const item of items) {
      if (item.kind === 'file' && item.type.startsWith('image/')) {
        const file = item.getAsFile()
        if (file) imageFiles.push(file)
      }
    }
    if (imageFiles.length > 0 && onAddFiles) {
      e.preventDefault()
      pasteHandledRef.current = true
      void (async () => {
        const paths: string[] = []
        for (const file of imageFiles.slice(0, 10)) {
          const path = await uploadAttachment(file)
          if (path) paths.push(path)
        }
        if (paths.length > 0) onAddFiles?.(paths)
      })()
    }
  }

  // Fallback nativo: WebKitGTK não entrega imagens no evento paste. Se o Ctrl+V
  // não foi tratado pelo paste em ~350ms, pergunta ao backend se há imagem na clipboard.
  const handlePasteKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (!(e.ctrlKey || e.metaKey) || e.key.toLowerCase() !== 'v') return
    pasteHandledRef.current = false
    window.setTimeout(async () => {
      if (pasteHandledRef.current) return
      const path = await pasteClipboardImage()
      if (path) onAddFiles?.([path])
    }, 350)
  }

  // Botão "Colar": lê a imagem da clipboard do sistema direto do backend
  const handlePasteButtonClick = async () => {
    if (!onAddFiles || pastingImage) return
    setPastingImage(true)
    try {
      const path = await pasteClipboardImage()
      if (path) onAddFiles([path])
    } finally {
      setPastingImage(false)
    }
  }

  // Anexa arquivos soltos na conversa. WebKitGTK entrega o drop como
  // text/uri-list com caminhos reais (atalho rápido, sem upload); Chromium
  // entrega File objects (conteúdo enviado ao backend e salvo em temp).
  const handleDrop = useCallback(
    async (e: React.DragEvent<HTMLDivElement>) => {
      e.preventDefault()
      setDragging(false)
      if (!onAddFiles) return
      const dt = e.dataTransfer

      // 1. Caminhos reais via uri-list (WebKitGTK / gerenciador de arquivos)
      const uriList = dt.getData('text/uri-list') || ''
      const gnomeList = dt.getData('x-special/gnome-copied-files') || ''
      const raw = uriList || (gnomeList.includes('file://') ? gnomeList : '')
      if (raw.includes('file://')) {
        const extracted: string[] = []
        for (const line of raw.split(/[\r\n]+/)) {
          const trimmed = line.trim()
          if (!trimmed.startsWith('file://')) continue
          try {
            const path = decodeURIComponent(trimmed.replace(/^file:\/\//, ''))
            if (path && !extracted.includes(path)) extracted.push(path)
          } catch {
            // ignore
          }
        }
        if (extracted.length > 0) {
          onAddFiles(extracted)
          return
        }
      }

      // 2. File objects (Chromium): upload do conteúdo
      const dropped = Array.from(dt.files ?? [])
      if (dropped.length === 0) return
      const paths: string[] = []
      for (const file of dropped.slice(0, 10)) {
        const path = await uploadAttachment(file)
        if (path) paths.push(path)
      }
      if (paths.length > 0) onAddFiles(paths)
    },
    [onAddFiles]
  )

  const handleSelectFilesClick = async () => {
    if (!hasNativePicker || !onAddFiles) return
    const chosen = await selectFilesViaWails()
    if (chosen && chosen.length > 0) {
      onAddFiles(chosen)
    }
  }

  return (
    <div
      onDragOver={(e) => {
        e.preventDefault()
        if (!dragging) setDragging(true)
      }}
      onDragLeave={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node)) setDragging(false)
      }}
      onDrop={(e) => void handleDrop(e)}
      style={{
        position: 'relative',
        display: 'flex',
        flexDirection: 'column',
        width: '100%',
        maxWidth: 780,
        margin: '0 auto',
        backgroundColor: 'var(--bg-prompt)',
        border: dragging ? '2px dashed var(--accent-blue)' : '1px solid var(--border-card)',
        borderRadius: 24,
        boxShadow: 'var(--shadow-prompt)',
        padding: dragging ? '9px 13px 7px 13px' : '10px 14px 8px 14px',
        transition: 'border-color 0.2s, box-shadow 0.2s',
        '--wails-drop-target': 'drop'
      } as React.CSSProperties}
    >
      {/* Painel flutuante de Prompts Salvos (@) */}
      {showPrompts && (
        <PromptsPalette
          paletteRef={promptPaletteRef}
          prompts={filteredSavedPrompts}
          isLoading={isLoadingPrompts}
          error={promptsError}
          selectedIndex={promptSelectedIndex}
          onSelectPrompt={handleSelectSavedPrompt}
          onClose={() => setShowPrompts(false)}
        />
      )}

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
        {/* Botão Colar imagem da clipboard (fallback nativo do WebKitGTK) */}
        <button
          type="button"
          onClick={handlePasteButtonClick}
          title="Colar imagem da clipboard (print)"
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: 32,
            height: 32,
            borderRadius: '50%',
            color: 'var(--text-primary)',
            backgroundColor: pastingImage ? 'var(--bg-pill-hover)' : 'var(--bg-pill)',
            marginTop: 2,
            flexShrink: 0,
            cursor: pastingImage ? 'wait' : 'pointer',
            transition: 'background 0.15s'
          }}
        >
          <ClipboardIcon size={16} className={pastingImage ? 'animate-spin' : ''} />
        </button>

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
              checkTriggers(e.target.value, e.target.selectionStart)
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
              checkTriggers(prompt, pos)
            }
          }}
          onClick={() => {
            if (showSkills || showPrompts) {
              const pos = textareaRef.current?.selectionStart ?? prompt.length
              checkTriggers(prompt, pos)
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
