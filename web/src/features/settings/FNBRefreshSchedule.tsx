import {useRef,useState} from 'react'
import {api} from '../../api'
import {Button,Field,Form} from '../../ui'
import {type Row} from '../../shared/types'
import {calendarSummary,monthDates,monthDatesError,validClock,weekDays,type CalendarSchedule} from './fnbSchedule'

type Props = {
 connection: Row
 busy: boolean
 run: (action: () => Promise<unknown>, message?: string) => Promise<boolean>
 reload: () => Promise<unknown>
}

export function FNBRefreshSchedule({connection,busy,run,reload}:Props) {
 const saved = connection.schedule as CalendarSchedule|null
 const [frequency,setFrequency] = useState(saved?.frequency||(connection.daily_time?'daily':([0,6,12,24,168].includes(Number(connection.interval_hours||0))?String(connection.interval_hours||0):'interval')))
 const [intervalHours,setIntervalHours] = useState(String(connection.interval_hours||6))
 const nextTimeId=useRef(12)
 const [times,setTimes] = useState((saved?.times||[connection.daily_time||'12:00']).map((value,id)=>({id,value})))
 const [days,setDays] = useState(saved?.weekdays||[1,2,3,4,5])
 const [dates,setDates] = useState((saved?.month_days||[1]).join(', '))
 const [lastDay,setLastDay] = useState(saved?.last_day||false)
 const locked = busy||connection.state==='refreshing'
 const calendar = ['daily','weekly','monthly'].includes(frequency)
 const draft:CalendarSchedule = {
  frequency:frequency as CalendarSchedule['frequency'], times:times.map(time=>time.value),
  ...(frequency==='weekly'?{weekdays:days}:{}),
  ...(frequency==='monthly'?{month_days:monthDates(dates),last_day:lastDay}:{}),
 }
 return <Form onSubmit={async()=>{
  if(await run(()=>api('/fnb/schedule','PUT',{
   interval_hours:calendar?0:Number(frequency==='interval'?intervalHours:frequency), daily_time:'',
   schedule:calendar?draft:null, version:connection.version,
  }),'Refresh schedule saved')) await reload()
 }}>
  <Field label="Refresh schedule" hint="All clock times use South African time (Africa/Johannesburg, UTC+02:00).">
   <select value={frequency} disabled={locked} onChange={event=>setFrequency(event.target.value)}>
    <option value="0">Off — refresh manually</option>
    <option value="6">Every 6 hours</option>
    <option value="12">Every 12 hours</option>
    <option value="24">Every 24 hours</option>
    <option value="168">Every 7 days</option>
    <option value="interval">Every … hours</option>
    <option value="daily">Every day at chosen times</option>
    <option value="weekly">On selected days of the week</option>
    <option value="monthly">On selected dates each month</option>
   </select>
  </Field>
  {frequency==='interval'&&<Field label="Hours between refreshes" hint="Choose 1 to 168 hours. The interval starts when you save or finish an update."><input type="number" min="1" max="168" step="1" required value={intervalHours} disabled={locked} onChange={event=>setIntervalHours(event.target.value)}/></Field>}
  {frequency==='weekly'&&<fieldset className="refresh-weekdays">
   <legend>Repeat on</legend>
   <div className="toolbar-actions">
    <Button disabled={locked} onClick={()=>setDays([1,2,3,4,5])}>Weekdays</Button>
    <Button disabled={locked} onClick={()=>setDays([6,0])}>Weekends</Button>
    <Button disabled={locked} onClick={()=>setDays([0,1,2,3,4,5,6])}>Every day</Button>
   </div>
   <div className="refresh-day-grid">{weekDays.map((day,index)=><Field key={day.value} label={day.label} validate={index===0?()=>days.length?'':'Choose at least one day.':undefined}>
    <input type="checkbox" checked={days.includes(day.value)} disabled={locked} onChange={event=>setDays(event.target.checked?[...days,day.value]:days.filter(value=>value!==day.value))}/>
   </Field>)}</div>
  </fieldset>}
  {frequency==='monthly'&&<>
   <Field label="Dates of the month" hint="Separate dates with commas, for example 1, 15, 25. Dates that do not exist in a month are skipped." validate={value=>monthDatesError(value,lastDay)}>
    <input type="text" value={dates} disabled={locked} onChange={event=>setDates(event.target.value)} placeholder="1, 15, 25"/>
   </Field>
   <label className="check"><input type="checkbox" checked={lastDay} disabled={locked} onChange={event=>setLastDay(event.target.checked)}/>Run on the last day of the month</label>
  </>}
  {calendar&&<>
   <div className="refresh-times">{times.map((time,index)=><div className="refresh-time-row" key={time.id}>
    <Field label={`Refresh time ${index+1}`} validate={value=>!validClock(value)?'Enter a time in HH:MM format.':times.filter(other=>other.value===value).length>1?'Choose a different time.':''}>
     <input type="time" required step="60" value={time.value} disabled={locked} onChange={event=>setTimes(times.map(other=>other.id===time.id?{...other,value:event.target.value}:other))}/>
    </Field>
    {times.length>1&&<Button variant="quiet" aria-label={`Remove refresh time ${index+1}`} disabled={locked} onClick={()=>setTimes(times.filter(other=>other.id!==time.id))}>Remove</Button>}
   </div>)}</div>
   {times.length<12&&<Button disabled={locked} onClick={()=>setTimes([...times,{id:nextTimeId.current++,value:''}])}>Add another time</Button>}
   <p className="footnote" role="status">{frequency==='monthly'&&monthDatesError(dates,lastDay)?'Choose valid monthly dates.':calendarSummary(draft)}</p>
  </>}
  <p className="footnote">Missed schedules run once when Sente starts again. Paused connections need a successful manual refresh.</p>
  <Button type="submit" loading={busy} disabled={locked}>Save refresh schedule</Button>
 </Form>
}
