import { createContext, useContext, useReducer, useCallback, type ReactNode } from 'react'

export type ToastType = 'success' | 'error' | 'info'

export type Toast = {
  id: number
  type: ToastType
  message: string
}

let nextId = 1

type ToastContextValue = {
  toasts: Toast[]
  remove: (id: number) => void
  success: (message: string) => void
  error: (message: string) => void
  info: (message: string) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

type Action =
  | { type: 'PUSH'; toast: Toast }
  | { type: 'REMOVE'; id: number }

function reducer(state: Toast[], action: Action): Toast[] {
  switch (action.type) {
    case 'PUSH':
      return [...state, action.toast]
    case 'REMOVE':
      return state.filter((t) => t.id !== action.id)
  }
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, dispatch] = useReducer(reducer, [])

  const remove = useCallback((id: number) => dispatch({ type: 'REMOVE', id }), [])

  const push = useCallback((type: ToastType, message: string, ttlMs = 2600) => {
    const id = nextId++
    dispatch({ type: 'PUSH', toast: { id, type, message } })
    setTimeout(() => remove(id), ttlMs)
  }, [remove])

  const success = useCallback((msg: string) => push('success', msg), [push])
  const error = useCallback((msg: string) => push('error', msg, 3600), [push])
  const info = useCallback((msg: string) => push('info', msg), [push])

  return (
    <ToastContext.Provider value={{ toasts, remove, success, error, info }}>
      {children}
    </ToastContext.Provider>
  )
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within ToastProvider')
  return ctx
}
