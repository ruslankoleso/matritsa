import { useEffect, useRef, useState } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../../api/client'
import { AppShell, type BellInfo, type NavEntry } from '../../design/AppShell'
import { IconChat } from '../../design/icons'
import { ToastProvider, useToast } from '../../design/Toast'
import { useLiveUpdates } from '../../lib/events'
import { waitingText } from '../../lib/format'
import Profile from '../Profile'
import Chats from './Chats'
import '../app.css'

function Support() {
  const toast = useToast()
  useLiveUpdates()
  const list = useQuery({ queryKey: ['support', 'list'], queryFn: api.supportChats, refetchInterval: 15_000 })
  const waiting = list.data?.waiting ?? 0
  const [ring, setRing] = useState(0)
  const prev = useRef<number | undefined>(undefined)
  useEffect(() => {
    if (prev.current !== undefined && waiting > prev.current) {
      setRing((r) => r + 1)
      toast('Клиент ждёт сотрудника в чате')
    }
    prev.current = waiting
  }, [waiting, toast])

  const nav: NavEntry[] = [{ to: '/support', label: 'Обращения', icon: <IconChat />, end: false, count: waiting }]
  const bell: BellInfo = {
    count: waiting,
    ring,
    title: 'Ждут сотрудника',
    items: (list.data?.items ?? []).filter((c) => c.status === 'waiting').slice(0, 4).map((c) => ({
      id: c.id, text: c.companyName || c.clientName, meta: `${c.lastText || 'Запрос на разговор'} · ${waitingText(c.waitingMinutes)}`, to: `/support/chats/${c.id}`, unread: true,
    })),
    allTo: '/support',
    allLabel: 'Открыть обращения',
    empty: 'Никто не ждёт ответа',
  }
  return (
    <AppShell variant="rail" nav={nav} bell={bell}>
      <Routes>
        <Route index element={<Chats />} />
        <Route path="chats/:id" element={<Chats />} />
        <Route path="profile" element={<Profile />} />
        <Route path="*" element={<Navigate to="/support" replace />} />
      </Routes>
    </AppShell>
  )
}

export default function SupportApp() {
  return (
    <ToastProvider>
      <Support />
    </ToastProvider>
  )
}
