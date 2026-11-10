import assert from 'node:assert/strict'
import test from 'node:test'
import { resolveTheme, themeLabel } from '../src/lib/theme.ts'

test('tema salvo vence o sistema', () => {
  assert.equal(resolveTheme('light', 'dark'), 'light')
  assert.equal(resolveTheme('dark', 'light'), 'dark')
})

test('sem preferência, segue o sistema', () => {
  assert.equal(resolveTheme(null, 'light'), 'light')
  assert.equal(resolveTheme(undefined, 'dark'), 'dark')
  assert.equal(resolveTheme('', 'light'), 'light')
  assert.equal(resolveTheme('auto', 'dark'), 'dark')
})

test('rótulos Dia e Noite', () => {
  assert.equal(themeLabel('light'), 'Dia')
  assert.equal(themeLabel('dark'), 'Noite')
})
