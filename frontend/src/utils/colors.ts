// Palette of the option tags. The option stores the index of its color in TAG_COLORS.

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

/** Column order of the palette picker. */
const DISPLAY_HUES: Hue[] = ['red', 'orange', 'yellow', 'olive', 'green', 'teal', 'blue', 'indigo', 'pink', 'purple', 'gray']
/** Row order of the palette picker, the vivid row is separated from the shades. */
const DISPLAY_SHADES: Shade[] = ['vivid', 'lightest', 'light', 'medium', 'dark']

// Index order of the colors: the lightest shades first so the new options get light colors, and the first 10 hues keep
// the order of the former palette so the saved option colors stay the same.
const INDEX_HUES: Hue[] = ['indigo', 'green', 'orange', 'red', 'purple', 'teal', 'pink', 'yellow', 'blue', 'gray', 'olive']
const INDEX_SHADES: Shade[] = ['lightest', 'light', 'medium', 'dark', 'vivid']

/** The saturated medium and dark shades use white text, the others use dark text. */
const WHITE_TEXT_SHADES: Shade[] = ['medium', 'dark']

export const TAG_COLORS: TagColor[] = INDEX_SHADES.flatMap((shade) =>
  INDEX_HUES.map((hue) => ({ bg: HUES[hue][shade], text: WHITE_TEXT_SHADES.includes(shade) ? '#ffffff' : '#1f2329' })),
)

/** Rows of the palette picker, each cell is the index in TAG_COLORS. */
export const PALETTE_ROWS: number[][] = DISPLAY_SHADES.map((shade) =>
  DISPLAY_HUES.map((hue) => INDEX_SHADES.indexOf(shade) * INDEX_HUES.length + INDEX_HUES.indexOf(hue)),
)

export function tagColor(index: number): TagColor {
  const len = TAG_COLORS.length
  return TAG_COLORS[((index % len) + len) % len]!
}
