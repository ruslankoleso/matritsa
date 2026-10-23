import type { Status } from '../api/types'
import { statusView } from '../lib/status'

export function StatusPill({ status }: { status: Status }) {
  const v = statusView(status)
  return <span className={`pill${v.tone === 'default' ? '' : ` pill-${v.tone}`}`}>{v.label}</span>
}
