import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { homeFor } from '../auth/RequireRole'
import { ApiError, api } from '../api/client'
import Register from './Register'
import type { RegistrationExample } from '../api/types'
import { Button } from '../design/Button'
import { FloatField } from '../design/FloatField'
import './login.css'

export const DEMO_CLIENTS = [
  { login: 'client1', inn: '7701234501', name: 'ООО «Тёплый хлеб»', story: 'Надёжная пекарня: один кредит выдан, новое автоматическое предложение ждёт решения клиента' },
  { login: 'client2', inn: '771234500002', name: 'ИП Смирнов А. В., кофейня «Гараж»', story: 'Закредитованный ИП: одна заявка у сотрудника, по другой есть встречное предложение' },
  { login: 'client3', inn: '7701234503', name: 'ООО «Новый старт»', story: 'Молодая компания без истории: сотрудник запросил уточнение' },
  { login: 'client4', inn: '7701234504', name: 'ООО «Старый порт»', story: 'Автоматический отказ: компания ликвидирована' },
  { login: 'client5', inn: '771234500005', name: 'ИП Кузнецова Е. С., салон «Лотос»', story: 'Пограничный случай: две заявки (1,5 и 20 млн ₽) у сотрудника, класс заёмщика один' },
  { login: 'client6', inn: '7701234506', name: 'ООО «ТехноСтрой»', story: 'Автоматический отказ: просрочка более 90 дней' },
  { login: 'client7', inn: '7701234507', name: 'ООО «Балтийский лён»', story: 'Сильный заёмщик, сумма выше лимита автоматического решения: рассматривает сотрудник' },
  { login: 'client8', inn: '771234500008', name: 'ИП Орлов К. А., мастерская «Дуб»', story: 'Мало опыта погашения: заявка у сотрудника; вторую сотрудник отклонил' },
  { login: 'client9', inn: '7701234509', name: 'ООО «Вектор Логистик»', story: 'Автоматический отказ: платежи по кредитам не оставляют запаса' },
  { login: 'client10', inn: '7701234510', name: 'ООО «Гранд Строй»', story: 'Автоматический отказ: процедура банкротства' },
  { login: 'client11', inn: '771234500011', name: 'ИП Васильева М. Н., студия «Лад»', story: 'Полная история: выдача, отказ от предложения, отзыв заявки и черновик' },
]

function Examples({ onPick }: { onPick: (inn: string) => void }) {
  const [list, setList] = useState<RegistrationExample[]>([])
  useEffect(() => {
    api.registrationExamples().then(setList, () => setList([]))
  }, [])
  return (
    <>
      <h2 id="demo-h">Тестовые ИНН для регистрации</h2>
      <p>Все компании синтетические. Для каждого ИНН указан тип компании и ожидаемый исход заявки. Подойдёт и любой другой ИНН с верной контрольной суммой.</p>
      <div className="demo-list">
        {list.map((e) => (
          <button type="button" key={e.inn} className="demo-item" onClick={() => onPick(e.inn)}>
            <span style={{ margin: 0 }}>
              <b>{e.name}</b>
              <span>ИНН {e.inn} · {e.note}</span>
            </span>
            <span className="go" aria-hidden>→</span>
          </button>
        ))}
      </div>
    </>
  )
}

export default function Login() {
  const { login } = useAuth()
  const nav = useNavigate()
  const [params] = useSearchParams()
  const inn = params.get('inn') ?? ''
  const [mode, setMode] = useState<'login' | 'register'>(params.get('mode') === 'register' ? 'register' : 'login')
  const [signupInn, setSignupInn] = useState('')
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
        <h1>{mode === 'login' ? 'Вход в кабинет' : 'Создать аккаунт'}</h1>
        <div className="login-tabs" role="tablist" aria-label="Вход или регистрация">
          <button type="button" role="tab" aria-selected={mode === 'login'} onClick={() => setMode('login')}>Войти</button>
          <button type="button" role="tab" aria-selected={mode === 'register'} onClick={() => setMode('register')}>Регистрация</button>
        </div>
        {mode === 'login' ? (
          <>
            <p>Вход для клиентов и сотрудников банка по логину и паролю.</p>
            <form className="login-form" onSubmit={onSubmit} noValidate>
              {error && <div className="login-error" role="alert">{error}</div>}
              <FloatField id="login" label="Логин" autoComplete="username" autoFocus value={name} onChange={setName} />
              <FloatField id="password" label="Пароль" type="password" autoComplete="current-password" value={password} onChange={setPassword} />
              <Button type="submit" size="lg" block busy={busy}>Войти</Button>
            </form>
          </>
        ) : (
          <>
            <p>Регистрация для предпринимателей. Подтверждение не требуется: аккаунт создаётся сразу.</p>
            <Register inn={signupInn} onInnChange={setSignupInn} />
          </>
        )}
        <p className="login-back"><Link to="/">← На главную</Link></p>
      </main>

      <aside className="login-demo" aria-labelledby="demo-h">
        {mode === 'register' ? <Examples onPick={setSignupInn} /> : <>
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
          <button type="button" className="demo-item" disabled={busy} onClick={() => void signIn('support1', 'demo1234')}>
            <span style={{ margin: 0 }}>
              <b>Мария Громова, специалист поддержки</b>
              <span>Чаты с клиентами: подключается, когда клиент просит сотрудника</span>
            </span>
            <span className="go" aria-hidden>→</span>
          </button>
        </div>
        </>}
      </aside>
    </div>
  )
}
