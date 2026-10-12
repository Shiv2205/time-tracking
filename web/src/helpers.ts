export function dateString(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}
export function today(): string {
  return dateString(new Date())
}
export function shiftDate(value: string, days: number): string {
  const date = new Date(`${value}T12:00:00`)
  date.setDate(date.getDate() + days)
  return dateString(date)
}
export function weekStart(value = today()): string {
  const date = new Date(`${value}T12:00:00`)
  return shiftDate(value, -((date.getDay() + 6) % 7))
}
export function shortDate(value: string): string {
  return new Date(`${value}T12:00:00`).toLocaleDateString('en', { month: 'short', day: 'numeric' })
}
export function weekday(value: string): string {
  return new Date(`${value}T12:00:00`).toLocaleDateString('en', { weekday: 'short' })
}
export function duration(minutes: number): string {
  return `${Math.floor(minutes / 60)}:${String(minutes % 60).padStart(2, '0')}`
}
export function hoursLabel(minutes: number): string {
  const hours = Math.floor(minutes / 60),
    remainder = minutes % 60
  return `${hours}h${remainder ? ` ${remainder}m` : ''}`
}
export function parseDuration(input: string): number {
  const value = input.trim()
  if (!/^\d+(?::[0-5]\d|(?:\.\d{1,4})?)$/.test(value))
    throw new Error('Use H:MM or decimal hours, for example 1:30 or 1.5.')
  let minutes: number
  if (value.includes(':')) {
    const [h, m] = value.split(':').map(Number)
    minutes = h! * 60 + m!
  } else {
    minutes = Number(value) * 60
    if (Math.abs(minutes - Math.round(minutes)) > 0.000001)
      throw new Error('Use a duration in whole minutes, such as 1:20.')
    minutes = Math.round(minutes)
  }
  if (minutes < 1 || minutes > 1440)
    throw new Error('Duration must be between 1 minute and 24 hours.')
  return minutes
}
export function editable(entry: { status: string }): boolean {
  return entry.status === 'draft' || entry.status === 'failed'
}
export function projectColor(id: string): string {
  let value = 0
  for (const letter of id) value = (value + letter.charCodeAt(0)) % 4
  return ['#528b77', '#b88a49', '#6e85af', '#9b76a0'][value]!
}
