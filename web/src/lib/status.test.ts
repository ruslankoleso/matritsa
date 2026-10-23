import { describe, expect, it } from 'vitest'
import { isFinal, routeStates, statusView } from './status'

describe('routeStates', () => {
  it('starts at the first stop for a draft', () => {
    expect(routeStates('DRAFT')).toEqual(['current', 'todo', 'todo', 'todo', 'todo'])
  })
  it('puts system checks on the second stop', () => {
    expect(routeStates('ENRICHMENT')).toEqual(['done', 'current', 'todo', 'todo', 'todo'])
  })
  it('asks for action on the right stop depending on who asked', () => {
    expect(routeStates('NEEDS_INFO', 'system')[1]).toBe('attention')
    expect(routeStates('NEEDS_INFO', 'staff')[2]).toBe('attention')
  })
  it('marks an offer as needing the client on the decision stop', () => {
    expect(routeStates('OFFER_MADE')).toEqual(['done', 'done', 'done', 'attention', 'todo'])
  })
  it('stops the route on rejection and finishes it on disbursement', () => {
    expect(routeStates('REJECTED')[3]).toBe('stopped')
    expect(routeStates('DISBURSED')).toEqual(['done', 'done', 'done', 'done', 'done'])
  })
  it('knows final statuses and has a label for each', () => {
    expect(isFinal('REJECTED')).toBe(true)
    expect(isFinal('SCORING')).toBe(false)
    expect(statusView('ANALYST_REVIEW').label).toBe('У сотрудника банка')
  })
})
