import { useEffect, useRef, useState } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { AppShell, type NavEntry } from '../../design/AppShell'
import { IconBell, IconHome, IconList, IconPlus, IconUser } from '../../design/icons'
import { ToastProvider, useToast } from '../../design/Toast'
import { useLiveUpdates } from '../../lib/events'
import { useMe, useNotifications } from './queries'
import Home from './Home'
import Wizard from './Wizard'
import ApplicationPage from './ApplicationPage'
import ApplicationsList from './ApplicationsList'
import Notifications from './Notifications'
import Profile from '../Profile'
import '../app.css'

function Cabinet() {
  const toast = useToast()
  useLiveUpdates((e) => {
    if (e.type === 'notification' && e.text) toast(e.text)
  })
  const me = useMe()
  const notes = useNotifications()
  const unread = notes.data?.filter((n) => !n.read).length ?? 0
  const [ring, setRing] = useState(0)
  const prev = useRef<number | undefined>(undefined)
  useEffect(() => {
    if (prev.current !== undefined && unread > prev.current) setRing((r) => r + 1)
    prev.current = unread
  }, [unread])
  const nav: NavEntry[] = [
    { to: '/cabinet', label: 'Главная', icon: <IconHome />, end: true },
    { to: '/cabinet/new', label: 'Новая заявка', icon: <IconPlus /> },
    { to: '/cabinet/applications', label: 'Мои заявки', icon: <IconList /> },
    { to: '/cabinet/notifications', label: 'Уведомления', icon: <IconBell />, count: unread, ring },
    { to: '/cabinet/profile', label: 'Профиль', icon: <IconUser /> },
  ]
  return (
    <AppShell scope={me.data?.company?.name ?? 'Кабинет предпринимателя'} scopeName={me.data?.company?.name ?? 'Кабинет'} nav={nav}>
      <Routes>
        <Route index element={<Home />} />
        <Route path="new" element={<Wizard />} />
        <Route path="applications" element={<ApplicationsList />} />
        <Route path="applications/:id" element={<ApplicationPage />} />
        <Route path="notifications" element={<Notifications />} />
        <Route path="profile" element={<Profile />} />
        <Route path="*" element={<Navigate to="/cabinet" replace />} />
      </Routes>
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
