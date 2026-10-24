import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react'
import { api, clearSession, loadSession, saveSession, type Session } from '../api/client'
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

  const login = useCallback(async (loginName: string, password: string) => {
    const s = await api.login(loginName, password)
    saveSession(s)
    setSession(s)
    return s
  }, [])

  const register = useCallback(async (form: RegisterRequest) => {
    const s = await api.register(form)
    saveSession(s)
    setSession(s)
    return s
  }, [])

  const logout = useCallback(() => {
    clearSession()
    setSession(null)
  }, [])

  const value = useMemo(() => ({ session, login, register, logout }), [session, login, register, logout])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export const useAuth = () => useContext(AuthContext)
