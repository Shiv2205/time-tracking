import { describe, expect, it } from 'vitest'
import { duration, editable, parseDuration, shiftDate, weekStart } from './helpers'

describe('whole minute arithmetic', () => {
  it('accepts clock and decimal durations without rounding', () => {
    expect(parseDuration('1:20')).toBe(80)
    expect(parseDuration('1.5')).toBe(90)
    expect(duration(95)).toBe('1:35')
  })
  it('rejects invalid, fractional-minute, and excessive input', () => {
    for (const value of ['0', '-1', '1:60', '1.3333', 'NaN', '24:01', '1e2'])
      expect(() => parseDuration(value)).toThrow()
  })
})
describe('calendar weeks', () => {
  it('starts on Monday and crosses month/year boundaries', () => {
    expect(weekStart('2026-10-11')).toBe('2026-10-05')
    expect(shiftDate('2026-12-28', 7)).toBe('2027-01-04')
    expect(shiftDate('2026-03-02', 6)).toBe('2026-03-08')
  })
  it('counts drafts and failed entries, excluding uncertain and submitted entries', () => {
    expect(editable({ status: 'failed' })).toBe(true)
    expect(editable({ status: 'uncertain' })).toBe(false)
    expect(editable({ status: 'submitted' })).toBe(false)
  })
})
