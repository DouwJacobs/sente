import {describe,it,expect} from 'vitest'
import {calendarSummary,monthDatesError,validClock} from './fnbSchedule'
describe('readable bank schedules',()=>{
 it('explains selected weekdays and sorted times in SAST',()=>{
  expect(calendarSummary({frequency:'weekly',weekdays:[3,1],times:['18:00','07:00']})).toBe('Every Monday, Wednesday at 07:00 and 18:00 SAST')
 })
 it('explains month end and missing-date choices',()=>{
  expect(calendarSummary({frequency:'monthly',month_days:[15,1],last_day:true,times:['12:00']})).toBe('Each month on 1, 15, the last day at 12:00 SAST')
  expect(monthDatesError('',true)).toBe('')
  for(const value of ['','0','32','1, 1','1,','Tuesday','2.5'])expect(monthDatesError(value,false)).not.toBe('')
  expect(monthDatesError('1, 15, 31',false)).toBe('')
 })
 it('keeps incomplete clocks out of the readable summary',()=>{
  expect(validClock('7:00')).toBe(false)
  expect(validClock('24:00')).toBe(false)
  expect(calendarSummary({frequency:'daily',times:['']})).toBe('Choose your refresh times.')
 })
})
