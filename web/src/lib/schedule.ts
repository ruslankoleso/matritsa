export interface ScheduleRow {
  n: number
  payment: number // kopecks
  interest: number
  principal: number
  balance: number // left to repay after the payment
}

/**
 * Annuity repayment schedule in kopecks for an offer: the bank's monthly payment every month, and the
 * last payment closes what is left, so the rows add up exactly to the amount plus interest.
 */
export function paymentSchedule(amount: number, rateBp: number, months: number, payment: number): ScheduleRow[] {
  const r = rateBp / 10000 / 12
  const rows: ScheduleRow[] = []
  let balance = amount
  for (let n = 1; n <= months && balance > 0; n++) {
    const interest = Math.round(balance * r)
    const pay = n === months ? balance + interest : Math.min(payment, balance + interest)
    const principal = pay - interest
    balance -= principal
    rows.push({ n, payment: pay, interest, principal, balance })
  }
  return rows
}

/** What the client pays over the whole term and how much of it is interest. */
export function scheduleTotals(rows: ScheduleRow[]): { total: number; overpayment: number } {
  const total = rows.reduce((s, r) => s + r.payment, 0)
  const overpayment = rows.reduce((s, r) => s + r.interest, 0)
  return { total, overpayment }
}
