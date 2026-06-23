import dayjs from 'dayjs'

export function formatDate(date?: string | Date | null): string {
  if (!date) return '-'
  return dayjs(date).format('YYYY-MM-DD HH:mm:ss')
}

export function formatDateShort(date?: string | Date | null): string {
  if (!date) return '-'
  return dayjs(date).format('YYYY-MM-DD HH:mm')
}

export function parseJSONArray(s?: string): string[] {
  if (!s) return []
  try {
    const arr = JSON.parse(s)
    if (Array.isArray(arr)) return arr.map(String)
    return []
  } catch {
    return []
  }
}

export function parseJSONObject<T = Record<string, unknown>>(s?: string): T {
  if (!s) return {} as T
  try {
    return JSON.parse(s)
  } catch {
    return {} as T
  }
}

export function shortId(id?: string, len = 8): string {
  if (!id) return '-'
  return id.length > len ? id.slice(0, len) : id
}

export function truncate(s: string | undefined | null, n: number): string {
  if (!s) return ''
  return s.length > n ? s.slice(0, n) + '…' : s
}

export function copyToClipboard(text: string): Promise<void> {
  if (navigator.clipboard) {
    return navigator.clipboard.writeText(text)
  }
  return new Promise((resolve, reject) => {
    try {
      const ta = document.createElement('textarea')
      ta.value = text
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
      resolve()
    } catch (e) {
      reject(e)
    }
  })
}
