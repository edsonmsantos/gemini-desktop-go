import React, { useState, useEffect, useCallback } from 'react'
import { CloseIcon, RefreshIcon, SpinnerIcon } from './icons'
import { DiagnosticsInfo } from '../types'
import { fetchDiagnostics, markOnboardingDone } from '../api'

interface OnboardingModalProps {
  isOpen: boolean
  onClose: () => void
}

export const OnboardingModal: React.FC<OnboardingModalProps> = ({ isOpen, onClose }) => {
  const [currentStep, setCurrentStep] = useState<number>(1)
  const [diagnostics, setDiagnostics] = useState<DiagnosticsInfo | null>(null)
  const [isLoadingDiagnostics, setIsLoadingDiagnostics] = useState<boolean>(false)
  const [diagnosticsError, setDiagnosticsError] = useState<string | null>(null)
  const [selectedAgent, setSelectedAgent] = useState<'agy' | 'gemini'>('agy')
  const [isFinishing, setIsFinishing] = useState<boolean>(false)

  const loadDiagnostics = useCallback(async () => {
    setIsLoadingDiagnostics(true)
    setDiagnosticsError(null)
    try {
      const data = await fetchDiagnostics()
      setDiagnostics(data)
      if (data.agentBinary && data.agentBinary.toLowerCase().includes('gemini')) {
        setSelectedAgent('gemini')
      } else {
        setSelectedAgent('agy')
      }
    } catch (err: unknown) {
      setDiagnosticsError(err instanceof Error ? err.message : 'Falha ao carregar diagnósticos')
    } finally {
      setIsLoadingDiagnostics(false)
    }
  }, [])

  useEffect(() => {
    if (isOpen) {
      setCurrentStep(1)
      loadDiagnostics()
    }
  }, [isOpen, loadDiagnostics])

  const handleFinish = async () => {
    if (isFinishing) return
    setIsFinishing(true)
    try {
      await markOnboardingDone()
    } catch {
      // ignore
    } finally {
      setIsFinishing(false)
      onClose()
    }
  }

  const handleNext = () => {
    if (currentStep < 3) {
      setCurrentStep((prev) => prev + 1)
    } else {
      handleFinish()
    }
  }

  const handleBack = () => {
    if (currentStep > 1) {
      setCurrentStep((prev) => prev - 1)
    }
  }

  if (!isOpen) return null

  return (
    <div
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: 'rgba(0, 0, 0, 0.65)',
        backdropFilter: 'blur(4px)',
        zIndex: 1100,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 16
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget) {
          handleFinish()
        }
      }}
    >
      <div
        style={{
          width: '100%',
          maxWidth: 620,
          backgroundColor: 'var(--bg-modal)',
          borderRadius: 16,
          border: '1px solid var(--border-card)',
          boxShadow: '0 20px 50px rgba(0, 0, 0, 0.55)',
          padding: 28,
          display: 'flex',
          flexDirection: 'column',
          gap: 20,
          overflow: 'hidden'
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Cabeçalho com indicador de progresso */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
              {[1, 2, 3].map((step) => {
                const isActive = step === currentStep
                const isPassed = step < currentStep
                return (
                  <div
                    key={step}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 6
                    }}
                  >
                    <div
                      style={{
                        width: 24,
                        height: 24,
                        borderRadius: '50%',
                        fontSize: 12,
                        fontWeight: 600,
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        backgroundColor: isActive
                          ? 'var(--accent-blue)'
                          : isPassed
                          ? 'var(--status-completed)'
                          : 'var(--bg-pill)',
                        color: isActive || isPassed ? '#ffffff' : 'var(--text-muted)',
                        transition: 'all 0.2s'
                      }}
                    >
                      {isPassed ? '✓' : step}
                    </div>
                    {step < 3 && (
                      <div
                        style={{
                          width: 28,
                          height: 2,
                          backgroundColor: isPassed ? 'var(--status-completed)' : 'var(--border-card)',
                          borderRadius: 1
                        }}
                      />
                    )}
                  </div>
                )
              })}
              <span style={{ fontSize: 12, color: 'var(--text-muted)', marginLeft: 8 }}>
                Passo {currentStep} de 3
              </span>
            </div>
          </div>

          <button
            type="button"
            onClick={handleFinish}
            title="Fechar e começar"
            style={{
              padding: 6,
              borderRadius: '50%',
              color: 'var(--text-secondary)',
              background: 'transparent',
              border: 'none',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center'
            }}
          >
            <CloseIcon size={18} />
          </button>
        </div>

        {/* Conteúdo do Passo */}
        <div style={{ minHeight: 280, display: 'flex', flexDirection: 'column' }}>
          {/* PASSO 1: Bem-vindo e Diagnóstico */}
          {currentStep === 1 && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
              <div>
                <h2 style={{ fontSize: 20, fontWeight: 600, color: 'var(--text-primary)', margin: 0 }}>
                  Bem-vindo ao Gemini Desktop! 👋
                </h2>
                <p style={{ fontSize: 13, color: 'var(--text-secondary)', marginTop: 6, marginBottom: 0, lineHeight: 1.5 }}>
                  Verificando o status dos componentes locais e do agente de IA para você começar.
                </p>
              </div>

              {isLoadingDiagnostics ? (
                <div
                  style={{
                    display: 'flex',
                    flexDirection: 'column',
                    alignItems: 'center',
                    justifyContent: 'center',
                    padding: 36,
                    gap: 12,
                    color: 'var(--text-muted)'
                  }}
                >
                  <span style={{ display: 'inline-flex', animation: 'spin 1s linear infinite' }}>
                    <SpinnerIcon size={24} />
                  </span>
                  <span style={{ fontSize: 13 }}>Executando diagnóstico local...</span>
                </div>
              ) : diagnosticsError ? (
                <div
                  style={{
                    padding: 16,
                    backgroundColor: 'rgba(234, 67, 53, 0.1)',
                    borderRadius: 10,
                    border: '1px solid var(--status-failed)',
                    color: 'var(--text-primary)',
                    fontSize: 13
                  }}
                >
                  <p style={{ margin: 0, fontWeight: 500 }}>Não foi possível obter o diagnóstico:</p>
                  <p style={{ margin: '4px 0 0 0', color: 'var(--text-muted)' }}>{diagnosticsError}</p>
                  <button
                    type="button"
                    onClick={loadDiagnostics}
                    style={{
                      marginTop: 10,
                      padding: '4px 10px',
                      borderRadius: 6,
                      background: 'var(--bg-pill)',
                      border: '1px solid var(--border-card)',
                      color: 'var(--text-primary)',
                      cursor: 'pointer',
                      fontSize: 12
                    }}
                  >
                    Tentar novamente
                  </button>
                </div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                  {/* Item 1: Agente encontrado */}
                  <div
                    style={{
                      display: 'flex',
                      alignItems: 'flex-start',
                      gap: 12,
                      padding: 12,
                      borderRadius: 10,
                      backgroundColor: 'var(--bg-card)',
                      border: '1px solid var(--border-card)'
                    }}
                  >
                    <div
                      style={{
                        width: 22,
                        height: 22,
                        borderRadius: '50%',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        backgroundColor: diagnostics?.agentFound ? 'rgba(52, 168, 83, 0.15)' : 'rgba(234, 67, 53, 0.15)',
                        color: diagnostics?.agentFound ? 'var(--status-completed)' : 'var(--status-failed)',
                        fontWeight: 'bold',
                        fontSize: 12,
                        flexShrink: 0,
                        marginTop: 2
                      }}
                    >
                      {diagnostics?.agentFound ? '✓' : '✗'}
                    </div>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 2, flex: 1 }}>
                      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                        <span style={{ fontSize: 13, fontWeight: 500, color: 'var(--text-primary)' }}>
                          Binário do Agente ACP
                        </span>
                        <span
                          style={{
                            fontSize: 11,
                            padding: '2px 8px',
                            borderRadius: 10,
                            backgroundColor: 'var(--bg-pill)',
                            color: 'var(--text-muted)'
                          }}
                        >
                          {diagnostics?.agentBinary || 'não configurado'}
                        </span>
                      </div>
                      <span style={{ fontSize: 12, color: 'var(--text-secondary)' }}>
                        {diagnostics?.agentFound
                          ? 'Binário encontrado e pronto para execução no sistema.'
                          : 'Binário não encontrado no PATH. Certifique-se de que o CLI está instalado.'}
                      </span>
                    </div>
                  </div>

                  {/* Item 2: Agente vivo */}
                  <div
                    style={{
                      display: 'flex',
                      alignItems: 'flex-start',
                      gap: 12,
                      padding: 12,
                      borderRadius: 10,
                      backgroundColor: 'var(--bg-card)',
                      border: '1px solid var(--border-card)'
                    }}
                  >
                    <div
                      style={{
                        width: 22,
                        height: 22,
                        borderRadius: '50%',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        backgroundColor: diagnostics?.agentAlive ? 'rgba(52, 168, 83, 0.15)' : 'rgba(234, 67, 53, 0.15)',
                        color: diagnostics?.agentAlive ? 'var(--status-completed)' : 'var(--status-failed)',
                        fontWeight: 'bold',
                        fontSize: 12,
                        flexShrink: 0,
                        marginTop: 2
                      }}
                    >
                      {diagnostics?.agentAlive ? '✓' : '✗'}
                    </div>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 2, flex: 1 }}>
                      <span style={{ fontSize: 13, fontWeight: 500, color: 'var(--text-primary)' }}>
                        Status do Processo
                      </span>
                      <span style={{ fontSize: 12, color: 'var(--text-secondary)' }}>
                        {diagnostics?.agentAlive
                          ? 'Processo do agente ACP está ativo e respondendo.'
                          : 'Agente ACP offline. O app tentará inicializá-lo ao enviar um prompt.'}
                      </span>
                    </div>
                  </div>

                  {/* Item 3: Conta Google */}
                  <div
                    style={{
                      display: 'flex',
                      alignItems: 'flex-start',
                      gap: 12,
                      padding: 12,
                      borderRadius: 10,
                      backgroundColor: 'var(--bg-card)',
                      border: '1px solid var(--border-card)'
                    }}
                  >
                    <div
                      style={{
                        width: 22,
                        height: 22,
                        borderRadius: '50%',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        backgroundColor: (diagnostics?.googleEmail || diagnostics?.agentAlive)
                          ? 'rgba(52, 168, 83, 0.15)'
                          : 'rgba(234, 67, 53, 0.15)',
                        color: (diagnostics?.googleEmail || diagnostics?.agentAlive)
                          ? 'var(--status-completed)'
                          : 'var(--status-failed)',
                        fontWeight: 'bold',
                        fontSize: 12,
                        flexShrink: 0,
                        marginTop: 2
                      }}
                    >
                      {diagnostics?.googleEmail || diagnostics?.agentAlive ? '✓' : '✗'}
                    </div>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 2, flex: 1 }}>
                      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                        <span style={{ fontSize: 13, fontWeight: 500, color: 'var(--text-primary)' }}>
                          Autenticação Google
                        </span>
                        {diagnostics?.googleEmail && (
                          <span
                            style={{
                              fontSize: 11,
                              padding: '2px 8px',
                              borderRadius: 10,
                              backgroundColor: 'rgba(52, 168, 83, 0.15)',
                              color: 'var(--status-completed)'
                            }}
                          >
                            {diagnostics.googleEmail}
                          </span>
                        )}
                      </div>
                      <span style={{ fontSize: 12, color: 'var(--text-secondary)' }}>
                        {diagnostics?.googleEmail
                          ? 'Sessão autenticada via Google Accounts.'
                          : diagnostics?.agentAlive
                          ? 'Agente autenticado (o agy gerencia as credenciais internamente).'
                          : 'Nenhuma conta detectada. Execute "agy auth login" ou "gemini" no terminal para conectar.'}
                      </span>
                    </div>
                  </div>

                  <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 4 }}>
                    <button
                      type="button"
                      onClick={loadDiagnostics}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 6,
                        fontSize: 12,
                        color: 'var(--text-muted)',
                        background: 'transparent',
                        border: 'none',
                        cursor: 'pointer',
                        padding: '4px 8px'
                      }}
                    >
                      <RefreshIcon size={12} />
                      Atualizar diagnóstico
                    </button>
                  </div>
                </div>
              )}
            </div>
          )}

          {/* PASSO 2: Escolha seu agente */}
          {currentStep === 2 && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
              <div>
                <h2 style={{ fontSize: 20, fontWeight: 600, color: 'var(--text-primary)', margin: 0 }}>
                  Escolha seu Agente 🤖
                </h2>
                <p style={{ fontSize: 13, color: 'var(--text-secondary)', marginTop: 6, marginBottom: 0, lineHeight: 1.5 }}>
                  O Gemini Desktop é compatível com múltiplos agentes baseados no protocolo ACP.
                </p>
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: 12, marginTop: 4 }}>
                {/* Opção AGY */}
                <label
                  style={{
                    display: 'flex',
                    alignItems: 'flex-start',
                    gap: 12,
                    padding: 14,
                    borderRadius: 12,
                    backgroundColor: selectedAgent === 'agy' ? 'rgba(66, 133, 244, 0.08)' : 'var(--bg-card)',
                    border: `1.5px solid ${selectedAgent === 'agy' ? 'var(--accent-blue)' : 'var(--border-card)'}`,
                    cursor: 'pointer',
                    transition: 'all 0.15s'
                  }}
                >
                  <input
                    type="radio"
                    name="agentChoice"
                    value="agy"
                    checked={selectedAgent === 'agy'}
                    onChange={() => setSelectedAgent('agy')}
                    style={{ marginTop: 3 }}
                  />
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 4, flex: 1 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--text-primary)' }}>
                        Antigravity CLI (agy)
                      </span>
                      <span
                        style={{
                          fontSize: 11,
                          padding: '2px 8px',
                          borderRadius: 8,
                          backgroundColor: 'rgba(66, 133, 244, 0.2)',
                          color: 'var(--accent-blue)',
                          fontWeight: 500
                        }}
                      >
                        Recomendado
                      </span>
                    </div>
                    <span style={{ fontSize: 12, color: 'var(--text-secondary)', lineHeight: 1.4 }}>
                      Suporte completo a ferramentas de terminal, subagentes, extensões, skills e servidores MCP.
                    </span>
                  </div>
                </label>

                {/* Opção Gemini CLI */}
                <label
                  style={{
                    display: 'flex',
                    alignItems: 'flex-start',
                    gap: 12,
                    padding: 14,
                    borderRadius: 12,
                    backgroundColor: selectedAgent === 'gemini' ? 'rgba(66, 133, 244, 0.08)' : 'var(--bg-card)',
                    border: `1.5px solid ${selectedAgent === 'gemini' ? 'var(--accent-blue)' : 'var(--border-card)'}`,
                    cursor: 'pointer',
                    transition: 'all 0.15s'
                  }}
                >
                  <input
                    type="radio"
                    name="agentChoice"
                    value="gemini"
                    checked={selectedAgent === 'gemini'}
                    onChange={() => setSelectedAgent('gemini')}
                    style={{ marginTop: 3 }}
                  />
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 4, flex: 1 }}>
                    <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--text-primary)' }}>
                      Gemini CLI (gemini)
                    </span>
                    <span style={{ fontSize: 12, color: 'var(--text-secondary)', lineHeight: 1.4 }}>
                      Agente padrão do Google com interface ACP direta e foco em tarefas rápidas.
                    </span>
                  </div>
                </label>
              </div>

              {/* Nota de rodapé informativa */}
              <div
                style={{
                  padding: '10px 14px',
                  borderRadius: 8,
                  backgroundColor: 'var(--bg-pill)',
                  border: '1px solid var(--border-subtle)',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 8,
                  marginTop: 6
                }}
              >
                <span style={{ fontSize: 14 }}>💡</span>
                <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>
                  A seleção acima é apenas informativa nesta versão. Você pode configurar o comando em{' '}
                  <code style={{ color: 'var(--text-primary)', fontFamily: 'var(--font-mono)' }}>~/.geminidesktop/config.json</code>.
                </span>
              </div>
            </div>
          )}

          {/* PASSO 3: Dicas */}
          {currentStep === 3 && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
              <div>
                <h2 style={{ fontSize: 20, fontWeight: 600, color: 'var(--text-primary)', margin: 0 }}>
                  Dicas para ser mais produtivo 🚀
                </h2>
                <p style={{ fontSize: 13, color: 'var(--text-secondary)', marginTop: 6, marginBottom: 0, lineHeight: 1.5 }}>
                  Conheça recursos essenciais pensados para acelerar o seu fluxo de desenvolvimento.
                </p>
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: 12, marginTop: 4 }}>
                {/* Dica 1 */}
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'flex-start',
                    gap: 14,
                    padding: 12,
                    borderRadius: 12,
                    backgroundColor: 'var(--bg-card)',
                    border: '1px solid var(--border-card)'
                  }}
                >
                  <div
                    style={{
                      fontSize: 22,
                      width: 36,
                      height: 36,
                      borderRadius: 8,
                      backgroundColor: 'var(--bg-pill)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      flexShrink: 0
                    }}
                  >
                    📋
                  </div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                    <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>
                      Cole Capturas de Tela Facilmente
                    </span>
                    <span style={{ fontSize: 12, color: 'var(--text-secondary)', lineHeight: 1.4 }}>
                      Pressione <kbd style={{ padding: '1px 5px', borderRadius: 4, background: 'var(--bg-pill)', border: '1px solid var(--border-subtle)', fontFamily: 'var(--font-mono)' }}>Ctrl+V</kbd> em qualquer lugar do campo de mensagem para anexar screenshots da sua área de transferência.
                    </span>
                  </div>
                </div>

                {/* Dica 2 */}
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'flex-start',
                    gap: 14,
                    padding: 12,
                    borderRadius: 12,
                    backgroundColor: 'var(--bg-card)',
                    border: '1px solid var(--border-card)'
                  }}
                >
                  <div
                    style={{
                      fontSize: 22,
                      width: 36,
                      height: 36,
                      borderRadius: 8,
                      backgroundColor: 'var(--bg-pill)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      flexShrink: 0
                    }}
                  >
                    📁
                  </div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                    <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>
                      Artefatos em Destaque
                    </span>
                    <span style={{ fontSize: 12, color: 'var(--text-secondary)', lineHeight: 1.4 }}>
                      Veja todos os arquivos gerados, relatórios e códigos criados pelo agente em abas dedicadas no topo da conversa.
                    </span>
                  </div>
                </div>

                {/* Dica 3 */}
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'flex-start',
                    gap: 14,
                    padding: 12,
                    borderRadius: 12,
                    backgroundColor: 'var(--bg-card)',
                    border: '1px solid var(--border-card)'
                  }}
                >
                  <div
                    style={{
                      fontSize: 22,
                      width: 36,
                      height: 36,
                      borderRadius: 8,
                      backgroundColor: 'var(--bg-pill)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      flexShrink: 0
                    }}
                  >
                    ✏️
                  </div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                    <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>
                      Edição e Regeneração
                    </span>
                    <span style={{ fontSize: 12, color: 'var(--text-secondary)', lineHeight: 1.4 }}>
                      Passe o mouse sobre qualquer mensagem para editar prompts ou regenerar respostas do agente quando necessário.
                    </span>
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Rodapé com botões de navegação */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            borderTop: '1px solid var(--border-subtle)',
            paddingTop: 16,
            marginTop: 4
          }}
        >
          {/* Botão Pular (texto) */}
          <button
            type="button"
            onClick={handleFinish}
            disabled={isFinishing}
            style={{
              padding: '8px 12px',
              background: 'transparent',
              border: 'none',
              color: 'var(--text-muted)',
              fontSize: 13,
              cursor: isFinishing ? 'not-allowed' : 'pointer',
              borderRadius: 6
            }}
          >
            Pular
          </button>

          {/* Botões Voltar / Avançar / Começar */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            {currentStep > 1 && (
              <button
                type="button"
                onClick={handleBack}
                disabled={isFinishing}
                style={{
                  padding: '8px 16px',
                  borderRadius: 8,
                  backgroundColor: 'var(--bg-pill)',
                  border: '1px solid var(--border-card)',
                  color: 'var(--text-primary)',
                  fontSize: 13,
                  fontWeight: 500,
                  cursor: isFinishing ? 'not-allowed' : 'pointer'
                }}
              >
                Voltar
              </button>
            )}

            {currentStep < 3 ? (
              <button
                type="button"
                onClick={handleNext}
                disabled={isFinishing}
                style={{
                  padding: '8px 20px',
                  borderRadius: 8,
                  backgroundColor: 'var(--accent-blue)',
                  border: 'none',
                  color: '#ffffff',
                  fontSize: 13,
                  fontWeight: 500,
                  cursor: isFinishing ? 'not-allowed' : 'pointer'
                }}
              >
                Avançar
              </button>
            ) : (
              <button
                type="button"
                onClick={handleFinish}
                disabled={isFinishing}
                style={{
                  padding: '8px 22px',
                  borderRadius: 8,
                  backgroundColor: 'var(--accent-blue)',
                  border: 'none',
                  color: '#ffffff',
                  fontSize: 13,
                  fontWeight: 600,
                  cursor: isFinishing ? 'not-allowed' : 'pointer',
                  boxShadow: '0 2px 8px rgba(66, 133, 244, 0.4)'
                }}
              >
                {isFinishing ? 'Iniciando...' : 'Começar'}
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
