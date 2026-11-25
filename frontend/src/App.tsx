import React, { useState, useEffect, useCallback, useRef } from 'react'
import { Sidebar } from './components/Sidebar'
import { WelcomeView } from './components/WelcomeView'
import { ChatView } from './components/ChatView'
import { SettingsModal } from './components/SettingsModal'
import { NewConversationModal } from './components/NewConversationModal'
import { PermissionDialog } from './components/PermissionDialog'
import { CommandCenterModal } from './components/CommandCenterModal'
import { OnboardingModal } from './components/OnboardingModal'
import {
  fetchAgentInfo,
  fetchOnboardingStatus,
  fetchModels,
  fetchWorkspaces,
  fetchConversations,
  fetchConversation,
  fetchCapabilities,
  setConversationMode,
  createConversation,
  patchConversation,
  deleteConversation,
  sendPrompt,
  resumeConversation,
  regenerateConversation,
  respondPermission,
  cancelTurn,
  wsManager
} from './api'
import {
  AgentInfo,
  Conversation,
  Message,
  Model,
  ModesInfo,
  PermissionRequest,
  Plan,
  ToolCall,
  Workspace,
  WSEvent
} from './types'

export const App: React.FC = () => {
  // Tema dual (padrão 'dark')
  const [theme, setTheme] = useState<'dark' | 'light'>(() => {
    return (localStorage.getItem('gemini_theme') as 'dark' | 'light') || 'dark'
  })

  // Dados do backend
  const [agentInfo, setAgentInfo] = useState<AgentInfo | null>(null)
  const [models, setModels] = useState<Model[]>([])
  const [selectedModelId, setSelectedModelId] = useState<string>('')
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [selectedCwd, setSelectedCwd] = useState<string>('')
  const [conversations, setConversations] = useState<Conversation[]>([])
  // IDs temporários para mensagens otimistas: únicos mesmo no mesmo tick
// (Date.now() puro colide; hack "-1" também)
let tempIdCounter = 0
const makeTempId = (): string => `tmp-${Date.now()}-${(++tempIdCounter).toString(36)}`

const [activeConversationId, setActiveConversationId] = useState<string | null>(null)
  const [activeSessionId, setActiveSessionId] = useState<string>('')
  const [activeModes, setActiveModes] = useState<ModesInfo | null>(null)
  const [selectedModeId, setSelectedModeId] = useState<string>('')
  const [processingConversationIds, setProcessingConversationIds] = useState<Set<string>>(new Set())

  // Mensagens, Arquivos e Streaming
  const [messages, setMessages] = useState<Message[]>([])
  const [prompt, setPrompt] = useState<string>('')
  const [attachedFiles, setAttachedFiles] = useState<string[]>([])
  const [isStreaming, setIsStreaming] = useState<boolean>(false)
  const [streamingText, setStreamingText] = useState<string>('')
  const [streamingThought, setStreamingThought] = useState<string>('')
  const [activeToolCalls, setActiveToolCalls] = useState<ToolCall[]>([])
  const [pendingPermission, setPendingPermission] = useState<PermissionRequest | null>(null)
  const [activePlan, setActivePlan] = useState<Plan | null>(null)

  // Modais de UI
  const [isSettingsOpen, setIsSettingsOpen] = useState<boolean>(false)
  const [settingsTab, setSettingsTab] = useState<'general' | 'mcp'>('general')
  const [isNewConvModalOpen, setIsNewConvModalOpen] = useState<boolean>(false)
  const [isCommandCenterOpen, setIsCommandCenterOpen] = useState<boolean>(false)
  const [isOnboardingOpen, setIsOnboardingOpen] = useState<boolean>(false)
  const [serverPort, setServerPort] = useState<number>(8399)
  const [agentCmd, setAgentCmd] = useState<string>('')

  // Listener global para Command Center (Ctrl+K / Cmd+K)
  useEffect(() => {
    const handleGlobalKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setIsCommandCenterOpen((prev) => !prev)
      }
    }
    window.addEventListener('keydown', handleGlobalKeyDown)
    return () => window.removeEventListener('keydown', handleGlobalKeyDown)
  }, [])

  // Refs para capturar o estado mais atual no callback do WebSocket sem recriar listener
  const activeConvIdRef = useRef<string | null>(null)
  activeConvIdRef.current = activeConversationId
  const activeSessIdRef = useRef<string>('')
  activeSessIdRef.current = activeSessionId
  const streamingTextRef = useRef<string>('')
  streamingTextRef.current = streamingText
  const streamingThoughtRef = useRef<string>('')
  streamingThoughtRef.current = streamingThought

  // Sincronizar tema no HTML e no localStorage
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
    localStorage.setItem('gemini_theme', theme)
  }, [theme])

  const toggleTheme = () => {
    setTheme((prev) => (prev === 'dark' ? 'light' : 'dark'))
  }

  // Carregar dados iniciais do backend
  const loadInitialData = useCallback(async () => {
    // 1. Info da conta / agente
    const info = await fetchAgentInfo()
    setAgentInfo(info)

    // 2. Modelos
    const modelList = await fetchModels()
    setModels(modelList)
    if (modelList.length > 0) {
      const def = modelList.find((m) => m.isDefault) || modelList[0]
      setSelectedModelId(def.id)
    }

    // 3. Workspaces
    const wsList = await fetchWorkspaces()
    setWorkspaces(wsList)
    if (wsList.length > 0) {
      setSelectedCwd(wsList[0].cwd)
    }

    // 4. Histórico de Conversas
    try {
      const convs = await fetchConversations()
      setConversations(convs)
    } catch {
      // ignore
    }

    // 5. Configurações se disponíveis via Wails binding
    if (window.go?.main?.App?.GetConfig) {
      try {
        const cfg = await window.go.main.App.GetConfig()
        setServerPort(cfg.port || 8399)
        setAgentCmd(cfg.agent || '')
      } catch {
        // ignore
      }
    }

    // 6. Verificar status de Onboarding (primeira execução)
    try {
      const done = await fetchOnboardingStatus()
      if (!done) {
        setIsOnboardingOpen(true)
      }
    } catch {
      // ignore
    }
  }, [])

  useEffect(() => {
    loadInitialData()
  }, [loadInitialData])

  // Iniciar e monitorar conexão WebSocket
  useEffect(() => {
    wsManager.connect()

    const unsubscribe = wsManager.subscribe((event: WSEvent) => {
      handleWSEvent(event)
    })

    return () => {
      unsubscribe()
    }
  }, [])

  // Processar eventos de WebSocket
  const handleWSEvent = useCallback((event: WSEvent) => {
    // 1. Atualizar Set de processamento para qualquer conversa
    const eventConvId = event.conversationId
    if (eventConvId) {
      if (event.type === 'turn_done' || event.type === 'error') {
        setProcessingConversationIds((prev) => {
          if (!prev.has(eventConvId)) return prev
          const next = new Set(prev)
          next.delete(eventConvId)
          return next
        })
      } else {
        setProcessingConversationIds((prev) => {
          if (prev.has(eventConvId)) return prev
          const next = new Set(prev)
          next.add(eventConvId)
          return next
        })
      }
    }

    // 2. Filtrar por conversa ou sessão se especificado para a UI ativa
    if (event.conversationId && activeConvIdRef.current && event.conversationId !== activeConvIdRef.current) {
      return
    }
    if (event.sessionId && activeSessIdRef.current && event.sessionId !== activeSessIdRef.current) {
      return
    }

    switch (event.type) {
      case 'message_chunk':
        if (event.messageChunk?.text) {
          setStreamingText((prev) => prev + event.messageChunk!.text)
        }
        break

      case 'thought_chunk':
        if (event.thoughtChunk?.text) {
          setStreamingThought((prev) => prev + event.thoughtChunk!.text)
        }
        break

      case 'tool_call':
        if (event.toolCall) {
          const tc = event.toolCall
          setActiveToolCalls((prev) => {
            const idx = prev.findIndex((item) => item.id === tc.id)
            if (idx >= 0) {
              const updated = [...prev]
              updated[idx] = tc
              return updated
            }
            return [...prev, tc]
          })
        }
        break

      case 'plan':
        if (event.plan) {
          setActivePlan(event.plan)
        }
        break

      case 'permission_request':
        if (event.permissionRequest) {
          setPendingPermission(event.permissionRequest)
        }
        break

      case 'turn_done':
      case 'error': {
        const textToConsolidate = streamingTextRef.current
        const thoughtToConsolidate = streamingThoughtRef.current

        if (thoughtToConsolidate) {
          setMessages((prev) => [
            ...prev,
            {
              id: makeTempId(),
              conversationId: activeConvIdRef.current || '',
              seq: prev.length + 1,
              role: 'thought',
              content: thoughtToConsolidate,
              createdAt: Date.now()
            }
          ])
        }

        if (textToConsolidate) {
          setMessages((prev) => [
            ...prev,
            {
              id: makeTempId(),
              conversationId: activeConvIdRef.current || '',
              seq: prev.length + 1,
              role: 'assistant',
              content: textToConsolidate,
              createdAt: Date.now()
            }
          ])
        } else if (event.error?.msg) {
          setMessages((prev) => [
            ...prev,
            {
              id: makeTempId(),
              conversationId: activeConvIdRef.current || '',
              seq: prev.length + 1,
              role: 'error',
              content: event.error!.msg,
              createdAt: Date.now()
            }
          ])
        }

        // Limpar estados temporários do turno
        setStreamingText('')
        setStreamingThought('')
        setActiveToolCalls([])
        setPendingPermission(null)
        setActivePlan(null)
        setIsStreaming(false)

        // Atualizar lista e conversa do backend
        if (activeConvIdRef.current) {
          fetchConversation(activeConvIdRef.current).then((res) => {
            if (res.messages && res.messages.length > 0) {
              setMessages(res.messages)
            }
          }).catch(() => {})
          fetchConversations().then(setConversations).catch(() => {})
        }
        break
      }
    }
  }, [])

  // Manipular adição e remoção de arquivos anexados (máximo 10)
  const handleAddFiles = (newFiles: string[]) => {
    setAttachedFiles((prev) => {
      const combined = [...prev]
      for (const f of newFiles) {
        if (!combined.includes(f) && combined.length < 10) {
          combined.push(f)
        }
      }
      return combined
    })
  }

  const handleRemoveFile = (index: number) => {
    setAttachedFiles((prev) => prev.filter((_, i) => i !== index))
  }

  // Regenerar o último turno: trunca o histórico e reenvia o último prompt
  const handleRegenerate = async () => {
    if (!activeConversationId || isStreaming) return
    try {
      await regenerateConversation(activeConversationId)
      const res = await fetchConversation(activeConversationId)
      setMessages(res.messages || [])
      setIsStreaming(true)
      setStreamingText('')
      setStreamingThought('')
      setActiveToolCalls([])
      setPendingPermission(null)
      setActivePlan(null)
      setProcessingConversationIds((prev) => new Set(prev).add(activeConversationId))
    } catch {
      // Falha silenciosa: histórico permanece intacto
    }
  }

  // Editar o último prompt do usuário: trunca e reenvia com o novo texto
  const handleEditMessage = async (newText: string) => {
    if (!activeConversationId || isStreaming || !newText.trim()) return
    try {
      await regenerateConversation(activeConversationId, newText.trim())
      const res = await fetchConversation(activeConversationId)
      setMessages(res.messages || [])
      setIsStreaming(true)
      setStreamingText('')
      setStreamingThought('')
      setActiveToolCalls([])
      setPendingPermission(null)
      setActivePlan(null)
      setProcessingConversationIds((prev) => new Set(prev).add(activeConversationId))
    } catch {
      // Falha silenciosa: histórico permanece intacto
    }
  }

  // Carregar capacidades de uma conversa (modes, tier, configOptions)
  const loadCapabilitiesForConv = useCallback(async (convId: string, fallbackModeId?: string) => {
    try {
      const caps = await fetchCapabilities(convId)
      if (caps.modes) {
        setActiveModes(caps.modes)
        const curr = fallbackModeId || caps.modes.currentModeId || caps.modes.current || ''
        if (curr) {
          setSelectedModeId(curr)
        }
      } else {
        setActiveModes(null)
      }
      if (caps.tier) {
        setAgentInfo((prev) => prev ? { ...prev, tier: caps.tier } : { email: null, tier: caps.tier })
      }
    } catch {
      setActiveModes(null)
    }
  }, [])

  // Selecionar conversa existente do histórico
  const handleSelectConversation = async (id: string) => {
    try {
      const data = await fetchConversation(id)
      setActiveConversationId(id)
      setActiveSessionId(data.conversation.sessionId || '')
      setMessages(data.messages || [])
      if (data.conversation.modelId) {
        setSelectedModelId(data.conversation.modelId)
      }
      if (data.conversation.modeId) {
        setSelectedModeId(data.conversation.modeId)
      }
      if (data.conversation.cwd) {
        setSelectedCwd(data.conversation.cwd)
      }
      // Estado de streaming coerente: se ESTA conversa está em turno, mantém a
      // visão ao vivo; se não, limpa resíduos da conversa anterior
      const processing = processingConversationIds.has(id)
      setIsStreaming(processing)
      if (!processing) {
        setStreamingText('')
        setStreamingThought('')
        setActiveToolCalls([])
        setPendingPermission(null)
        setActivePlan(null)
      }
      setAttachedFiles([])

      // Resumir conversa no ACP se necessário
      resumeConversation(id).then((res) => {
        if (res.sessionId) {
          setActiveSessionId(res.sessionId)
        }
      }).catch(() => {})

      // Buscar capabilities (modes, tier)
      loadCapabilitiesForConv(id, data.conversation.modeId)
    } catch {
      // ignore
    }
  }

  // Confirmar criação de nova conversa via Modal de Nova Conversa
  const handleConfirmNewConversation = async (cwd?: string) => {
    try {
      const newConv = await createConversation(cwd, selectedModelId, selectedModeId || undefined)
      setActiveConversationId(newConv.id)
      setActiveSessionId(newConv.sessionId)
      if (newConv.cwd) {
        setSelectedCwd(newConv.cwd)
      }
      setMessages([])
      setStreamingText('')
      setStreamingThought('')
      setActiveToolCalls([])
      setPendingPermission(null)
      setActivePlan(null)
      setIsStreaming(false)
      setPrompt('')
      setAttachedFiles([])
      fetchConversations().then(setConversations).catch(() => {})
      loadCapabilitiesForConv(newConv.id, newConv.modeId)
    } catch {
      // Se falhar, reseta para welcome view
      setActiveConversationId(null)
      setActiveSessionId('')
      setActiveModes(null)
      setMessages([])
    }
  }

  // Iniciar nova conversa (abre modal)
  const handleOpenNewConversationModal = () => {
    setIsNewConvModalOpen(true)
  }

  // Excluir conversa
  const handleDeleteConversation = async (id: string) => {
    try {
      await deleteConversation(id)
      setConversations((prev) => prev.filter((c) => c.id !== id))
      setProcessingConversationIds((prev) => {
        if (!prev.has(id)) return prev
        const next = new Set(prev)
        next.delete(id)
        return next
      })
      if (activeConversationId === id) {
        setActiveConversationId(null)
        setActiveSessionId('')
        setActiveModes(null)
        setSelectedModeId('')
        setMessages([])
        setPrompt('')
        setAttachedFiles([])
      }
    } catch {
      // ignore
    }
  }

  // Renomear conversa
  const handleRenameConversation = async (id: string, newTitle: string) => {
    const prevConvs = conversations
    setConversations((prev) =>
      prev.map((c) => (c.id === id ? { ...c, title: newTitle } : c))
    )
    try {
      await patchConversation(id, { title: newTitle })
      const updated = await fetchConversations()
      setConversations(updated)
    } catch (err) {
      setConversations(prevConvs)
      throw err
    }
  }

  // Alterar modelo
  const handleSelectModel = async (modelId: string) => {
    setSelectedModelId(modelId)
    if (activeConversationId) {
      try {
        await patchConversation(activeConversationId, { modelId })
      } catch {
        // ignore
      }
    }
  }

  // Alterar modo de operação
  const handleSelectMode = async (modeId: string) => {
    setSelectedModeId(modeId)
    if (activeConversationId) {
      try {
        await setConversationMode(activeConversationId, modeId)
        setActiveModes((prev) => prev ? { ...prev, currentModeId: modeId, current: modeId } : null)
      } catch {
        // ignore
      }
    }
  }

  // Enviar prompt
  const handleSendPrompt = async () => {
    const text = prompt.trim()
    const filesToSend = [...attachedFiles]
    if (!text && filesToSend.length === 0) return
    if (isStreaming) return

    setPrompt('')
    setAttachedFiles([])
    let convId = activeConversationId

    try {
      // Se não houver conversa ativa, cria uma nova
      if (!convId) {
        const newConv = await createConversation(selectedCwd || undefined, selectedModelId, selectedModeId || undefined)
        convId = newConv.id
        setActiveConversationId(newConv.id)
        setActiveSessionId(newConv.sessionId)
        if (newConv.cwd) {
          setSelectedCwd(newConv.cwd)
        }
        fetchConversations().then(setConversations).catch(() => {})
        loadCapabilitiesForConv(newConv.id, newConv.modeId)
      }

      // Adiciona convId ao conjunto de processamento
      setProcessingConversationIds((prev) => new Set(prev).add(convId!))

      // Prepara metadados dos arquivos anexados para exibição imediata
      const metaJSON = filesToSend.length > 0 ? JSON.stringify(filesToSend.map((f) => {
        const parts = f.split(/[/\\]/)
        const base = parts[parts.length - 1] || f
        return `${base} (${f})`
      })) : undefined

      // Adiciona mensagem do usuário na tela
      const userMsg: Message = {
        id: makeTempId(),
        conversationId: convId,
        seq: messages.length + 1,
        role: 'user',
        content: text || 'Arquivo(s) anexado(s)',
        meta: metaJSON,
        createdAt: Date.now()
      }
      setMessages((prev) => [...prev, userMsg])
      setIsStreaming(true)
      setStreamingText('')
      setStreamingThought('')
      setActiveToolCalls([])
      setPendingPermission(null)

      // Envia prompt ao backend com lista de arquivos
      await sendPrompt(convId, text, filesToSend)
    } catch (err: unknown) {
      setIsStreaming(false)
      if (convId) {
        setProcessingConversationIds((prev) => {
          if (!prev.has(convId!)) return prev
          const next = new Set(prev)
          next.delete(convId!)
          return next
        })
      }
      const errorMsg = err instanceof Error ? err.message : 'Erro ao processar mensagem'
      setMessages((prev) => [
        ...prev,
        {
          id: makeTempId(),
          conversationId: convId || '',
          seq: prev.length + 1,
          role: 'error',
          content: errorMsg,
          createdAt: Date.now()
        }
      ])
    }
  }

  // Cancelar turno
  const handleCancelTurn = async () => {
    try {
      await cancelTurn(activeSessionId)
      setIsStreaming(false)
      if (activeConversationId) {
        setProcessingConversationIds((prev) => {
          if (!prev.has(activeConversationId)) return prev
          const next = new Set(prev)
          next.delete(activeConversationId)
          return next
        })
      }
    } catch {
      // ignore
    }
  }

  // Responder pedido de permissão
  const handleRespondPermission = async (callId: string, allow: boolean) => {
    try {
      await respondPermission(callId, allow)
      setPendingPermission(null)
    } catch {
      // ignore
    }
  }

  // Negar pedido de permissão e responder com feedback em texto
  const handleRejectWithFeedback = async (callId: string, feedback: string) => {
    try {
      await respondPermission(callId, false)
      setPendingPermission(null)
      if (activeConversationId && feedback.trim()) {
        await sendPrompt(activeConversationId, feedback.trim())
      }
    } catch {
      // ignore
    }
  }

  const handleOpenSettings = (tab?: 'general' | 'mcp') => {
    setSettingsTab(tab || 'general')
    setIsSettingsOpen(true)
  }

  const isWelcome = !activeConversationId && messages.length === 0
  const activeConversation = conversations.find((c) => c.id === activeConversationId)
  const currentCwd = activeConversation?.cwd || selectedCwd || ''

  return (
    <div
      style={{
        display: 'flex',
        width: '100vw',
        height: '100vh',
        backgroundColor: 'var(--bg-app)',
        color: 'var(--text-primary)',
        overflow: 'hidden'
      }}
    >
      {/* Sidebar esquerda (280px, colapsável) */}
      <Sidebar
        conversations={conversations}
        activeConversationId={activeConversationId}
        processingConversationIds={processingConversationIds}
        workspaces={workspaces}
        selectedCwd={selectedCwd}
        agentInfo={agentInfo}
        theme={theme}
        onToggleTheme={toggleTheme}
        onNewConversation={handleOpenNewConversationModal}
        onSelectConversation={handleSelectConversation}
        onDeleteConversation={handleDeleteConversation}
        onRenameConversation={handleRenameConversation}
        onSelectCwd={(cwd) => setSelectedCwd(cwd)}
        onOpenSettings={() => handleOpenSettings('general')}
      />

      {/* Área Central: Welcome State ou Chat View */}
      <main style={{ flex: 1, height: '100vh', display: 'flex', flexDirection: 'column', position: 'relative' }}>
        {isWelcome ? (
          <WelcomeView
            prompt={prompt}
            onChangePrompt={setPrompt}
            onSend={handleSendPrompt}
            onCancel={handleCancelTurn}
            isStreaming={isStreaming}
            models={models}
            selectedModelId={selectedModelId}
            onSelectModel={handleSelectModel}
            modes={activeModes}
            selectedModeId={selectedModeId}
            onSelectMode={handleSelectMode}
            files={attachedFiles}
            onAddFiles={handleAddFiles}
            onRemoveFile={handleRemoveFile}
            cwd={currentCwd}
          />
        ) : (
          <ChatView
            messages={messages}
            streamingText={streamingText}
            streamingThought={streamingThought}
            activeToolCalls={activeToolCalls}
            pendingPermission={pendingPermission}
            activePlan={activePlan}
            isStreaming={isStreaming}
            prompt={prompt}
            onChangePrompt={setPrompt}
            onSend={handleSendPrompt}
            onCancel={handleCancelTurn}
            onRespondPermission={handleRespondPermission}
            models={models}
            selectedModelId={selectedModelId}
            onSelectModel={handleSelectModel}
            modes={activeModes}
            selectedModeId={selectedModeId}
            onSelectMode={handleSelectMode}
            files={attachedFiles}
            onAddFiles={handleAddFiles}
            onRemoveFile={handleRemoveFile}
            cwd={currentCwd}
            onRegenerate={handleRegenerate}
            onEditMessage={handleEditMessage}
            conversationTitle={activeConversation?.title || ''}
            conversationId={activeConversationId || undefined}
          />
        )}
      </main>

      {/* Modal de Nova Conversa (Workspace Selection / Temporária) */}
      <NewConversationModal
        isOpen={isNewConvModalOpen}
        onClose={() => setIsNewConvModalOpen(false)}
        workspaces={workspaces}
        onStartConversation={handleConfirmNewConversation}
      />

      {/* Modal de Configurações */}
      <SettingsModal
        isOpen={isSettingsOpen}
        onClose={() => setIsSettingsOpen(false)}
        port={serverPort}
        agentCmd={agentCmd}
        theme={theme}
        onToggleTheme={toggleTheme}
        initialTab={settingsTab}
        cwd={currentCwd}
      />

      {/* Modal Central Rico de Permissão */}
      <PermissionDialog
        request={pendingPermission}
        onRespond={handleRespondPermission}
        onRejectWithFeedback={handleRejectWithFeedback}
        disabled={!isStreaming}
      />

      {/* Command Center Ctrl+K */}
      <CommandCenterModal
        isOpen={isCommandCenterOpen}
        onClose={() => setIsCommandCenterOpen(false)}
        onNewConversation={handleOpenNewConversationModal}
        onToggleTheme={toggleTheme}
        onOpenSettings={handleOpenSettings}
        onSelectConversation={handleSelectConversation}
        theme={theme}
      />

      {/* Onboarding Wizard de Primeira Execução */}
      <OnboardingModal
        isOpen={isOnboardingOpen}
        onClose={() => setIsOnboardingOpen(false)}
      />
    </div>
  )
}
