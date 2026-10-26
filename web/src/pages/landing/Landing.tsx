import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { AnimatePresence, MotionConfig, motion, useMotionValueEvent, useReducedMotion, useScroll } from 'framer-motion'
import { useQuery } from '@tanstack/react-query'
import { api } from '../../api/client'
import type { Product } from '../../api/types'
import { Route } from '../../design/Route'
import { LinkButton } from '../../design/Button'
import { NotchSlider } from '../../design/NotchSlider'
import { RollingNumber } from '../../design/RollingNumber'
import { reducedMotion } from '../../lib/motion'
import { annuity, formatMoney, formatMoneyShort, formatRate, formatTerm } from '../../lib/format'
import { useAuth } from '../../auth/AuthContext'
import { homeFor } from '../../auth/RequireRole'
import type { StopState } from '../../lib/status'
import './landing.css'

const rub = (n: number) => n * 100
const NB = ' '

/* ---------------------------------------------------------------- navigation */

function Nav() {
  const { session } = useAuth()
  return (
    <header className="land-nav">
      <Link to="/" className="wordmark" aria-label="Матрица — на главную">
        <span className="wordmark-dot" aria-hidden />
        Матрица
      </Link>
      <nav aria-label="Разделы">
        <a href="#how">Как это работает</a>
        <a href="#calc">Калькулятор</a>
        <a href="#faq">Вопросы</a>
        <LinkButton to={session ? homeFor(session.role) : '/login'} variant="dark" size="sm">
          {session ? 'В кабинет' : 'Войти'}
        </LinkButton>
      </nav>
    </header>
  )
}

/* ---------------------------------------------------------------- hero */

const HERO_STEPS: { states: StopState[]; note: string; busy: boolean }[] = [
  { states: ['done', 'current', 'todo', 'todo', 'todo'], note: 'Сверяем данные о компании с реестром…', busy: true },
  { states: ['done', 'done', 'current', 'todo', 'todo'], note: 'Оценка и расчёт условий…', busy: true },
  { states: ['done', 'done', 'done', 'attention', 'todo'], note: 'Предложение готово: ознакомьтесь с условиями', busy: false },
  { states: ['done', 'done', 'done', 'done', 'current'], note: 'Подписание и выдача денег', busy: false },
]

const FACTORS = [
  ['Возраст бизнеса', '15 из 15'],
  ['Стабильность выручки', '20 из 20'],
  ['Текущая долговая нагрузка', '20 из 20'],
  ['Кредитная история', '25 из 25'],
]

function HeroFragment() {
  const reduce = useReducedMotion()
  const [i, setI] = useState(reduce ? 2 : 0)
  useEffect(() => {
    if (reduce) return
    const t = window.setInterval(() => setI((v) => (v + 1) % HERO_STEPS.length), 2800)
    return () => window.clearInterval(t)
  }, [reduce])
  const step = HERO_STEPS[i]
  return (
    <motion.div
      className="frag"
      data-ui
      initial={{ opacity: 0, y: 24, scale: 1.12 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      transition={{ duration: 1.1, ease: [0.22, 1, 0.36, 1], delay: 0.35 }}
      aria-label="Пример заявки"
    >
      <div className="frag-head">
        <div>
          <div className="frag-title">Оборотные средства · заявка № 1042</div>
          <div className="frag-amount num">{formatMoney(rub(1_200_000))}</div>
          <div className="frag-meta num">12{NB}месяцев · платёж около {formatMoney(annuity(rub(1_200_000), 1650, 12))}</div>
        </div>
        <span className="pill pill-brand">В работе</span>
      </div>
      <Route states={step.states} />
      <div className="frag-note" role="status" aria-live="polite">
        {step.busy && <span className="spinner" aria-hidden />}
        <AnimatePresence mode="wait" initial={false}>
          <motion.span key={i} initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -6 }} transition={{ duration: 0.25 }}>
            {step.note}
          </motion.span>
        </AnimatePresence>
      </div>
      <div className="frag-rows">
        {FACTORS.map(([k, v]) => (
          <div className="frag-row" key={k}>
            <span>{k}</span>
            <b>{v}</b>
          </div>
        ))}
        <div className="frag-demo">Пример расчёта</div>
      </div>
    </motion.div>
  )
}

/** Decorative circles. They drift on their own and, once the pointer moves, lean towards it. */
const CIRCLES: { cls: string; depth: number }[] = [
  { cls: 'hc hc-a', depth: 0.1 },
  { cls: 'hc hc-b', depth: 0.18 },
  { cls: 'hc hc-c', depth: 0.28 },
  { cls: 'hc hc-d', depth: 0.14 },
  { cls: 'hc hc-e', depth: 0.34 },
  { cls: 'hc hc-f', depth: 0.12 },
  { cls: 'hc hc-g', depth: 0.22 },
]

function HeroCircles() {
  const root = useRef<HTMLDivElement>(null)
  const ring = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const el = root.current
    if (!el || reducedMotion() || !window.matchMedia?.('(pointer: fine)')?.matches) return
    const hero = el.parentElement as HTMLElement
    const nodes = Array.from(el.querySelectorAll<HTMLElement>('.hc'))
    const target = { x: 0, y: 0 }
    const cur = nodes.map(() => ({ x: 0, y: 0 }))
    const pointer = { x: -100, y: -100, rx: -100, ry: -100 }
    let raf = 0
    let inside = false
    const onMove = (e: PointerEvent) => {
      const r = hero.getBoundingClientRect()
      inside = e.clientY >= r.top && e.clientY <= r.bottom
      target.x = (e.clientX - (r.left + r.width / 2)) / r.width
      target.y = (e.clientY - (r.top + r.height / 2)) / r.height
      pointer.x = e.clientX - r.left
      pointer.y = e.clientY - r.top
      ring.current?.classList.toggle('big', !!(e.target as HTMLElement).closest('a,button'))
    }
    const loop = () => {
      nodes.forEach((n, i) => {
        const d = CIRCLES[i]?.depth ?? 0.1
        const tx = inside ? target.x * d * 520 : 0
        const ty = inside ? target.y * d * 360 : 0
        cur[i].x += (tx - cur[i].x) * 0.05
        cur[i].y += (ty - cur[i].y) * 0.05
        n.style.transform = `translate3d(${cur[i].x.toFixed(1)}px,${cur[i].y.toFixed(1)}px,0)`
      })
      pointer.rx += (pointer.x - pointer.rx) * 0.2
      pointer.ry += (pointer.y - pointer.ry) * 0.2
      if (ring.current) {
        ring.current.style.transform = `translate3d(${pointer.rx}px,${pointer.ry}px,0)`
        ring.current.style.opacity = inside ? '1' : '0'
      }
      raf = requestAnimationFrame(loop)
    }
    window.addEventListener('pointermove', onMove, { passive: true })
    raf = requestAnimationFrame(loop)
    return () => {
      window.removeEventListener('pointermove', onMove)
      cancelAnimationFrame(raf)
    }
  }, [])
  return (
    <div className="hero-circles" ref={root} aria-hidden>
      {CIRCLES.map((c, i) => (
        <div key={i} className={c.cls}>
          <i />
        </div>
      ))}
      <div className="hero-cursor" ref={ring} />
    </div>
  )
}

function Hero() {
  const { session } = useAuth()
  return (
    <section className="hero" aria-labelledby="h1">
      <HeroCircles />
      <div className="hero-copy">
        <h1 id="h1">
          {'Кредит для бизнеса шаг за шагом'.split(' ').map((w, i) => (
            <span key={i}>
              <span className="word" style={{ animationDelay: `${i * 90}ms` }}>{w}</span>{' '}
            </span>
          ))}
        </h1>
        <motion.p className="hero-lead" initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.6, delay: 0.12, ease: [0.22, 1, 0.36, 1] }}>
          Выберите продукт, подайте заявку и смотрите, как она движется по маршруту, — всё в одном кабинете, без звонков и писем.
        </motion.p>
        <motion.div className="hero-actions" initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.6, delay: 0.22, ease: [0.22, 1, 0.36, 1] }}>
          <LinkButton to={session ? homeFor(session.role) : '/login'} size="lg" magnetic>
            {session ? 'Открыть кабинет' : 'Войти в кабинет'}
          </LinkButton>
          <a className="btn btn-ghost btn-lg" href="#how">
            Как это работает
          </a>
        </motion.div>
      </div>
      <motion.div
        className="hero-stage"
        initial={{ clipPath: 'inset(0 0 100% 0)' }}
        animate={{ clipPath: 'inset(0 0 0% 0)' }}
        transition={{ duration: 1.1, ease: [0.22, 1, 0.36, 1] }}
      >
        <HeroFragment />
      </motion.div>
    </section>
  )
}

/* ---------------------------------------------------------------- people: who this is for */

const PEOPLE: { src: string; alt: string; name: string; need: string }[] = [
  { src: '/photos/baker.jpg', alt: 'Руки пекаря месят тесто на присыпанном мукой столе', name: 'Пекарня', need: 'Оборотные деньги на муку и сырьё перед сезоном' },
  { src: '/photos/barista.jpg', alt: 'Бариста трамбует кофе в холдере у кофемашины', name: 'Кофейня', need: 'Овердрафт на случай кассового разрыва' },
  { src: '/photos/workshop.jpg', alt: 'Мастер шлифует деревянную деталь в столярной мастерской', name: 'Мастерская', need: 'Кредит на оборудование с длинным сроком' },
]

function People() {
  return (
    <section className="people" aria-labelledby="people-h">
      <div className="people-copy">
        <h2 id="people-h">Для тех, кто делает дело</h2>
        <p>У каждого бизнеса свой путь к деньгам. Мы показываем каждый шаг и объясняем решения понятным языком.</p>
      </div>
      <div className="people-row">
        {PEOPLE.map((p, i) => (
          <motion.figure
            className="person"
            key={p.name}
            initial={{ opacity: 0, y: 40 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true, margin: '-10% 0px' }}
            transition={{ duration: 0.7, delay: i * 0.1, ease: [0.22, 1, 0.36, 1] }}
          >
            <img src={p.src} alt={p.alt} width={720} height={720} loading="lazy" />
            <figcaption><b>{p.name}</b>{p.need}</figcaption>
          </motion.figure>
        ))}
      </div>
    </section>
  )
}

/* ---------------------------------------------------------------- how it works */

function StripCard({ stage }: { stage: 0 | 1 | 2 | 3 }) {
  const tick = (s: 'done' | 'now' | 'todo') => <i className={`tick${s === 'todo' ? ' todo' : s === 'now' ? ' now' : ''}`} />
  return (
    <div className="strip-card" data-ui>
      <div className="frag-title">Оборотные средства</div>
      <div className="frag-amount num">{formatMoney(rub(1_200_000))}</div>
      {stage === 0 && (
        <>
          <span className="pill">Черновик</span>
          <div className="frag-rows">
            <div className="frag-row"><span>Цель</span><b>Закупка сырья</b></div>
            <div className="frag-row"><span>Срок</span><b>12{NB}мес.</b></div>
            <div className="frag-row"><span>Документы</span><b>0 из 2</b></div>
          </div>
        </>
      )}
      {stage === 1 && (
        <>
          <span className="pill pill-info">Сверка данных</span>
          <div className="frag-rows">
            <div className="frag-row"><span>{tick('done')}Реестр компаний</span><b>есть</b></div>
            <div className="frag-row"><span>{tick('now')}Кредитная история</span><b>…</b></div>
            <div className="frag-row"><span>{tick('done')}Документы</span><b>2 из 2</b></div>
          </div>
        </>
      )}
      {stage === 2 && (
        <>
          <span className="pill pill-brand">Оценка</span>
          <div className="frag-rows">
            <div className="frag-row"><span>{tick('done')}Возраст бизнеса</span><b>+15</b></div>
            <div className="frag-row"><span>{tick('done')}Стабильность выручки</span><b>+20</b></div>
            <div className="frag-row"><span>{tick('now')}Текущая нагрузка</span><b>…</b></div>
            <div className="frag-row"><span>{tick('todo')}Кредитная история</span><b>—</b></div>
          </div>
        </>
      )}
      {stage === 3 && (
        <>
          <span className="pill pill-ok">Есть предложение</span>
          <div className="frag-rows">
            <div className="frag-row"><span>Платёж в месяц</span><b>{formatMoneyShort(annuity(rub(1_200_000), 1650, 12))}</b></div>
            <div className="frag-row"><span>Ставка</span><b>{formatRate(1650)}</b></div>
            <div className="frag-row"><span>Срок</span><b>12{NB}мес.</b></div>
          </div>
        </>
      )}
    </div>
  )
}

const CLAUSES: [string, string][] = [
  ['Сначала', 'вы описываете, на что нужны средства.'],
  ['Потом', 'мы сверяем данные о вашей компании.'],
  ['Затем', 'оцениваем и объясняем каждый фактор.'],
  ['В итоге', 'вы видите условия и платёж.'],
]

const FIELD_TONES = ['#df291c', '#df291c', '#ffffff', '#fbcbc6']
const FIELD_BASES = ['rgba(27,26,25,.14)', 'rgba(27,26,25,.14)', 'rgba(255,255,255,.35)', 'rgba(255,255,255,.22)']

/** The route is drawn as the section scrolls past; each stop lights up once the line reaches it. */
function StripTrack({ target }: { target: React.RefObject<HTMLElement | null> }) {
  const { scrollYProgress } = useScroll({ target, offset: ['start 75%', 'end 60%'] })
  const [p, setP] = useState(reducedMotion() ? 1 : 0)
  useMotionValueEvent(scrollYProgress, 'change', (v) => setP(reducedMotion() ? 1 : v))
  return (
    <div className="strip-track" aria-hidden>
      {FIELD_TONES.map((tone, i) => {
        const seg = Math.min(1, Math.max(0, p * 4 - i))
        return (
          <div className="strip-seg" key={i} style={{ background: FIELD_BASES[i] }}>
            <div className="strip-seg-fill" style={{ background: tone, transform: `scaleX(${seg})` }} />
            <span className="strip-stop" data-on={seg > 0.5} style={{ borderColor: tone }} />
          </div>
        )
      })}
    </div>
  )
}

function HowItWorks() {
  const sec = useRef<HTMLElement>(null)
  return (
    <section id="how" aria-labelledby="how-h" ref={sec} style={{ position: 'relative' }}>
      <StripTrack target={sec} />
      <div className="strip">
        {CLAUSES.map(([first, rest], i) => (
          <motion.div
            className="strip-field"
            key={first}
            initial={{ opacity: 0, y: 40 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true, margin: '-12% 0px' }}
            transition={{ duration: 0.7, delay: i * 0.08, ease: [0.22, 1, 0.36, 1] }}
          >
            {i === 0 && <h2 className="strip-heading" id="how-h">Маршрут заявки</h2>}
            <div className="strip-card-wrap">
              <StripCard stage={i as 0 | 1 | 2 | 3} />
            </div>
            <p className="strip-clause">
              {first} <span>{rest}</span>
            </p>
          </motion.div>
        ))}
      </div>
    </section>
  )
}

/* ---------------------------------------------------------------- showcase: the calculator, blown up */

const FALLBACK_PRODUCTS: Product[] = [
  { id: 'working_capital', title: 'Кредит на оборотные средства', description: '', hint: '', minAmount: rub(300_000), maxAmount: rub(10_000_000), minTerm: 3, maxTerm: 24, baseRateBp: 1650, autoApproveLimit: 0, documents: [] },
  { id: 'equipment', title: 'Кредит на оборудование', description: '', hint: '', minAmount: rub(500_000), maxAmount: rub(20_000_000), minTerm: 12, maxTerm: 60, baseRateBp: 1500, autoApproveLimit: 0, documents: [] },
  { id: 'overdraft', title: 'Овердрафт', description: '', hint: '', minAmount: rub(100_000), maxAmount: rub(5_000_000), minTerm: 12, maxTerm: 12, baseRateBp: 1800, autoApproveLimit: 0, documents: [] },
  { id: 'express', title: 'Экспресс-кредит', description: '', hint: '', minAmount: rub(50_000), maxAmount: rub(1_000_000), minTerm: 3, maxTerm: 12, baseRateBp: 2100, autoApproveLimit: 0, documents: [] },
]

const SEGMENT: Record<string, { short: string; word: string; field: string }> = {
  working_capital: { short: 'Оборот', word: 'оборотные средства', field: 'var(--red-500)' },
  equipment: { short: 'Техника', word: 'оборудование', field: 'var(--deep)' },
  overdraft: { short: 'Овердрафт', word: 'овердрафт', field: 'var(--red-700)' },
  express: { short: 'Экспресс', word: 'экспресс', field: 'var(--red-900)' },
}

function Showcase() {
  const { data } = useQuery({ queryKey: ['products'], queryFn: api.products, staleTime: 5 * 60_000, retry: 1 })
  const products = data && data.length ? data : FALLBACK_PRODUCTS
  const [idx, setIdx] = useState(0)
  const p = products[Math.min(idx, products.length - 1)]
  const [amountRub, setAmountRub] = useState(1_200_000)
  const [term, setTerm] = useState(12)

  const minA = p.minAmount / 100
  const maxA = p.maxAmount / 100
  const a = Math.min(Math.max(amountRub, minA), maxA)
  const t = Math.min(Math.max(term, p.minTerm), p.maxTerm)
  const stepA = maxA - minA > 2_000_000 ? 50_000 : 10_000
  const fixedTerm = p.minTerm === p.maxTerm
  const payment = annuity(rub(a), p.baseRateBp, t)
  const overpay = payment * t - rub(a)
  const seg = SEGMENT[p.id] ?? SEGMENT.working_capital

  return (
    <section id="calc" className="show" style={{ ['--stage-c' as string]: seg.field }} aria-labelledby="calc-h">
      <div className="show-copy">
        <h2 id="calc-h">
          Платёж на{' '}
          <AnimatePresence mode="wait" initial={false}>
            <motion.span key={seg.word} className="swap" initial={{ opacity: 0, y: 14 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -14 }} transition={{ duration: 0.25 }}>
              {seg.word}
            </motion.span>
          </AnimatePresence>
        </h2>
        <p className="show-lead">Измените сумму и срок: платёж пересчитывается сразу. Расчёт ориентировочный, по базовой ставке; точные условия банк определит после проверки.</p>
      </div>
      <motion.div className="calc-anchor" initial={{ opacity: 0, x: 60 }} whileInView={{ opacity: 1, x: 0 }} viewport={{ once: true, margin: '-10% 0px' }} transition={{ duration: 0.8, ease: [0.22, 1, 0.36, 1] }}>
        <div className="calc" data-ui>
          <div className="calc-seg" role="group" aria-label="Кредитный продукт">
            {products.map((pr, i) => (
              <button key={pr.id} type="button" aria-pressed={i === idx} onClick={() => setIdx(i)}>
                {SEGMENT[pr.id]?.short ?? pr.title}
              </button>
            ))}
          </div>
          <div>
            <div className="calc-line"><span>Сумма</span><b>{formatMoney(rub(a))}</b></div>
            <NotchSlider label="Сумма" value={a} min={minA} max={maxA} step={stepA} format={(v) => formatMoneyShort(rub(v))} onChange={setAmountRub} />
          </div>
          {fixedTerm ? (
            <div className="calc-line">
              <span>Срок</span>
              <b>{formatTerm(p.minTerm)}</b>
            </div>
          ) : (
            <div>
              <div className="calc-line"><span>Срок</span><b>{formatTerm(t)}</b></div>
              <NotchSlider label="Срок" value={t} min={p.minTerm} max={p.maxTerm} step={1} format={(v) => formatTerm(v)} onChange={setTerm} />
            </div>
          )}
          <div className="calc-result" aria-live="polite">
            <small>Платёж в месяц</small>
            <strong><RollingNumber value={payment} /></strong>
          </div>
          <div className="calc-note">
            Ставка от {formatRate(p.baseRateBp)} годовых · переплата за весь срок около {formatMoneyShort(overpay)}
          </div>
        </div>
      </motion.div>
    </section>
  )
}

/* ---------------------------------------------------------------- features: sentences with switches */

const FEATURES: { sentence: string; more: string; on: boolean }[] = [
  { sentence: 'Статус заявки отображается в реальном времени', more: 'Он обновляется сам, без звонков и писем: всё происходит внутри кабинета.', on: true },
  { sentence: 'Решение сопровождается пояснением', more: 'Оценка складывается из 12 критериев, а ожидаемые потери считаются отдельно — у каждого шага есть пояснение.', on: true },
  { sentence: 'Документы загружаются один раз', more: 'Список зависит от продукта: для овердрафта нужна только выписка по счёту.', on: false },
  { sentence: 'Вместо отказа — другие условия', more: 'Если заявка не проходит как есть, сотрудник может предложить меньшую сумму или другой срок.', on: false },
  { sentence: 'Подсказки подбираются под ваш бизнес', more: 'Лента предложений меняется от отрасли, оборотов и этапа заявки.', on: false },
]

function Features() {
  const [on, setOn] = useState(() => FEATURES.map((f) => f.on))
  const toggle = (i: number) => setOn((v) => v.map((x, j) => (j === i ? !x : x)))
  return (
    <section aria-label="Что вы получаете">
      {FEATURES.map((f, i) => (
        <motion.div
          className="band"
          data-on={on[i]}
          key={f.sentence}
          initial={{ opacity: 0, x: -24 }}
          whileInView={{ opacity: 1, x: 0 }}
          viewport={{ once: true, margin: '-8% 0px' }}
          transition={{ duration: 0.6, delay: i * 0.05, ease: [0.22, 1, 0.36, 1] }}
        >
          <div>
            <p className="band-text" id={`f${i}`}>{f.sentence}</p>
            <div className="band-more" id={`f${i}-more`}><div><p>{f.more}</p></div></div>
          </div>
          {/* An expander, not a switch: a switch that is "off" reads as "this feature is off". */}
          <button type="button" aria-expanded={on[i]} aria-controls={`f${i}-more`} aria-labelledby={`f${i}`} className="band-toggle" onClick={() => toggle(i)} data-ui>
            <span className="plus" aria-hidden />
          </button>
        </motion.div>
      ))}
    </section>
  )
}

/* ---------------------------------------------------------------- faq: tabbed index cards */

const FAQ: { tab: string; q: string; a: string }[] = [
  { tab: 'Сроки', q: 'Сколько ждать решения?', a: 'Если данных достаточно и сумма небольшая, решение приходит автоматически, обычно за несколько минут. В остальных случаях заявку смотрит сотрудник банка: вы увидите это в статусе и получите уведомление в кабинете.' },
  { tab: 'Документы', q: 'Какие документы нужны?', a: 'Это зависит от продукта: для овердрафта достаточно выписки по расчётному счёту, для кредита на оборудование нужны ещё отчётность и счёт на покупку. Кабинет покажет точный список и объяснит, зачем нужен каждый документ.' },
  { tab: 'Отказ', q: 'Что, если банк откажет?', a: 'Вы увидите причины отказа. Иногда вместо отказа банк предлагает другие условия: меньшую сумму или другой срок. Такое предложение появится в кабинете.' },
  { tab: 'Данные', q: 'Откуда банк берёт сведения о компании?', a: 'Из государственного реестра и бюро кредитных историй. Вам достаточно указать ИНН: название, вид деятельности и остальное мы подтянем сами.' },
]

function Faq() {
  const [open, setOpen] = useState(0)
  return (
    <section id="faq" className="faq" aria-labelledby="faq-h">
      <h2 id="faq-h">Частые вопросы</h2>
      <div className="faq-stack">
        {FAQ.map((f, i) => (
          <motion.div
            className="faq-card"
            data-open={open === i}
            style={{ ['--i' as string]: i }}
            key={f.q}
            initial={{ opacity: 0, y: 30 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true, margin: '-6% 0px' }}
            transition={{ duration: 0.5, delay: i * 0.06, ease: [0.22, 1, 0.36, 1] }}
          >
            <span className="faq-tab" aria-hidden>{f.tab}</span>
            <div className="faq-body">
              <button type="button" className="faq-q" aria-expanded={open === i} aria-controls={`faq-a-${i}`} onClick={() => setOpen(open === i ? -1 : i)}>
                <span>{f.q}</span>
                <span className="plus" aria-hidden />
              </button>
              <div className="faq-a" id={`faq-a-${i}`} role="region" aria-label={f.q}>
                <div><p>{f.a}</p></div>
              </div>
            </div>
          </motion.div>
        ))}
      </div>
    </section>
  )
}

/* ---------------------------------------------------------------- cta: a sentence with a blank */

export function groupInn(raw: string): string {
  const d = raw.replace(/\D/g, '').slice(0, 12)
  return d.length > 10 ? d.replace(/(\d{4})(\d{4})(\d{0,4})/, '$1 $2 $3').trim() : d.replace(/(\d{4})(\d{0,6})/, '$1 $2').trim()
}

const innOk = (v: string) => /^\d{10}$|^\d{12}$/.test(v.replace(/\s/g, ''))

function Cta() {
  const nav = useNavigate()
  const [inn, setInn] = useState('')
  const [err, setErr] = useState('')
  const ok = innOk(inn)
  const check = () => {
    const n = inn.replace(/\s/g, '').length
    if (n && !ok) setErr('ИНН — это 10 цифр у организаций и 12 у предпринимателей. Проверьте, пожалуйста.')
  }
  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (!ok) {
      setErr('ИНН — это 10 цифр у организаций и 12 у предпринимателей. Проверьте, пожалуйста.')
      return
    }
    nav(`/login?inn=${inn.replace(/\s/g, '')}`)
  }
  return (
    <section className="cta" aria-labelledby="cta-h">
      <form onSubmit={submit} noValidate>
        <p className="cta-sentence" id="cta-h">
          Начнём с вашего ИНН{' '}
          <span className={`cta-field${ok ? ' ok' : ''}${err ? ' bad' : ''}`} key={err}>
            <input className="cta-input" inputMode="numeric" autoComplete="off" placeholder="ИНН" value={inn} aria-label="ИНН компании" aria-invalid={!!err} aria-describedby="cta-after"
              onChange={(e) => { setInn(groupInn(e.target.value)); setErr('') }} onBlur={check} />
            <svg className="cta-check" viewBox="0 0 24 24" aria-hidden><path d="M5 12.5l4.5 4.5L19 7.5" /></svg>
          </span>{' '}
          <button className="cta-go" type="submit">Продолжить</button>
        </p>
        <p className="cta-after" id="cta-after">{ok ? 'Формат верный. Откроем вход в кабинет и подскажем демо-доступ.' : 'Мы откроем вход в кабинет. ИНН нигде не сохраняется — он нужен только, чтобы подсказать демо-доступ.'}</p>
        {err && <p className="cta-error" role="alert">{err}</p>}
      </form>
    </section>
  )
}

/* ---------------------------------------------------------------- footer: the mark, built large */

function Footer() {
  const { session } = useAuth()
  const letters = useMemo(() => 'МАТРИЦА'.split(''), [])
  return (
    <footer className="land-footer">
      <div className="mark" aria-label="Матрица" role="img">
        {letters.map((l, i) => (
          <motion.span key={i} aria-hidden initial={{ scaleY: 0 }} whileInView={{ scaleY: 1 }} viewport={{ once: true }} transition={{ duration: 0.6, delay: i * 0.06, ease: [0.22, 1, 0.36, 1] }} style={{ transformOrigin: 'bottom' }}>
            {l}
          </motion.span>
        ))}
      </div>
      <div className="land-footer-links">
        <nav aria-label="Страницы"><a href="#how">Как это работает</a><a href="#calc">Калькулятор</a><a href="#faq">Вопросы</a></nav>
        <nav aria-label="Кабинеты"><Link to="/login">Вход для клиентов</Link><Link to="/login">Вход для сотрудников банка</Link></nav>
        <div className="contacts">
          <b>Команда МАТРИЦА РУСИ</b>
          <span>Хакатон «Альфа Хак», 2026.</span>
        </div>
      </div>
      <p className="credits">
        Фотографии: «Baker woman&apos;s hands kneading bread dough», «Barista prepares espresso at coffee shop», «Craftsman shaping wood in a workshop…» — Shixart1985,{' '}
        <a href="https://creativecommons.org/licenses/by/2.0" rel="noreferrer">CC BY 2.0</a>, через{' '}
        <a href="https://commons.wikimedia.org/" rel="noreferrer">Wikimedia Commons</a>. Это иллюстрации мира малого бизнеса, а не клиенты и не продукт банка.
      </p>
      <div className="land-footer-bottom">
        <span>© 2026 Матрица · стиль вдохновлён брендом кейса, официальные знаки не используются</span>
        <Link to={session ? homeFor(session.role) : '/login'} className="btn btn-sm">{session ? 'В кабинет' : 'Войти'}</Link>
      </div>
    </footer>
  )
}

export default function Landing() {
  return (
    <MotionConfig reducedMotion="user">
      <div className="land">
        <Nav />
        <main>
          <Hero />
          <People />
          <HowItWorks />
          <Showcase />
          <Features />
          <Faq />
          <Cta />
        </main>
        <Footer />
      </div>
    </MotionConfig>
  )
}
