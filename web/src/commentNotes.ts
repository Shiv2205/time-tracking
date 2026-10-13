export interface CommentSection {
  header: string
  points: string[]
  hours: string
}

export interface ParsedComments {
  raw: string
  sections: CommentSection[]
}

const sectionPattern =
  /^([^\r\n:]+):\r?\nBrief:\r?\n((?:- [^\r\n]+\r?\n)+)Duration: (\d+(?:\.\d+)?)$/

function parseSection(text: string): CommentSection | undefined {
  const match = sectionPattern.exec(text)
  if (!match) return
  return {
    header: match[1]!,
    points: match[2]!
      .trimEnd()
      .split(/\r?\n/)
      .map((line) => line.slice(2)),
    hours: match[3]!,
  }
}

// Only recognize complete formatted sections at the end. Earlier freeform notes remain editable
// and are kept as a literal prefix when another section is appended.
export function parseCommentNotes(notes: string): ParsedComments {
  let raw = notes
  const sections: CommentSection[] = []
  while (raw) {
    const separators = [...raw.matchAll(/\r?\n\r?\n/g)]
    const separator = separators[separators.length - 1]
    const section = parseSection(
      separator ? raw.slice(separator.index! + separator[0].length) : raw,
    )
    if (!section) break
    try {
      validateCommentSections([section])
    } catch {
      break
    }
    sections.unshift(section)
    raw = separator ? raw.slice(0, separator.index) : ''
  }
  return { raw, sections }
}

export function formatCommentSection(section: CommentSection): string {
  return `${section.header.trim()}:\nBrief:\n${section.points.map((point) => `- ${point.trim()}`).join('\n')}\nDuration: ${section.hours.trim()}`
}

export function formatCommentNotes(raw: string, sections: CommentSection[]): string {
  const formatted = sections.map(formatCommentSection).join('\n\n')
  return raw && formatted ? `${raw}\n\n${formatted}` : raw || formatted
}

export function validateCommentSections(sections: CommentSection[]): void {
  for (const [index, section] of sections.entries()) {
    const label = `Comment ${index + 1}`
    if (!section.header.trim() || section.header.includes(':'))
      throw new Error(`${label}: enter a header without a colon.`)
    if (!section.points.length || section.points.some((point) => !point.trim()))
      throw new Error(`${label}: fill in every brief point, or remove empty points.`)
    if (!/^\d+(?:\.\d+)?$/.test(section.hours.trim()))
      throw new Error(`${label}: enter duration as decimal hours, for example 1.5.`)
    const hours = Number(section.hours.trim())
    if (!Number.isFinite(hours) || hours <= 0 || hours > 24)
      throw new Error(`${label}: duration must be more than 0 and at most 24 hours.`)
  }
}
