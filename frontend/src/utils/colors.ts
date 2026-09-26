// 选项标签调色板，贴近飞书多维表格的浅底深字配色。
export interface TagColor {
  bg: string
  text: string
  /** 看板列头等需要实色的场景。 */
  solid: string
}

export const TAG_COLORS: TagColor[] = [
  { bg: '#e1eaff', text: '#1c3fa8', solid: '#3370ff' },
  { bg: '#d9f5d6', text: '#237b19', solid: '#34c724' },
  { bg: '#fee7cd', text: '#a44904', solid: '#ff8800' },
  { bg: '#fde2e2', text: '#ac2020', solid: '#f54a45' },
  { bg: '#ece2fe', text: '#5a26b3', solid: '#7f3bf5' },
  { bg: '#d5f6f2', text: '#0d7a6e', solid: '#14c0a7' },
  { bg: '#fdddef', text: '#a51e6f', solid: '#f14bab' },
  { bg: '#faf1d1', text: '#8f6a00', solid: '#dca600' },
  { bg: '#d9f3fd', text: '#0b6a8e', solid: '#1ab0e5' },
  { bg: '#eff0f1', text: '#373c43', solid: '#8f959e' },
]

export function tagColor(index: number): TagColor {
  const len = TAG_COLORS.length
  return TAG_COLORS[((index % len) + len) % len]!
}
