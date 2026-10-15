export type CalendarSchedule = {
 frequency: 'daily' | 'weekly' | 'monthly'
 times: string[]
 weekdays?: number[]
 month_days?: number[]
 last_day?: boolean
}
export const weekDays = [
 {value:1,label:'Monday'}, {value:2,label:'Tuesday'}, {value:3,label:'Wednesday'},
 {value:4,label:'Thursday'}, {value:5,label:'Friday'}, {value:6,label:'Saturday'}, {value:0,label:'Sunday'},
]
export function validClock(value:string) { return /^([01]\d|2[0-3]):[0-5]\d$/.test(value) }
export function monthDates(value:string):number[] {
 return value.trim()?value.split(',').map(part=>Number(part.trim())):[]
}
export function monthDatesError(value:string,lastDay:boolean) {
 if(!value.trim()) return lastDay?'':'Choose at least one date or the last day of the month.'
 const days=monthDates(value)
 return days.some(day=>!Number.isInteger(day)||day<1||day>31)||new Set(days).size!==days.length
  ?'Enter distinct dates from 1 to 31, separated by commas.':''
}
export function calendarSummary(schedule:CalendarSchedule):string {
 const times=[...schedule.times].sort().join(' and ')
 if(!times||schedule.times.some(time=>!validClock(time)))return 'Choose your refresh times.'
 if(schedule.frequency==='daily')return `Every day at ${times} SAST`
 if(schedule.frequency==='weekly') {
  const days=weekDays.filter(day=>schedule.weekdays?.includes(day.value)).map(day=>day.label)
  return days.length?`Every ${days.join(', ')} at ${times} SAST`:'Choose at least one day.'
 }
 const dates=[...(schedule.month_days||[])].sort((a,b)=>a-b).map(String)
 if(schedule.last_day)dates.push('the last day')
 return dates.length?`Each month on ${dates.join(', ')} at ${times} SAST`:'Choose your monthly dates.'
}
