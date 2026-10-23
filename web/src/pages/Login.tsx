import { useState, type FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { homeFor } from '../auth/RequireRole'
import { ApiError } from '../api/client'
import { Button } from '../design/Button'
import { FloatField } from '../design/FloatField'
import './login.css'

export const DEMO_CLIENTS = [
  { login: 'client1', inn: '7701234501', name: 'ООО «Тёплый хлеб»', story: 'Надёжная пекарня: решение приходит автоматически' },
  { login: 'client2', inn: '771234500002', name: 'ИП Смирнов А. В., кофейня «Гараж»', story: 'Закредитованный ИП: сотрудник предложит меньшую сумму' },
  { login: 'client3', inn: '7701234503', name: 'ООО «Новый старт»', story: 'Молодая компания без истории: заявку смотрит сотрудник' },
  { login: 'client4', inn: '7701234504', name: 'ООО «Старый порт»', story: 'Ликвидированная компания: автоматический отказ с причинами' },
  { login: 'client5', inn: '771234500005', name: 'ИП Кузнецова Е. С., салон «Лотос»', story: 'Пограничный случай: решение принимает сотрудник' },
  { login: 'client6', inn: '7701234506', name: 'ООО «ТехноСтрой»', story: 'Просрочка более 90 дней: отказ по жёсткому правилу' },
]

export default function Login() {
  const { login } = useAuth()
  const nav = useNavigate()
  const [params] = useSearchParams()
  const inn = params.get('inn') ?? ''
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const signIn = async (l: string, p: string) => {
    setBusy(true)
    setError('')
    try {
      const s = await login(l, p)
      nav(homeFor(s.role), { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось войти. Попробуйте ещё раз.')
    } finally {
      setBusy(false)
    }
  }

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !password) {
      setError('Введите логин и пароль')
      return
    }
    void signIn(name.trim(), password)
  }

  return (
    <div className="login">
      <main className="login-main">
        <Link to="/" className="wordmark" style={{ display: 'inline-flex', alignItems: 'center', gap: 10, fontSize: 22, fontWeight: 700, textDecoration: 'none' }}>
          <span className="wordmark-dot" aria-hidden style={{ position: 'relative', width: 24, height: 24, borderRadius: '50%', background: 'var(--red-500)', display: 'inline-block' }}>
            <span style={{ position: 'absolute', inset: 7, borderRadius: '50%', background: '#fff' }} />
          </span>
          Матрица
        </Link>
        <h1>Вход в кабинет</h1>
        <p>Для клиентов банка и сотрудников. Письма и SMS не нужны — только логин и пароль.</p>
        <form className="login-form" onSubmit={onSubmit} noValidate>
          {error && <div className="login-error" role="alert">{error}</div>}
          <FloatField id="login" label="Логин" autoComplete="username" autoFocus value={name} onChange={setName} />
          <FloatField id="password" label="Пароль" type="password" autoComplete="current-password" value={password} onChange={setPassword} />
          <Button type="submit" size="lg" block busy={busy}>Войти</Button>
        </form>
        <p className="login-back"><Link to="/">← На главную</Link></p>
      </main>

      <aside className="login-demo" aria-labelledby="demo-h">
        <h2 id="demo-h">Демо-доступы для проверки</h2>
        <p>Все компании синтетические. У каждой своя история — так видно разные исходы: автоматическое решение, ручная проверка, отказ. Пароль у всех: demo1234.</p>
        {inn && DEMO_CLIENTS.some((c) => c.inn === inn) && (
          <p style={{ marginTop: 14, fontWeight: 500, color: 'var(--red-700)' }}>
            Для ИНН {inn} есть демо-доступ — он подсвечен ниже.
          </p>
        )}
        <div className="demo-group">Предприниматели</div>
        <div className="demo-list">
          {DEMO_CLIENTS.map((c) => (
            <button type="button" key={c.login} className="demo-item" data-hit={c.inn === inn} disabled={busy} onClick={() => void signIn(c.login, 'demo1234')}>
              <span style={{ margin: 0 }}>
                <b>{c.name}</b>
                <span>{c.story}</span>
              </span>
              <span className="go" aria-hidden>→</span>
            </button>
          ))}
        </div>
        <div className="demo-group">Банк</div>
        <div className="demo-list">
          <button type="button" className="demo-item" disabled={busy} onClick={() => void signIn('staff1', 'demo1234')}>
            <span style={{ margin: 0 }}>
              <b>Анна Белова, кредитный аналитик</b>
              <span>Очередь заявок, разбор оценки, решение и встречные предложения</span>
            </span>
            <span className="go" aria-hidden>→</span>
          </button>
        </div>
      </aside>
    </div>
  )
}
