import { createContext, useContext, useReducer, useCallback, useEffect, type ReactNode } from 'react'
import * as socialApi from '../api/social'
import { useAuth } from './auth'

export type SocialState = {
  followers: { id: number; username: string; avatar_url?: string }[]
  vloggers: { id: number; username: string; avatar_url?: string }[]
  loading: boolean
}

type Action =
  | { type: 'SET_FOLLOWERS'; followers: SocialState['followers'] }
  | { type: 'SET_VLOGGERS'; vloggers: SocialState['vloggers'] }
  | { type: 'SET_LOADING'; loading: boolean }
  | { type: 'CLEAR' }

function reducer(state: SocialState, action: Action): SocialState {
  switch (action.type) {
    case 'SET_FOLLOWERS':
      return { ...state, followers: action.followers }
    case 'SET_VLOGGERS':
      return { ...state, vloggers: action.vloggers }
    case 'SET_LOADING':
      return { ...state, loading: action.loading }
    case 'CLEAR':
      return { followers: [], vloggers: [], loading: false }
  }
}

type SocialContextValue = SocialState & {
  isFollowing: (accountId: number) => boolean
  refreshMine: () => Promise<void>
  follow: (id: number) => Promise<void>
  unfollow: (id: number) => Promise<void>
  clear: () => void
}

const SocialContext = createContext<SocialContextValue | null>(null)

export function SocialProvider({ children }: { children: ReactNode }) {
  const auth = useAuth()
  const [state, dispatch] = useReducer(reducer, { followers: [], vloggers: [], loading: false })

  const isFollowing = useCallback(
    (accountId: number) => state.vloggers.some((v) => v.id === accountId),
    [state.vloggers],
  )

  const refreshMine = useCallback(async () => {
    if (!auth.isLoggedIn) return
    dispatch({ type: 'SET_LOADING', loading: true })
    try {
      const [fRes, vRes] = await Promise.all([socialApi.getAllFollowers(), socialApi.getAllVloggers()])
      dispatch({ type: 'SET_FOLLOWERS', followers: fRes.followers })
      dispatch({ type: 'SET_VLOGGERS', vloggers: vRes.vloggers })
    } finally {
      dispatch({ type: 'SET_LOADING', loading: false })
    }
  }, [auth.isLoggedIn])

  const follow = useCallback(async (id: number) => {
    await socialApi.follow(id)
    await refreshMine()
  }, [refreshMine])

  const unfollow = useCallback(async (id: number) => {
    await socialApi.unfollow(id)
    await refreshMine()
  }, [refreshMine])

  const clear = useCallback(() => dispatch({ type: 'CLEAR' }), [])

  useEffect(() => {
    if (auth.isLoggedIn) refreshMine()
    else clear()
  }, [auth.isLoggedIn, refreshMine, clear])

  return (
    <SocialContext.Provider value={{ ...state, isFollowing, refreshMine, follow, unfollow, clear }}>
      {children}
    </SocialContext.Provider>
  )
}

export function useSocial(): SocialContextValue {
  const ctx = useContext(SocialContext)
  if (!ctx) throw new Error('useSocial must be used within SocialProvider')
  return ctx
}
