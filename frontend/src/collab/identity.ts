import { randstr } from '@/utils/id'

export interface Identity {
  memberId: string
  name: string
  color: string
}

const STORAGE_KEY = 'sayrud.identity'

const ANIMALS = ['水獭', '企鹅', '海豚', '狐狸', '熊猫', '考拉', '刺猬', '松鼠', '鲸鱼', '猫头鹰', '羊驼', '浣熊']
const COLORS = ['#3370ff', '#f54a45', '#ff8800', '#14c0a7', '#7f3bf5', '#f5319d', '#00b2d6', '#8fac02']

function pick<T>(list: T[]): T {
  return list[Math.floor(Math.random() * list.length)]!
}

/** 浏览器维度的匿名协作者身份，仅用于在线头像与单元格光标。 */
export function getIdentity(): Identity {
  try {
    const saved = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? 'null') as Identity | null
    if (saved?.memberId && saved.name && saved.color) return saved
  } catch {
    // 忽略损坏的缓存，重新生成。
  }
  const identity: Identity = { memberId: 'mem' + randstr(12), name: '匿名' + pick(ANIMALS), color: pick(COLORS) }
  localStorage.setItem(STORAGE_KEY, JSON.stringify(identity))
  return identity
}
