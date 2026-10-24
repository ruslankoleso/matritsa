import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api, clearSession, loadSession, onSessionChange, saveSession, type Session } from '../api/client'
import { queryClient } from '../api/queryClient'
import type { RegisterRequest } from '../api/types'

export interface AuthValue {
  session: Session | null
  login: (login: string, password: string) => Promise<Session>
  register: (form: RegisterRequest) => Promise<Session>
  logout: () => void
}

export const AuthContext = createContext<AuthValue>({
  session: null,
  login: async () => {
    throw new Error('AuthProvider is missing')
  },
  register: async () => {
    throw new Error('AuthProvider is missing')
  },
  logout: () => {},
})

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session | null>(() => loadSession())

  // The cache holds the previous person's data (company, applications). It must never outlive the session.
  useEffect(() => onSessionChange((s) => {
    if (!s) {
      queryClient.clear()
      setSession(null)
    }
  }), [])

  const login = useCallback(async (loginName: string, password: string) => {
    const s = await api.login(loginName, password)
    queryClient.clear()
    saveSession(s)
    setSession(s)
    return s
  }, [])

  const register = useCallback(async (form: RegisterRequest) => {
    const s = await api.register(form)
    queryClient.clear()
    saveSession(s)
    setSession(s)
    return s
  }, [])

  const logout = useCallback(() => {
    clearSession()
    queryClient.clear()
    setSession(null)
  }, [])

  const value = useMemo(() => ({ session, login, register, logout }), [session, login, register, logout])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export const useAuth = () => useContext(AuthContext)
