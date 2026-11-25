import React from 'react'
import { PromptBar } from './PromptBar'
import { Model } from '../types'

interface WelcomeViewProps {
  prompt: string
  onChangePrompt: (val: string) => void
  onSend: () => void
  isStreaming: boolean
  models: Model[]
  selectedModelId: string
  onSelectModel: (id: string) => void
  onCancel: () => void
  files?: string[]
  onAddFiles?: (files: string[]) => void
  onRemoveFile?: (index: number) => void
}

export const WelcomeView: React.FC<WelcomeViewProps> = ({
  prompt,
  onChangePrompt,
  onSend,
  isStreaming,
  models,
  selectedModelId,
  onSelectModel,
  onCancel,
  files = [],
  onAddFiles,
  onRemoveFile
}) => {
  return (
    <div
      style={{
        position: 'relative',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        flex: 1,
        height: '100%',
        padding: '0 24px',
        overflow: 'hidden'
      }}
    >
      {/* Glow radial azul característico na base (fiel à captura do Gemini) */}
      <div
        style={{
          position: 'absolute',
          bottom: 0,
          left: 0,
          right: 0,
          height: '60%',
          background: 'var(--accent-glow)',
          pointerEvents: 'none',
          zIndex: 0
        }}
      />

      <div
        style={{
          position: 'relative',
          zIndex: 1,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          gap: 36,
          width: '100%',
          maxWidth: 820,
          marginTop: -60
        }}
      >
        <h1
          style={{
            fontSize: 34,
            fontWeight: 400,
            color: 'var(--text-primary)',
            letterSpacing: '-0.02em',
            textAlign: 'center'
          }}
        >
          Por onde começamos?
        </h1>

        <div style={{ width: '100%' }}>
          <PromptBar
            prompt={prompt}
            onChangePrompt={onChangePrompt}
            onSend={onSend}
            onCancel={onCancel}
            isStreaming={isStreaming}
            models={models}
            selectedModelId={selectedModelId}
            onSelectModel={onSelectModel}
            files={files}
            onAddFiles={onAddFiles}
            onRemoveFile={onRemoveFile}
          />
        </div>
      </div>
    </div>
  )
}
