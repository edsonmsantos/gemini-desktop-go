import React from 'react'
import { FolderIcon } from './icons'
import { Workspace } from '../types'

interface WorkspaceListProps {
  workspaces: Workspace[]
  selectedCwd: string
  onSelectCwd: (cwd: string) => void
}

export const WorkspaceList: React.FC<WorkspaceListProps> = ({
  workspaces,
  selectedCwd,
  onSelectCwd
}) => {
  if (workspaces.length === 0) {
    return null
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 2, padding: '4px 10px' }}>
      <div
        style={{
          fontSize: 12,
          fontWeight: 500,
          color: 'var(--text-muted)',
          padding: '6px 8px',
          textTransform: 'uppercase',
          letterSpacing: '0.04em'
        }}
      >
        Workspaces
      </div>
      {workspaces.map((ws) => {
        const isSelected = selectedCwd === ws.cwd
        const basename = ws.cwd.split(/[/\\]/).filter(Boolean).pop() || ws.cwd

        return (
          <button
            key={ws.cwd}
            onClick={() => onSelectCwd(ws.cwd)}
            title={ws.cwd}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 10,
              padding: '7px 10px',
              borderRadius: 8,
              backgroundColor: isSelected ? 'var(--bg-sidebar-active)' : 'transparent',
              color: isSelected ? 'var(--text-primary)' : 'var(--text-secondary)',
              textAlign: 'left',
              transition: 'background 0.15s, color 0.15s',
              width: '100%',
              overflow: 'hidden'
            }}
            onMouseEnter={(e) => {
              if (!isSelected) {
                e.currentTarget.style.backgroundColor = 'var(--bg-sidebar-hover)'
                e.currentTarget.style.color = 'var(--text-primary)'
              }
            }}
            onMouseLeave={(e) => {
              if (!isSelected) {
                e.currentTarget.style.backgroundColor = 'transparent'
                e.currentTarget.style.color = 'var(--text-secondary)'
              }
            }}
          >
            <FolderIcon size={15} color={isSelected ? 'var(--accent-blue)' : 'var(--text-muted)'} />
            <div style={{ display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
              <span
                style={{
                  fontSize: 13,
                  fontWeight: isSelected ? 500 : 400,
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis'
                }}
              >
                {basename}
              </span>
              <span
                style={{
                  fontSize: 11,
                  color: 'var(--text-muted)',
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis'
                }}
              >
                {ws.cwd}
              </span>
            </div>
          </button>
        )
      })}
    </div>
  )
}
