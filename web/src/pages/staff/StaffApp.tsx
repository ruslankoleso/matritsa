import { useEffect, useRef, useState } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../../api/client'
import { AppShell, type BellInfo, type NavEntry } from '../../design/AppShell'
import { IconQueue } from '../../design/icons'
import { ToastProvider, useToast } from '../../design/Toast'
import { useLiveUpdates } from '../../lib/events'
import { formatMoneyShort, waitingText } from '../../lib/format'
import Queue from './Queue'
import StaffApplicationPage from './StaffApplicationPage'
import Profile from '../Profile'
import '../app.css'

function Staff() {
  const toast = useToast()
  useLiveUpdates((e) => {
    if (e.type === 'queue' && e.status === 'ANALYST_REVIEW') toast('Новая заявка ждёт вашего решения')
  })
  const counts = useQuery({ queryKey: ['queue', '', ''], queryFn: () => api.queue(), refetchInterval: 30_000 })
  const waiting = counts.data?.counts['ANALYST_REVIEW'] ?? 0
  const [ring, setRing] = useState(0)
  const prev = useRef<number | undefined>(undefined)
  useEffect(() => {
    if (prev.current !== undefined && waiting > prev.current) setRing((r) => r + 1)
    prev.current = waiting
  }, [waiting])
  const nav: NavEntry[] = [{ to: '/staff', label: 'Очередь заявок', icon: <IconQueue />, end: true, count: waiting }]
  const bell: BellInfo = {
    count: waiting,
    ring,
    title: 'Ждут решения',
    items: (counts.data?.items ?? []).filter((q) => q.status === 'ANALYST_REVIEW').slice(0, 4).map((q) => ({
      id: q.id, text: q.companyName, meta: `${q.productTitle}, ${formatMoneyShort(q.amount)} · ${waitingText(q.waitingMinutes)}`, to: `/staff/applications/${q.id}`, unread: true,
    })),
    allTo: '/staff',
    allLabel: 'Открыть очередь заявок',
    empty: 'Все заявки разобраны',
  }
  return (
    <AppShell variant="rail" nav={nav} bell={bell}>
      <Routes>
        <Route index element={<Queue />} />
        <Route path="applications/:id" element={<StaffApplicationPage />} />
        <Route path="profile" element={<Profile />} />
        <Route path="*" element={<Navigate to="/staff" replace />} />
      </Routes>
    </AppShell>
  )
}

export default function StaffApp() {
  return (
    <ToastProvider>
      <Staff />
    </ToastProvider>
  )
}
