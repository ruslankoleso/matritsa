import { lazy, Suspense } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import Landing from './pages/landing/Landing'
import Login from './pages/Login'
import { RequireRole } from './auth/RequireRole'

const Cabinet = lazy(() => import('./pages/cabinet/CabinetApp'))
const Staff = lazy(() => import('./pages/staff/StaffApp'))
const Support = lazy(() => import('./pages/support/SupportApp'))

const Loading = () => (
  <div style={{ minHeight: '100svh', display: 'grid', placeItems: 'center' }}>
    <span className="spinner" aria-label="Загрузка" />
  </div>
)

export default function App() {
  return (
    <Suspense fallback={<Loading />}>
      <Routes>
        <Route path="/" element={<Landing />} />
        <Route path="/login" element={<Login />} />
        <Route path="/cabinet/*" element={<RequireRole role="client"><Cabinet /></RequireRole>} />
        <Route path="/staff/*" element={<RequireRole role="staff"><Staff /></RequireRole>} />
        <Route path="/support/*" element={<RequireRole role="support"><Support /></RequireRole>} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Suspense>
  )
}
