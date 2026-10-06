import {describe,it,expect} from 'vitest'
import {proposeRulePattern} from './ruleProposal'
describe('reviewer-confirmed rule proposal',()=>{
 it('keeps merchant text and removes changing numeric references',()=>{
  expect(proposeRulePattern('Synthetic retailer 123456 2026/10/04')).toBe('Synthetic retailer')
  expect(proposeRulePattern('Market 123456 purchase reference')).toBe('Market')
  expect(proposeRulePattern('123456 Market')).toBe('Market')
  expect(proposeRulePattern('24 Hour Market')).toBe('24 Hour Market')
  expect(proposeRulePattern('123456')).toBe('123456')
  expect(proposeRulePattern('A'.repeat(250))).toHaveLength(200)
  expect(new TextEncoder().encode(proposeRulePattern('🛒'.repeat(100))).length).toBe(200)
 })
})
