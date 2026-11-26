import React, { useState, useEffect, useCallback, useRef } from 'react'
import {
  BrowserWizardBrowser,
  BrowserWizardState
} from '../types'
import {
  fetchBrowserWizardState,
  installBrowserPolicy
} from '../api'
import {
  CheckIcon,
  CloseIcon,
  ExternalLinkIcon,
  GlobeIcon,
  RefreshIcon,
  SpinnerIcon,
  TerminalIcon
} from './icons'

interface BrowserSetupWizardProps {
  onComplete?: () => void
  onSkip?: () => void
  isEmbeddedInModal?: boolean
}

const BROWSER_NAMES: Record<string, string> = {
  chrome: 'Google Chrome',
  brave: 'Brave Browser',
  edge: 'Microsoft Edge',
  chromium: 'Chromium'
}

export const BrowserSetupWizard: React.FC<BrowserSetupWizardProps> = ({
  onComplete,
  onSkip,
  isEmbeddedInModal = false
}) => {
  const [state, setState] = useState<BrowserWizardState | null>(null)
  const [isLoading, setIsLoading] = useState<boolean>(true)
  const [error, setError] = useState<string | null>(null)

  const [installingBrowser, setInstallingBrowser] = useState<string | null>(null)
  const [restartingBrowser, setRestartingBrowser] = useState<string | null>(null)
  const [elapsedSeconds, setElapsedSeconds] = useState<number>(0)
  const [showManualFallback, setShowManualFallback] = useState<boolean>(false)
  const [statusMessage, setStatusMessage] = useState<string | null>(null)

  const pollIntervalRef = useRef<number | null>(null)
  const timerIntervalRef = useRef<number | null>(null)

  const loadState = useCallback(async (silent = false) => {
    if (!silent) {
      setIsLoading(true)
      setError(null)
    }
    try {
      const data = await fetchBrowserWizardState()
      setState(data)
      return data
    } catch (err: unknown) {
      if (!silent) {
        setError(err instanceof Error ? err.message : 'Falha ao carregar estado do navegador')
      }
      return null
    } finally {
      if (!silent) {
        setIsLoading(false)
      }
    }
  }, [])

  useEffect(() => {
    loadState()
  }, [loadState])

  // Polling e Timer quando aguardando reinício do navegador
  useEffect(() => {
    if (restartingBrowser) {
      setElapsedSeconds(0)
      setShowManualFallback(false)

      timerIntervalRef.current = window.setInterval(() => {
        setElapsedSeconds((prev) => {
          const next = prev + 1
          if (next >= 120) {
            setShowManualFallback(true)
          }
          return next
        })
      }, 1000)

      pollIntervalRef.current = window.setInterval(async () => {
        const latest = await loadState(true)
        if (latest && latest.extensionConnected) {
          setRestartingBrowser(null)
          setStatusMessage('Extensão conectada com sucesso!')
        }
      }, 2500)

      return () => {
        if (timerIntervalRef.current !== null) {
          clearInterval(timerIntervalRef.current)
          timerIntervalRef.current = null
        }
        if (pollIntervalRef.current !== null) {
          clearInterval(pollIntervalRef.current)
          pollIntervalRef.current = null
        }
      }
    }
  }, [restartingBrowser, loadState])

  const handleInstallPolicy = async (browser: string) => {
    setInstallingBrowser(browser)
    setStatusMessage(null)
    setError(null)
    try {
      await installBrowserPolicy(browser)
      setRestartingBrowser(browser)
      await loadState(true)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Erro ao aplicar política no navegador')
    } finally {
      setInstallingBrowser(null)
    }
  }

  const handleCheckNow = async () => {
    const latest = await loadState(true)
    if (latest && latest.extensionConnected) {
      setRestartingBrowser(null)
      setStatusMessage('Extensão conectada com sucesso!')
    }
  }

  const handleOpenLink = (url: string) => {
    window.open(url, '_blank', 'noopener,noreferrer')
  }

  if (isLoading && !state) {
    return (
      <div
        style={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          padding: 40,
          gap: 12
        }}
      >
        <SpinnerIcon size={24} />
        <span style={{ fontSize: 13, color: 'var(--text-secondary)' }}>
          Detectando navegadores e estado da extensão...
        </span>
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* Título e descrição */}
      <div>
        <h2 style={{ fontSize: 20, fontWeight: 600, color: 'var(--text-primary)', margin: 0 }}>
          Integração com o Navegador 🌐
        </h2>
        <p style={{ fontSize: 13, color: 'var(--text-secondary)', marginTop: 6, marginBottom: 0, lineHeight: 1.5 }}>
          Permita que o Gemini Desktop interaja diretamente com abas, faça pesquisas e capture telas via extensão nativa.
        </p>
      </div>

      {/* Badge de Dependência Externa: agy CLI */}
      {state && !state.agentFound && (
        <div
          style={{
            display: 'flex',
            alignItems: 'flex-start',
            gap: 12,
            padding: 14,
            borderRadius: 12,
            backgroundColor: 'rgba(234, 67, 53, 0.08)',
            border: '1px solid rgba(234, 67, 53, 0.25)'
          }}
        >
          <div style={{ fontSize: 18, marginTop: 1 }}>⚠️</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4, flex: 1 }}>
            <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>
              Antigravity CLI (agy) não detectado
            </span>
            <span style={{ fontSize: 12, color: 'var(--text-secondary)', lineHeight: 1.4 }}>
              Para automação completa e acesso aos modelos Gemini, instale o Antigravity CLI e faça login com sua conta Google.
            </span>
            <div style={{ marginTop: 4 }}>
              <button
                type="button"
                onClick={() => handleOpenLink('https://antigravity.google')}
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 6,
                  padding: '4px 10px',
                  borderRadius: 6,
                  backgroundColor: 'rgba(234, 67, 53, 0.15)',
                  border: '1px solid rgba(234, 67, 53, 0.3)',
                  color: 'var(--text-primary)',
                  fontSize: 12,
                  fontWeight: 500,
                  cursor: 'pointer'
                }}
              >
                <span>Baixar Antigravity CLI</span>
                <ExternalLinkIcon size={12} />
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Mensagem de Erro Geral */}
      {error && (
        <div
          style={{
            padding: 12,
            borderRadius: 10,
            backgroundColor: 'rgba(234, 67, 53, 0.1)',
            border: '1px solid rgba(234, 67, 53, 0.3)',
            color: '#f28b82',
            fontSize: 12,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between'
          }}
        >
          <span>{error}</span>
          <button
            type="button"
            onClick={() => setError(null)}
            style={{ background: 'transparent', border: 'none', color: 'inherit', cursor: 'pointer' }}
          >
            <CloseIcon size={14} />
          </button>
        </div>
      )}

      {/* Mensagem de Sucesso */}
      {statusMessage && (
        <div
          style={{
            padding: 12,
            borderRadius: 10,
            backgroundColor: 'rgba(52, 168, 83, 0.1)',
            border: '1px solid rgba(52, 168, 83, 0.3)',
            color: 'var(--status-completed)',
            fontSize: 13,
            fontWeight: 500,
            display: 'flex',
            alignItems: 'center',
            gap: 8
          }}
        >
          <CheckIcon size={16} />
          <span>{statusMessage}</span>
        </div>
      )}

      {/* Estado: Extensão Já Conectada com Sucesso */}
      {state && state.extensionConnected && !restartingBrowser && (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 14,
            padding: 16,
            borderRadius: 12,
            backgroundColor: 'rgba(52, 168, 83, 0.08)',
            border: '1px solid rgba(52, 168, 83, 0.25)'
          }}
        >
          <div
            style={{
              width: 36,
              height: 36,
              borderRadius: '50%',
              backgroundColor: 'rgba(52, 168, 83, 0.2)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: 'var(--status-completed)',
              flexShrink: 0
            }}
          >
            <CheckIcon size={20} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 2, flex: 1 }}>
            <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>
              Extensão conectada e ativa
            </span>
            <span style={{ fontSize: 12, color: 'var(--text-secondary)' }}>
              A ponte de automação está se comunicando em tempo real com o seu navegador.
            </span>
          </div>
          <button
            type="button"
            onClick={() => loadState(false)}
            title="Atualizar status"
            style={{
              padding: 6,
              borderRadius: 6,
              background: 'transparent',
              border: '1px solid var(--border-card)',
              color: 'var(--text-secondary)',
              cursor: 'pointer'
            }}
          >
            <RefreshIcon size={14} />
          </button>
        </div>
      )}

      {/* TELA DE REINÍCIO DO NAVEGADOR (quando acabamos de aplicar a política) */}
      {restartingBrowser ? (
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            gap: 16,
            padding: 20,
            borderRadius: 12,
            backgroundColor: 'var(--bg-card)',
            border: '1px solid var(--border-card)'
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <SpinnerIcon size={20} />
            <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
              <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--text-primary)' }}>
                Reinicie o {BROWSER_NAMES[restartingBrowser] || restartingBrowser}
              </span>
              <span style={{ fontSize: 12, color: 'var(--text-secondary)' }}>
                Feche e abra novamente todas as janelas do navegador para que a política seja carregada.
              </span>
            </div>
          </div>

          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '10px 14px',
              borderRadius: 8,
              backgroundColor: 'var(--bg-pill)',
              fontSize: 12,
              color: 'var(--text-secondary)'
            }}
          >
            <span>Aguardando conexão da extensão ({elapsedSeconds}s / 120s)...</span>
            <button
              type="button"
              onClick={handleCheckNow}
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: 6,
                padding: '4px 10px',
                borderRadius: 6,
                backgroundColor: 'var(--accent-blue)',
                border: 'none',
                color: '#ffffff',
                fontSize: 12,
                fontWeight: 500,
                cursor: 'pointer'
              }}
            >
              <RefreshIcon size={12} />
              <span>Verificar de novo</span>
            </button>
          </div>

          {/* Timeout de 120s ou botão de ajuda -> Fallback Guiado */}
          {(!showManualFallback && elapsedSeconds < 120) ? (
            <div style={{ textAlign: 'center', marginTop: -4 }}>
              <button
                type="button"
                onClick={() => setShowManualFallback(true)}
                style={{
                  background: 'transparent',
                  border: 'none',
                  color: 'var(--accent-blue)',
                  fontSize: 12,
                  cursor: 'pointer',
                  textDecoration: 'underline'
                }}
              >
                A extensão não conectou após reiniciar? Ver opções manuais
              </button>
            </div>
          ) : (
            <div
              style={{
                display: 'flex',
                flexDirection: 'column',
                gap: 12,
                padding: 14,
                borderRadius: 10,
                backgroundColor: 'rgba(255, 255, 255, 0.03)',
                border: '1px solid var(--border-card)'
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <TerminalIcon size={16} />
                <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>
                  Instalação Manual Alternativa (Fallback)
                </span>
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 12, color: 'var(--text-secondary)' }}>
                <span><strong>Opção 1: Carregar extensão descompactada (Load unpacked)</strong></span>
                <ol style={{ margin: 0, paddingLeft: 20, display: 'flex', flexDirection: 'column', gap: 4 }}>
                  <li>Abra seu navegador e acesse <code style={{ color: 'var(--text-primary)' }}>chrome://extensions</code> ou <code style={{ color: 'var(--text-primary)' }}>brave://extensions</code>.</li>
                  <li>Ative a chave <strong>Modo do desenvolvedor</strong> no canto superior direito.</li>
                  <li>Clique em <strong>Carregar sem compactação</strong> (Load unpacked).</li>
                  <li>Selecione o diretório: <code style={{ color: 'var(--text-primary)' }}>~/.browser-bridge/extension</code></li>
                </ol>
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 12, color: 'var(--text-secondary)', marginTop: 4 }}>
                <span><strong>Opção 2: Pelo terminal</strong></span>
                <div style={{ padding: '6px 10px', borderRadius: 6, backgroundColor: 'var(--bg-pill)', fontFamily: 'var(--font-mono)' }}>
                  browser-bridge install browsers
                </div>
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 4 }}>
                <button
                  type="button"
                  onClick={() => setRestartingBrowser(null)}
                  style={{
                    padding: '6px 12px',
                    borderRadius: 6,
                    backgroundColor: 'var(--bg-pill)',
                    border: '1px solid var(--border-card)',
                    color: 'var(--text-primary)',
                    fontSize: 12,
                    cursor: 'pointer'
                  }}
                >
                  Voltar para lista de navegadores
                </button>
              </div>
            </div>
          )}
        </div>
      ) : (
        /* LISTA DE NAVEGADORES DETECTADOS */
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-secondary)', textTransform: 'uppercase', letterSpacing: 0.5 }}>
              Navegadores suportados no sistema
            </span>
            <button
              type="button"
              onClick={() => loadState(false)}
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: 4,
                padding: '2px 8px',
                borderRadius: 4,
                backgroundColor: 'transparent',
                border: 'none',
                color: 'var(--text-muted)',
                fontSize: 11,
                cursor: 'pointer'
              }}
            >
              <RefreshIcon size={12} />
              <span>Recarregar</span>
            </button>
          </div>

          {state && state.browsers.map((b: BrowserWizardBrowser) => {
            const browserName = BROWSER_NAMES[b.browser] || b.browser
            const isInstallingThis = installingBrowser === b.browser

            return (
              <div
                key={b.browser}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '12px 14px',
                  borderRadius: 10,
                  backgroundColor: 'var(--bg-card)',
                  border: '1px solid var(--border-card)',
                  opacity: b.detected ? 1 : 0.65
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                  <GlobeIcon size={20} color={b.detected ? 'var(--accent-blue)' : 'var(--text-muted)'} />
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>
                        {browserName}
                      </span>
                      {b.detected ? (
                        <span
                          style={{
                            fontSize: 10,
                            padding: '1px 6px',
                            borderRadius: 4,
                            backgroundColor: 'rgba(52, 168, 83, 0.15)',
                            color: 'var(--status-completed)',
                            fontWeight: 500
                          }}
                        >
                          Detectado
                        </span>
                      ) : (
                        <span
                          style={{
                            fontSize: 10,
                            padding: '1px 6px',
                            borderRadius: 4,
                            backgroundColor: 'var(--bg-pill)',
                            color: 'var(--text-muted)',
                            fontWeight: 500
                          }}
                        >
                          Não detectado
                        </span>
                      )}
                    </div>
                    <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>
                      {b.policyApplied
                        ? 'Política de extensão instalada ✓'
                        : 'Política de extensão ausente'}
                    </span>
                  </div>
                </div>

                <div>
                  <button
                    type="button"
                    onClick={() => handleInstallPolicy(b.browser)}
                    disabled={isInstallingThis || installingBrowser !== null}
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: 6,
                      padding: '6px 12px',
                      borderRadius: 6,
                      backgroundColor: b.policyApplied ? 'var(--bg-pill)' : 'var(--accent-blue)',
                      border: b.policyApplied ? '1px solid var(--border-card)' : 'none',
                      color: b.policyApplied ? 'var(--text-primary)' : '#ffffff',
                      fontSize: 12,
                      fontWeight: 500,
                      cursor: isInstallingThis || installingBrowser !== null ? 'not-allowed' : 'pointer'
                    }}
                  >
                    {isInstallingThis && <SpinnerIcon size={12} />}
                    <span>
                      {b.policyApplied
                        ? 'Reinstalar política'
                        : 'Instalar extensão automaticamente'}
                    </span>
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Ações inferiores quando usado de forma autônoma (não modal) */}
      {!isEmbeddedInModal && (
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginTop: 8 }}>
          {onSkip && (
            <button
              type="button"
              onClick={onSkip}
              style={{
                background: 'transparent',
                border: 'none',
                color: 'var(--text-secondary)',
                fontSize: 13,
                cursor: 'pointer'
              }}
            >
              Pular por enquanto
            </button>
          )}
          {onComplete && (
            <button
              type="button"
              onClick={onComplete}
              style={{
                padding: '8px 18px',
                borderRadius: 8,
                backgroundColor: 'var(--accent-blue)',
                border: 'none',
                color: '#ffffff',
                fontSize: 13,
                fontWeight: 500,
                cursor: 'pointer'
              }}
            >
              Concluir
            </button>
          )}
        </div>
      )}
    </div>
  )
}
