import React from 'react'

interface DiffViewerProps {
  oldString?: string
  newString?: string
  filePath?: string
  maxHeight?: number | string
}

interface DiffLine {
  type: 'add' | 'remove' | 'same'
  text: string
  oldLineNumber?: number
  newLineNumber?: number
}

function computeSimpleDiff(oldText: string, newText: string): DiffLine[] {
  const oldLines = oldText ? oldText.split('\n') : []
  const newLines = newText ? newText.split('\n') : []

  // Se uma das versões estiver vazia
  if (oldLines.length === 0 && newLines.length > 0) {
    return newLines.map((l, i) => ({ type: 'add', text: l, newLineNumber: i + 1 }))
  }
  if (newLines.length === 0 && oldLines.length > 0) {
    return oldLines.map((l, i) => ({ type: 'remove', text: l, oldLineNumber: i + 1 }))
  }

  const lines: DiffLine[] = []
  let o = 0
  let n = 0

  while (o < oldLines.length || n < newLines.length) {
    if (o < oldLines.length && n < newLines.length) {
      if (oldLines[o] === newLines[n]) {
        lines.push({
          type: 'same',
          text: oldLines[o],
          oldLineNumber: o + 1,
          newLineNumber: n + 1
        })
        o++
        n++
      } else {
        // Encontra se a linha antiga existe adiante
        lines.push({
          type: 'remove',
          text: oldLines[o],
          oldLineNumber: o + 1
        })
        lines.push({
          type: 'add',
          text: newLines[n],
          newLineNumber: n + 1
        })
        o++
        n++
      }
    } else if (o < oldLines.length) {
      lines.push({
        type: 'remove',
        text: oldLines[o],
        oldLineNumber: o + 1
      })
      o++
    } else if (n < newLines.length) {
      lines.push({
        type: 'add',
        text: newLines[n],
        newLineNumber: n + 1
      })
      n++
    }
  }

  return lines
}

export const DiffViewer: React.FC<DiffViewerProps> = ({
  oldString = '',
  newString = '',
  filePath,
  maxHeight = 240
}) => {
  const diffLines = computeSimpleDiff(oldString, newString)

  return (
    <div
      style={{
        borderRadius: 8,
        border: '1px solid var(--border-subtle)',
        backgroundColor: 'var(--bg-code)',
        overflow: 'hidden',
        fontSize: 12,
        fontFamily: 'var(--font-mono)'
      }}
    >
      {filePath && (
        <div
          style={{
            padding: '6px 10px',
            backgroundColor: 'var(--bg-code-header)',
            borderBottom: '1px solid var(--border-subtle)',
            fontSize: 11,
            color: 'var(--text-secondary)',
            fontWeight: 500,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap'
          }}
          title={filePath}
        >
          {filePath}
        </div>
      )}

      <div
        style={{
          maxHeight,
          overflowY: 'auto',
          overflowX: 'auto',
          padding: '4px 0',
          lineHeight: 1.5
        }}
      >
        {diffLines.length === 0 ? (
          <div style={{ padding: '8px 12px', color: 'var(--text-muted)' }}>
            Sem alterações de conteúdo
          </div>
        ) : (
          diffLines.map((line, idx) => {
            const isAdd = line.type === 'add'
            const isRemove = line.type === 'remove'

            const bg = isAdd
              ? 'rgba(52, 168, 83, 0.15)'
              : isRemove
              ? 'rgba(234, 67, 53, 0.15)'
              : 'transparent'

            const textColor = isAdd
              ? '#81c995'
              : isRemove
              ? '#f28b82'
              : 'var(--text-primary)'

            const prefix = isAdd ? '+' : isRemove ? '-' : ' '

            return (
              <div
                key={idx}
                style={{
                  display: 'flex',
                  backgroundColor: bg,
                  padding: '1px 8px',
                  userSelect: 'text',
                  whiteSpace: 'pre'
                }}
              >
                <span
                  style={{
                    width: 16,
                    color: textColor,
                    fontWeight: isAdd || isRemove ? 600 : 400,
                    userSelect: 'none',
                    flexShrink: 0
                  }}
                >
                  {prefix}
                </span>
                <span style={{ color: textColor, flex: 1 }}>{line.text}</span>
              </div>
            )
          })
        )}
      </div>
    </div>
  )
}
