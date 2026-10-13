import { describe, expect, it } from 'vitest'
import { formatCommentNotes, parseCommentNotes, validateCommentSections } from './commentNotes'

describe('formatted work comments', () => {
  const first = {
    header: 'API work',
    points: ['Added the endpoint', 'Reviewed error handling'],
    hours: '1.5',
  }
  const second = {
    header: 'Follow-up',
    points: ['Tested the fix'],
    hours: '0.5',
  }

  it('formats and reopens multiple editable sections', () => {
    const notes = formatCommentNotes('', [first, second])
    expect(notes).toBe(
      'API work:\nBrief:\n- Added the endpoint\n- Reviewed error handling\nDuration: 1.5\n\nFollow-up:\nBrief:\n- Tested the fix\nDuration: 0.5',
    )
    expect(parseCommentNotes(notes)).toEqual({ raw: '', sections: [first, second] })
  })

  it('preserves a freeform prefix and recognizes appended sections', () => {
    const raw = 'Earlier notes:\r\nKeep this exactly.\r\n\r\nSecond paragraph.'
    const notes = formatCommentNotes(raw, [first])
    expect(parseCommentNotes(notes)).toEqual({ raw, sections: [first] })
    expect(parseCommentNotes('A heading:\nBrief:\nSomething else')).toEqual({
      raw: 'A heading:\nBrief:\nSomething else',
      sections: [],
    })
  })

  it('requires complete sections with positive decimal-hour durations', () => {
    expect(() => validateCommentSections([{ ...first, points: [''] }])).toThrow(/brief point/)
    expect(() => validateCommentSections([{ ...first, hours: '0' }])).toThrow(/duration/)
    expect(() => validateCommentSections([{ ...first, hours: '1:30' }])).toThrow(/decimal hours/)
  })
})
