import {describe,it,expect} from 'vitest'
import {fnbNextRefresh,fnbPauseExplanation,fnbRecoveryInstruction,fnbScheduleLabel} from './fnbConnectionStatus'
const scheduled={schedule:{frequency:'daily',times:['05:00']},state:'ready',next_due:Date.UTC(2026,9,16,3)/1000}
describe('bank connection status and recovery',()=>{
 it('uses the actual connection state for paused, running and scheduled updates',()=>{
  expect(fnbNextRefresh({...scheduled,state:'refreshing'})).toBe('Update in progress')
  expect(fnbNextRefresh({...scheduled,state:'action_required'})).toBe('Paused')
  expect(fnbNextRefresh(scheduled)).toContain('05:00')
  expect(fnbNextRefresh(scheduled)).toContain('SAST')
  expect(fnbNextRefresh({state:'action_required'})).toBe('Off — update manually')
  expect(fnbNextRefresh({...scheduled,next_due:null})).toBe('Not yet scheduled')
 })
 it('explains interruptions, approval and infrastructure problems without guessing',()=>{
  expect(fnbPauseExplanation('INTERRUPTED')).toContain('interrupted')
  expect(fnbPauseExplanation('APPROVAL_REQUIRED')).toContain('approval in your FNB app')
  expect(fnbRecoveryInstruction('KEY_UNAVAILABLE')).toContain('restore the FNB encryption key')
  expect(fnbRecoveryInstruction('SESSION_CONFLICT')).toContain('Sign out')
  expect(fnbRecoveryInstruction('BROWSER_NOT_FOUND')).toContain('server administrator')
  for(const code of ['unknown untrusted error','__proto__','constructor'])expect(fnbPauseExplanation(code)).toBe('The bank connection needs attention before automatic updates can continue.')
  expect(fnbPauseExplanation(undefined)).toBe('The bank connection needs attention before automatic updates can continue.')
 })
 it('keeps saved clock times distinct from rolling intervals',()=>{
  expect(fnbScheduleLabel(scheduled)).toBe('Every day at 05:00 SAST')
  expect(fnbScheduleLabel({interval_hours:24})).toBe('Every 24 hours')
  expect(fnbScheduleLabel({daily_time:'05:00'})).toBe('Daily at 05:00 SAST')
 })
})
