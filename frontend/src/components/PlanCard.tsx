import React from 'react'
import { CheckIcon } from './icons'
import { Plan } from '../types'

interface PlanCardProps {
  plan: Plan
}

export const PlanCard: React.FC<PlanCardProps> = ({ plan }) => {
  if (!plan.entries || plan.entries.length === 0) return null

  return (
    <div
      style={{
        margin: '12px 0',
        borderRadius: 10,
        border: '1px solid var(--border-card)',
        backgroundColor: 'var(--bg-card)',
        padding: '12px 14px',
        display: 'flex',
        flexDirection: 'column',
        gap: 10
      }}
    >
      <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>
        Plano de Etapas
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        {plan.entries.map((entry, idx) => {
          const isDone = entry.status === 'completed' || entry.status === 'done'
          const isInProgress = entry.status === 'in_progress'

          return (
            <div
              key={idx}
              style={{
                display: 'flex',
                alignItems: 'flex-start',
                gap: 10,
                fontSize: 13,
                color: isDone ? 'var(--text-muted)' : 'var(--text-primary)'
              }}
            >
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  width: 18,
                  height: 18,
                  borderRadius: 4,
                  marginTop: 2,
                  backgroundColor: isDone
                    ? 'var(--status-completed)'
                    : isInProgress
                    ? 'var(--status-progress)'
                    : 'transparent',
                  border: isDone || isInProgress ? 'none' : '1px solid var(--border-card)',
                  color: '#ffffff',
                  flexShrink: 0
                }}
              >
                {isDone && <CheckIcon size={12} />}
                {isInProgress && (
                  <span
                    className="pulsing"
                    style={{
                      width: 6,
                      height: 6,
                      borderRadius: '50%',
                      backgroundColor: '#ffffff'
                    }}
                  />
                )}
              </div>
              <span
                style={{
                  textDecoration: isDone ? 'line-through' : 'none',
                  lineHeight: 1.4
                }}
              >
                {entry.content}
              </span>
            </div>
          )
        })}
      </div>
    </div>
  )
}
