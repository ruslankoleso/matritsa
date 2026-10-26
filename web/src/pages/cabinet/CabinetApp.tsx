import { useEffect, useRef, useState } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api/client'
import { AppShell, type BellInfo, type NavEntry } from '../../design/AppShell'
import { ChatWidget } from '../../design/ChatWidget'
import { IconHome, IconList, IconPlus } from '../../design/icons'
import { ToastProvider, useToast } from '../../design/Toast'
import { useLiveUpdates } from '../../lib/events'
import { formatDateTime } from '../../lib/format'
import { useNotifications } from './queries'
import Home from './Home'
import Wizard from './Wizard'
import ApplicationPage from './ApplicationPage'
import ApplicationsList from './ApplicationsList'
import Notifications from './Notifications'
import Profile from '../Profile'
import '../app.css'

function Cabinet() {
  const toast = useToast()
  const qc = useQueryClient()
  useLiveUpdates((e) => {
    if (e.type === 'notification' && e.text) toast(e.text)
  })
  const notes = useNotifications()
  const read = useMutation({ mutationFn: api.markRead, onSuccess: () => qc.invalidateQueries({ queryKey: ['notifications'] }) })
  const unread = notes.data?.filter((n) => !n.read).length ?? 0
  const [ring, setRing] = useState(0)
  const prev = useRef<number | undefined>(undefined)
  useEffect(() => {
    if (prev.current !== undefined && unread > prev.current) setRing((r) => r + 1)
    prev.current = unread
  }, [unread])

  const sections: NavEntry[] = [
    { to: '/cabinet', label: 'Главная', icon: <IconHome />, end: true },
    { to: '/cabinet/new', label: 'Новая заявка', icon: <IconPlus />, match: (path, q) => path === '/cabinet/new' || (path.startsWith('/cabinet/applications/') && q.has('step')) },
    { to: '/cabinet/applications', label: 'Мои заявки', icon: <IconList />, match: (path, q) => path.startsWith('/cabinet/applications') && !q.has('step') },
  ]
  const bell: BellInfo = {
    count: unread,
    ring,
    title: 'Уведомления',
    items: (notes.data ?? []).slice(0, 4).map((n) => ({
      id: n.id, text: n.text, meta: formatDateTime(n.createdAt), unread: !n.read,
      to: n.applicationId ? `/cabinet/applications/${n.applicationId}` : '/cabinet/notifications',
    })),
    allTo: '/cabinet/notifications',
    allLabel: 'Открыть все уведомления',
    empty: 'Новых уведомлений нет',
    onOpenItem: (it) => {
      if (it.unread) read.mutate(it.id)
    },
  }
  return (
    <AppShell variant="top" nav={sections} bell={bell}>
      <Routes>
        <Route index element={<Home />} />
        <Route path="new" element={<Wizard />} />
        <Route path="applications" element={<ApplicationsList />} />
        <Route path="applications/:id" element={<ApplicationPage />} />
        <Route path="notifications" element={<Notifications />} />
        <Route path="profile" element={<Profile />} />
        <Route path="*" element={<Navigate to="/cabinet" replace />} />
      </Routes>
      <ChatWidget />
    </AppShell>
  )
}

export default function CabinetApp() {
  return (
    <ToastProvider>
      <Cabinet />
    </ToastProvider>
  )
}
