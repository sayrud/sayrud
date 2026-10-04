import assert from 'node:assert/strict'
import test from 'node:test'

import { APPEARANCE_COLORS, APPEARANCE_ICONS, appearanceBackground, appearanceIcon, appearanceSelection } from '../src/utils/appearance.ts'

test('appearance choices resolve saved values and preserve defaults for unknown values', () => {
  assert.equal(appearanceIcon(), APPEARANCE_ICONS.table)
  assert.equal(appearanceIcon(undefined, 'project'), APPEARANCE_ICONS.blocks)
  assert.equal(appearanceIcon('toString'), APPEARANCE_ICONS.table)

  assert.equal(appearanceBackground(), undefined)
  assert.equal(appearanceBackground('toString'), undefined)

  for (const [key, component] of Object.entries(APPEARANCE_ICONS)) assert.equal(appearanceIcon(key), component)

  for (const [key, background] of Object.entries(APPEARANCE_COLORS)) assert.equal(appearanceBackground(key), background)
})

test('appearance selection reflects saved settings without selecting defaults after removal', () => {
  assert.deepEqual(appearanceSelection('calendar', 'purple'), { icon: 'calendar', color: 'purple' })
  assert.deepEqual(appearanceSelection('', ''), { icon: '', color: '' })
  assert.deepEqual(appearanceSelection(), { icon: '', color: '' })

  assert.deepEqual(appearanceSelection('calendar'), { icon: 'calendar', color: '' })
  assert.deepEqual(appearanceSelection(undefined, 'teal'), { icon: '', color: 'teal' })
  assert.deepEqual(appearanceSelection('unknown', 'unknown'), { icon: '', color: '' })
  assert.deepEqual(appearanceSelection('toString', 'constructor'), { icon: '', color: '' })
})
