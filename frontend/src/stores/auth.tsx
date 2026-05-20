import { createContext, useContext, useReducer, useEffect, useRef, useCallback, type ReactNode } from 'react'
import { decodeJwtPayload, type JwtPayload } from '../utils/jwt'
import { registerAuthGetter } from '../api/client'

const ACCESS_KEY = 'access_token'
const REFRESH_KEY = 'refresh_token'

function readStored(key: string): string | null {
  try { return localStorage.getItem(key) } catch { return null }
}

function writeStored(key: string, value: string) {
  localStorage.setItem(key, value)
}

function removeStored(key: string) {
  localStorage.removeItem(key)
}

export type AuthState = {
  token: string | null
  refreshToken: string | null
  isLoggedIn: boolean
  claims: JwtPayload | null
}

type Action =
  | { type: 'SET_TOKEN'; token: string }
  | { type: 'SET_TOKENS'; token: string; refreshToken: string }
  | { type: 'CLEAR_TOKENS' }

function reducer(state: AuthState, action: Action): AuthState {
  switch (action.type) {
    case 'SET_TOKEN':
      writeStored(ACCESS_KEY, action.token)
      return {
        ...state,
        token: action.token,
        isLoggedIn: true,
        claims: decodeJwtPayload(action.token),
      }
    case 'SET_TOKENS':
      writeStored(ACCESS_KEY, action.token)
      writeStored(REFRESH_KEY, action.refreshToken)
      return {
        token: action.token,
        refreshToken: action.refreshToken,
        isLoggedIn: true,
        claims: decodeJwtPayload(action.token),
      }
    case 'CLEAR_TOKENS':
      removeStored(ACCESS_KEY)
      removeStored(REFRESH_KEY)
      return { token: null, refreshToken: null, isLoggedIn: false, claims: null }
  }
}

type AuthContextValue = AuthState & {
  setToken: (token: string) => void
  setTokens: (token: string, refreshToken: string) => void
  clearTokens: () => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(reducer, undefined, () => {
    const token = readStored(ACCESS_KEY)
    const refreshToken = readStored(REFRESH_KEY)
    return {
      token,
      refreshToken,
      isLoggedIn: !!token,
      claims: token ? decodeJwtPayload(token) : null,
    }
  })

  const setToken = useCallback((token: string) => dispatch({ type: 'SET_TOKEN', token }), [])
  const setTokens = useCallback((token: string, refreshToken: string) => dispatch({ type: 'SET_TOKENS', token, refreshToken }), [])
  const clearTokens = useCallback(() => dispatch({ type: 'CLEAR_TOKENS' }), [])

  // Keep a ref so the API client always reads the latest state
  const stateRef = useRef(state)
  stateRef.current = state

  useEffect(() => {
    registerAuthGetter(() => ({
      token: stateRef.current.token,
      refreshToken: stateRef.current.refreshToken,
      setToken,
      setTokens,
      clearTokens,
    }))
  }, [setToken, setTokens, clearTokens])

  const value: AuthContextValue = { ...state, setToken, setTokens, clearTokens }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
