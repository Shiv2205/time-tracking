// Zoho can return notes as HTML. Convert them to readable text without inserting
// provider markup into the page. Plain-text notes retain their original spacing.
export function readableRemoteNotes(notes: string): string {
  if (!/<\/?[a-z][^>]*>|&(?:#\d+|#x[\da-f]+|[a-z]+);/i.test(notes)) return notes

  const template = document.createElement('template')
  template.innerHTML = notes
  let result = ''
  const lineBreak = () => {
    if (result && !result.endsWith('\n')) result += '\n'
  }
  const visit = (node: Node) => {
    if (node.nodeType === Node.TEXT_NODE) {
      result += node.textContent || ''
      return
    }
    if (node.nodeType !== Node.ELEMENT_NODE) return
    const tag = (node as Element).tagName.toLowerCase()
    if (['script', 'style', 'template', 'svg', 'iframe'].includes(tag)) return
    if (tag === 'br') {
      lineBreak()
      return
    }
    const block =
      ['p', 'div', 'ul', 'ol', 'li', 'blockquote', 'pre'].includes(tag) || /^h[1-6]$/.test(tag)
    if (block) lineBreak()
    if (tag === 'li') result += '- '
    node.childNodes.forEach(visit)
    if (block) lineBreak()
  }
  template.content.childNodes.forEach(visit)
  return result
    .replace(/[ \t]+\n/g, '\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}
