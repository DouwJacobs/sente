import {useEffect,useState} from 'react'
import {api} from './api'
import {Button,Field,Form,Loading} from './ui'
import {FNBRefreshSchedule} from './features/settings/FNBRefreshSchedule'
import {FNBConnectionOptions} from './features/settings/FNBConnectionOptions'
import {fnbNextRefresh,fnbPauseExplanation,fnbRecoveryInstruction,fnbScheduleLabel,hasFNBSchedule} from './features/settings/fnbConnectionStatus'
import {useTask} from './shared/useTask'
import {type PageProps,type Row} from './shared/types'

export function FNBConnection({data,revision,refresh,notify,onAccounts,onTransactions,active=true}:PageProps&{onAccounts:()=>void;onTransactions:()=>void;active?:boolean}) {
 const [status,setStatus]=useState<Row|null>(null)
 const [username,setUsername]=useState(''),[password,setPassword]=useState('')
 const [debug,setDebug]=useState(false),[editing,setEditing]=useState(false)
 const [loading,setLoading]=useState(true),[loadError,setLoadError]=useState(''),[retry,setRetry]=useState(0)
 const {busy,run}=useTask(notify)
 useEffect(()=>{if(!active){setUsername('');setPassword('');setEditing(false)}},[active])
 const load=()=>run(async()=>{
  const next=await api('/fnb?summary=1')
  setStatus(next);setDebug(!!next.connection?.debug_browser)
 })
 useEffect(()=>{
  let alive=true
  setLoading(true);setLoadError('')
  api('/fnb?summary=1').then(next=>{
   if(alive){setStatus(next);setDebug(!!next.connection?.debug_browser)}
  }).catch(error=>{if(alive){setLoadError(error.message);notify(error.message,true)}})
   .finally(()=>alive&&setLoading(false))
  return()=>{alive=false}
 },[revision,retry])
 useEffect(()=>{
  if(!status?.connection)return
  let alive=true
  const timer=window.setInterval(()=>{
   api('/fnb?summary=1').then(next=>{
    if(!alive)return
    if(next.connection?.last_success!==status.connection?.last_success)refresh()
    else setStatus(next)
   }).catch(()=>{})
  },15000)
  return()=>{alive=false;clearInterval(timer)}
 },[status?.connection?.last_success,!!status?.connection,revision])
 if(!data.user.admin)return null
 const connection=status?.connection
 const updating=connection?.state==='refreshing'
 const paused=connection?.state==='action_required'
 const locked=busy||updating
 const credentialForm=<Form onSubmit={async()=>{
  if(await run(()=>api('/fnb','PUT',{username,password,version:connection?.version||0}),'FNB login details saved. Automatic updates are off.')){
   setUsername('');setPassword('');setEditing(false);await load();refresh()
  }
 }}>
  <Field label="FNB username" validate={value=>!value.trim()?'Enter your FNB username':new TextEncoder().encode(value).length>200?'FNB username is too long':''}>
   <input autoComplete="off" required maxLength={200} disabled={locked} value={username} onChange={event=>setUsername(event.target.value)}/>
  </Field>
  <Field label="FNB password" validate={value=>new TextEncoder().encode(value).length>512?'FNB password is too long':''}>
   <input type="password" autoComplete="off" required maxLength={512} disabled={locked} value={password} onChange={event=>setPassword(event.target.value)}/>
  </Field>
  <p className="footnote">Your login details are encrypted on this computer. Software with full access to the computer can still read them. FNB may ask you to approve the sign-in.</p>
  <div className="editor-actions">
   <Button type="submit" variant="primary" loading={busy} disabled={locked}>{connection?'Update login details':'Connect FNB'}</Button>
   {connection&&<Button disabled={locked} onClick={()=>{setEditing(false);setUsername('');setPassword('')}}>Cancel</Button>}
  </div>
 </Form>
 return <section className="panel banking-settings">
  <div className="section-head">
   <h2>FNB connection</h2>
   {connection&&!paused&&<span className="banking-connection-state">{updating?'Updating':'Connected'}</span>}
  </div>
  {loadError&&status&&<p className="footnote">Connection status could not refresh. <Button onClick={()=>setRetry(value=>value+1)}>Retry</Button></p>}
  {!status
   ?loading?<Loading>Loading connection</Loading>:<p>Unable to load connection status. <Button onClick={()=>setRetry(value=>value+1)}>Retry</Button></p>
   :!connection?<>
    <p className="muted">Connect FNB to update balances and import transactions.</p>
    {credentialForm}
   </>:<>
    {paused&&<section className="banking-pause" aria-label="Connection needs attention">
     <h3>{hasFNBSchedule(connection)?'Automatic updates paused':'Bank connection needs attention'}</h3>
     <p>{fnbPauseExplanation(connection.last_error)}</p>
     <p>{fnbRecoveryInstruction(connection.last_error)} {hasFNBSchedule(connection)&&'Your schedule is saved and resumes after a successful refresh.'}</p>
    </section>}
    <dl className="banking-overview">
     <div><dt>Last successful refresh</dt><dd>{connection.last_success?new Date(connection.last_success).toLocaleString():'Not yet'}</dd></div>
     <div><dt>Next scheduled refresh</dt><dd>{fnbNextRefresh(connection)}</dd></div>
    </dl>
    <div className="banking-refresh">
     <Button variant="primary" loading={locked} disabled={locked} onClick={async()=>{
      await run(()=>api('/fnb/refresh','POST'),'FNB accounts refreshed');await load();refresh()
     }}>Refresh now</Button>
     {!paused&&<span className="muted">Updates accounts and balances; FNB approval may be needed.</span>}
    </div>
    <details className="banking-group">
     <summary><strong>Automatic refresh</strong><span>{fnbScheduleLabel(connection)}</span></summary>
     <div className="banking-group-body">
      <p className="footnote">Update balances and import posted transactions while Sente is running.</p>
      <FNBRefreshSchedule key={`${connection.interval_hours}-${connection.daily_time||''}-${JSON.stringify(connection.schedule||null)}`} connection={connection} busy={busy} run={run} reload={load}/>
     </div>
    </details>
    <FNBConnectionOptions
     connection={connection} busy={busy} debug={debug} onDebug={setDebug}
     onSaveDebug={async()=>{if(await run(()=>api('/fnb/debug','PUT',{debug_browser:debug,version:connection.version}),'Browser mode saved'))await load()}}
     editing={editing} credentialForm={credentialForm} onEdit={()=>setEditing(true)}
     onDisconnect={async()=>{
      if(await run(()=>api('/fnb','DELETE'),'FNB disconnected. Saved login details removed.')){
       setEditing(false);setUsername('');setPassword('');await load();refresh()
      }
     }}
    />
   </>}
  <div className="banking-navigation">
   <Button variant="quiet" onClick={onAccounts}>Open Accounts</Button>
   <Button variant="quiet" onClick={onTransactions}>Get transactions</Button>
  </div>
 </section>
}
