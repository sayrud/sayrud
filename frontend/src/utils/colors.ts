// Palette of the option tags. The option stores the index of its color in TAG_COLORS.

import type { Theme } from './theme'

export interface TagColor {
  bg: string
  /** Dark text on the light backgrounds, white text on the dark ones. */
  text: string
}

type Shade = 'vivid' | 'lightest' | 'light' | 'medium' | 'dark'

const HUES = {
  red: { vivid: '#ef7f77', lightest: '#fbe3e2', light: '#f7c9c5', medium: '#e25b53', dark: '#ac3b32' },
  orange: { vivid: '#f38c3c', lightest: '#fae8d4', light: '#f7cb9b', medium: '#e07b35', dark: '#9a4e1f' },
  yellow: { vivid: '#f7c744', lightest: '#faf1d1', light: '#f8e28e', medium: '#d5a23a', dark: '#7f5f1b' },
  olive: { vivid: '#9aad34', lightest: '#edf3c1', light: '#d4e27a', medium: '#86962e', dark: '#5e6b21' },
  green: { vivid: '#5fbb5a', lightest: '#ddf5d8', light: '#aee3a6', medium: '#5aa852', dark: '#3a7331' },
  teal: { vivid: '#5ebfa8', lightest: '#d5f2ec', light: '#96e3d6', medium: '#4da995', dark: '#2e6f65' },
  blue: { vivid: '#56aee3', lightest: '#d9f0fb', light: '#a7daf8', medium: '#4595ca', dark: '#2a6592' },
  indigo: { vivid: '#82a2f8', lightest: '#e1e9ff', light: '#c7d6fd', medium: '#6f8ff4', dark: '#2556e0' },
  pink: { vivid: '#da7fb6', lightest: '#fae3f0', light: '#f2c6e3', medium: '#d160a2', dark: '#a33477' },
  purple: { vivid: '#b391f2', lightest: '#ede5fd', light: '#d8c9f8', medium: '#9a70e8', dark: '#7535df' },
  gray: { vivid: '#8f959e', lightest: '#eff0f1', light: '#dee0e3', medium: '#646a73', dark: '#3a3e45' },
} satisfies Record<string, Record<Shade, string>>

type Hue = keyof typeof HUES

// The same indexes in the dark theme. The shades are reversed, lightest is the darkest and dark is the brightest, to keep
// the same contrast levels on the dark backgrounds.
const DARK_HUES = {
  red: { vivid: '#a5433c', lightest: '#52231f', light: '#712b28', medium: '#c15048', dark: '#e78882' },
  orange: { vivid: '#975727', lightest: '#462c15', light: '#623c1b', medium: '#ac632c', dark: '#e58c3a' },
  yellow: { vivid: '#a67f2f', lightest: '#443512', light: '#5f481b', medium: '#cb9d37', dark: '#f3cd5f' },
  olive: { vivid: '#626f23', lightest: '#32380e', light: '#424c16', medium: '#6f7e26', dark: '#98ae36' },
  green: { vivid: '#427331', lightest: '#203a17', light: '#2e5022', medium: '#4c8538', dark: '#6eb854' },
  teal: { vivid: '#3b7169', lightest: '#203a36', light: '#2a4d47', medium: '#3f8378', dark: '#56b4a2' },
  blue: { vivid: '#326a8e', lightest: '#1b3545', light: '#234257', medium: '#3d7da7', dark: '#57afe0' },
  indigo: { vivid: '#365ec7', lightest: '#1d3062', light: '#24418f', medium: '#436fe3', dark: '#7fa3f8' },
  pink: { vivid: '#9f487b', lightest: '#52203e', light: '#6f3055', medium: '#b4528c', dark: '#de7db7' },
  purple: { vivid: '#754ccd', lightest: '#3b226f', light: '#4f2b9d', medium: '#8458e4', dark: '#b291f7' },
  gray: { vivid: '#757575', lightest: '#373737', light: '#434343', medium: '#a6a6a6', dark: '#e0e0e0' },
} satisfies Record<Hue, Record<Shade, string>>

/** Column order of the palette picker. */
const DISPLAY_HUES: Hue[] = ['red', 'orange', 'yellow', 'olive', 'green', 'teal', 'blue', 'indigo', 'pink', 'purple', 'gray']
/** Row order of the palette picker, the vivid row is separated from the shades. */
const DISPLAY_SHADES: Shade[] = ['vivid', 'lightest', 'light', 'medium', 'dark']

// Index order of the colors: the lightest shades first so the new options get light colors, and the first 10 hues keep
// the order of the former palette so the saved option colors stay the same.
const INDEX_HUES: Hue[] = ['indigo', 'green', 'orange', 'red', 'purple', 'teal', 'pink', 'yellow', 'blue', 'gray', 'olive']
const INDEX_SHADES: Shade[] = ['lightest', 'light', 'medium', 'dark', 'vivid']

function buildColors(hues: Record<Hue, Record<Shade, string>>, whiteTextShades: Shade[]): TagColor[] {
  return INDEX_SHADES.flatMap((shade) =>
    INDEX_HUES.map((hue) => ({ bg: hues[hue][shade], text: whiteTextShades.includes(shade) ? '#ffffff' : '#1f2329' })),
  )
}

/** The saturated medium and dark shades use white text, the others use dark text. */
export const TAG_COLORS: TagColor[] = buildColors(HUES, ['medium', 'dark'])
/** In the dark palette only the brightest dark shade uses dark text. */
const DARK_TAG_COLORS: TagColor[] = buildColors(DARK_HUES, ['vivid', 'lightest', 'light', 'medium'])

/** Rows of the palette picker, each cell is the index in TAG_COLORS. */
export const PALETTE_ROWS: number[][] = DISPLAY_SHADES.map((shade) =>
  DISPLAY_HUES.map((hue) => INDEX_SHADES.indexOf(shade) * INDEX_HUES.length + INDEX_HUES.indexOf(hue)),
)

export function tagColor(index: number, theme: Theme = 'light'): TagColor {
  const colors = theme === 'dark' ? DARK_TAG_COLORS : TAG_COLORS
  const len = colors.length
  return colors[((index % len) + len) % len]!
}
