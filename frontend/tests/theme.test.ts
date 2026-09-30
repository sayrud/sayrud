import assert from 'node:assert/strict'
import test from 'node:test'

import { PALETTE_ROWS, TAG_COLORS, tagColor } from '../src/utils/colors.ts'
import { parseThemeMode, resolveTheme } from '../src/utils/theme.ts'

test('tagColor keeps the saved index and switches the palette by theme', () => {
  assert.deepEqual(tagColor(0), TAG_COLORS[0])
  assert.deepEqual(tagColor(0, 'light'), TAG_COLORS[0])
  // Index 0 is indigo lightest: dark text on a light background in light, light text on a dark background in dark.
  assert.deepEqual(tagColor(0, 'dark'), { bg: '#1d3062', text: '#ffffff' })
  assert.deepEqual(tagColor(TAG_COLORS.length, 'dark'), tagColor(0, 'dark'))
  // The last palette row (the dark shade) is the brightest in dark and uses dark text.
  for (const i of PALETTE_ROWS[4]!) assert.equal(tagColor(i, 'dark').text, '#1f2329')
})

test('parseThemeMode falls back to light for missing or unknown values', () => {
  assert.equal(parseThemeMode('dark'), 'dark')
  assert.equal(parseThemeMode('system'), 'system')
  assert.equal(parseThemeMode('light'), 'light')
  assert.equal(parseThemeMode(null), 'light')
  assert.equal(parseThemeMode(undefined), 'light')
  assert.equal(parseThemeMode('auto'), 'light')
})

test('resolveTheme follows the system only in system mode', () => {
  assert.equal(resolveTheme('light', true), 'light')
  assert.equal(resolveTheme('dark', false), 'dark')
  assert.equal(resolveTheme('system', true), 'dark')
  assert.equal(resolveTheme('system', false), 'light')
})
