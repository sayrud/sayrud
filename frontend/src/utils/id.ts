const CHARS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'

export function randstr(length: number): string {
  let out = ''
  const bytes = crypto.getRandomValues(new Uint8Array(length))
  for (let i = 0; i < length; i++) out += CHARS[bytes[i]! % CHARS.length]
  return out
}

// 前缀与长度同后端 BeforeCreate 钩子。
export const newTableUID = () => 'tbl' + randstr(13)
export const newFieldUID = () => 'fld' + randstr(7)
export const newRecordUID = () => 'rec' + randstr(11)
export const newOptionUID = () => 'opt' + randstr(6)
export const newViewUID = () => 'viw' + randstr(10)
export const newProjectUID = () => 'prj' + randstr(10)
