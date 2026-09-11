import { useState } from 'react'
import { MASCOT_ICONS } from '#/components/mascotIcons'
import type { ToastKind, ToastMessage } from '#/data/store'
import { useAppStore } from '#/data/store'

const KIND_STYLE: Record<ToastKind, { background: string; border: string }> = {
  error: { background: '#3a1a1e', border: '#c85a5a' },
  warning: { background: '#3a301a', border: '#d9a441' },
  info: { background: '#1b1d2e', border: 'rgba(233,233,237,.14)' },
}

function ToastOverlay({ id, kind, text }: ToastMessage) {
  const store = useAppStore()
  const [Icon] = useState(
    () => MASCOT_ICONS[Math.floor(Math.random() * MASCOT_ICONS.length)],
  )
  const style = KIND_STYLE[kind]

  return (
    <div
      className="fixed inset-0 z-50 grid place-items-center"
      style={{ background: 'rgba(16,17,32,.62)', cursor: 'pointer' }}
      onClick={() => store.dismissToast(id)}
    >
      <div
        className="card elev-lg flex flex-col items-center"
        style={{
          background: style.background,
          border: `1px solid ${style.border}`,
          padding: '28px 36px',
          gap: 14,
        }}
      >
        <Icon size={110} />
        <span
          className="mono"
          style={{ fontSize: 14, color: '#e9e9ed', textAlign: 'center' }}
        >
          {text}
        </span>
        <button
          className="btn btn-primary mono"
          onClick={() => store.dismissToast(id)}
        >
          kk bae!
        </button>
      </div>
    </div>
  )
}

export function ToastStack() {
  const store = useAppStore()
  const toast = store.toasts.at(0)
  if (!toast) return null

  return <ToastOverlay key={toast.id} {...toast} />
}
