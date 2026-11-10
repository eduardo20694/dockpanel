import assert from 'node:assert/strict'
import test from 'node:test'
import { suggestionClass, suggestionLabel } from '../src/lib/triageLabels.ts'

test('rótulos do relato', () => {
  assert.equal(suggestionLabel('rollback', '1.0.7'), 'rollback para 1.0.7')
  assert.equal(suggestionLabel('rollback'), 'rollback')
  assert.equal(suggestionLabel('restart'), 'restart')
  assert.equal(suggestionLabel('none'), 'nada a fazer')
  assert.equal(suggestionClass('rollback'), 'badge-critical')
  assert.equal(suggestionClass('none'), 'badge-ok')
})
