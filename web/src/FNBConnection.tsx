import {useEffect,useState} from 'react'
import {api} from './api'
import {Button,Field,Form,Badge,Loading} from './ui'
import {useTask,type PageProps,type Row} from './App'
export function FNBConnection({data,revision,refresh,notify}:PageProps){
 const[status,setStatus]=useState<Row|null>(null),[username,setUsername]=useState(''),[password,setPassword]=useState(''),[hours,setHours]=useState('0'),[debug,setDebug]=useState(false),[editing,setEditing]=useState(false)
 const[loading,setLoading]=useState(true),[loadError,setLoadError]=useState(''),[retry,setRetry]=useState(0)
 const{busy,run}=useTask(notify)
 const load=()=>run(async()=>{const next=await api('/fnb?summary=1');setStatus(next);setHours(String(next.connection?.interval_hours||0));setDebug(!!next.connection?.debug_browser)})
 useEffect(()=>{let alive=true;setLoading(true);setLoadError('');api('/fnb?summary=1').then(next=>{if(alive){setStatus(next);setHours(String(next.connection?.interval_hours||0));setDebug(!!next.connection?.debug_browser)}}).catch(e=>{if(alive){setLoadError(e.message);notify(e.message,true)}}).finally(()=>alive&&setLoading(false));return()=>{alive=false}},[revision,retry])
 useEffect(()=>{
  if(!status?.connection)return
  let alive=true
  const timer=window.setInterval(()=>{api('/fnb?summary=1').then(next=>{if(!alive)return;if(next.connection?.last_success!==status.connection?.last_success)refresh();else setStatus(next)}).catch(()=>{})},15000)
  return()=>{alive=false;clearInterval(timer)}
 },[status?.connection?.last_success,!!status?.connection,revision])
 if(!data.user.admin)return null
 const connection=status?.connection
 const scheduleLabel=connection?.interval_hours?({6:'Every 6 hours',12:'Every 12 hours',24:'Every day',168:'Every week'} as Record<number,string>)[connection.interval_hours]:'Off'
 const dateLabel=(value:string|null)=>value?new Date(value).toLocaleString():'Not yet'
 const credentialForm=<Form onSubmit={async()=>{if(await run(()=>api('/fnb','PUT',{username,password,version:connection?.version||0}),'FNB credentials saved; scheduled refresh is paused')){setUsername('');setPassword('');setEditing(false);await load();refresh()}}}><Field label="FNB username" validate={value=>!value.trim()?'Enter your FNB username':new TextEncoder().encode(value).length>200?'FNB username is too long':''}><input autoComplete="off" required maxLength={200} value={username} onChange={e=>setUsername(e.target.value)}/></Field><Field label="FNB password" validate={value=>new TextEncoder().encode(value).length>512?'FNB password is too long':''}><input type="password" autoComplete="off" required maxLength={512} value={password} onChange={e=>setPassword(e.target.value)}/></Field><p className="footnote">Credentials are encrypted on this machine. Same-machine processes with sufficient access can recover them. FNB may require you to approve a login.</p><div className="editor-actions"><Button type="submit" variant="primary" loading={busy} disabled={busy}>{connection?'Replace credentials':'Connect FNB'}</Button>{connection&&<Button loading={busy} disabled={busy} onClick={()=>{setEditing(false);setUsername('');setPassword('')}}>Cancel</Button>}</div></Form>
 return <section className="panel banking-settings"><div className="section-head"><h2>FNB connection</h2>{connection&&<Badge tone={connection.state==='action_required'?'bad':undefined}>{connection.state==='action_required'?'Needs attention':connection.state==='refreshing'?'Refreshing':'Connected'}</Badge>}</div>
 {loadError&&status&&<p role="alert">Connection status could not refresh. <Button onClick={()=>setRetry(v=>v+1)}>Retry</Button></p>}
 <p className="muted">Sync ZAR account details and ledger balances. New accounts are private. Fetch transactions from Imports.</p>
 {!status?loading?<Loading>Loading connection</Loading>:<p role="alert">Unable to load connection status. <Button onClick={()=>setRetry(v=>v+1)}>Retry</Button></p>:!connection?<>{credentialForm}</>:<>
 <div className="banking-refresh"><Button variant="primary" loading={busy||connection.state==='refreshing'} disabled={busy||connection.state==='refreshing'} onClick={async()=>{await run(()=>api('/fnb/refresh','POST'),'FNB accounts refreshed');await load();refresh()}}>{connection.state==='refreshing'?'Refreshing':'Refresh now'}</Button><span className="muted">FNB may ask you to approve the login.</span></div>
 <dl className="banking-overview"><div><dt>Last successful refresh</dt><dd>{dateLabel(connection.last_success)}</dd></div><div><dt>Next scheduled refresh</dt><dd>{connection.state==='ready'&&connection.next_due?new Date(connection.next_due*1000).toLocaleString():connection.interval_hours?'Paused':'Off'}</dd></div></dl>
 {connection.state==='action_required'&&<p className="footnote">Automatic refresh is paused. Open Troubleshooting for details.</p>}
 <details className="banking-group"><summary><strong>Automatic refresh</strong><span>{scheduleLabel}{connection.state==='action_required'&&connection.interval_hours?' · Paused':''}</span></summary><div className="banking-group-body">
 <p className="footnote">Scheduled refreshes update accounts and balances while the tracker is running. Fetch transactions manually from Imports. Phone approval may still be required.</p>
 <Form onSubmit={async()=>{if(await run(()=>api('/fnb/schedule','PUT',{interval_hours:Number(hours),version:connection.version}),'Refresh schedule saved'))await load()}}><Field label="Refresh schedule"><select value={hours} onChange={e=>setHours(e.target.value)}><option value="0">Off — refresh manually</option><option value="6">Every 6 hours</option><option value="12">Every 12 hours</option><option value="24">Every day</option><option value="168">Every week</option></select></Field><Button type="submit" loading={busy} disabled={busy||connection.state==='refreshing'}>Save refresh schedule</Button></Form>
 </div></details>
 <details className="banking-group"><summary><strong>Troubleshooting</strong><span>Browser mode and layout diagnostics</span></summary><div className="banking-group-body">
 {connection.last_error&&<p className="error-text">{connection.last_error}. {connection.last_error==='SESSION_CONFLICT'?'Let the previous FNB session end or sign out, then refresh.':connection.last_error==='LOGOUT_REQUIRED'?'Sign out of FNB manually before another refresh.':connection.last_error.startsWith('TRANSACTION_')?'Retry Fetch FNB transactions in Imports.':'Retry Refresh now or replace credentials if needed.'}</p>}
 <p className="footnote">Last attempt: {dateLabel(connection.last_attempt)}</p>
 <Form onSubmit={async()=>{if(await run(()=>api('/fnb/debug','PUT',{debug_browser:debug,version:connection.version}),'Browser mode saved'))await load()}}><label className="check"><input type="checkbox" checked={debug} onChange={e=>setDebug(e.target.checked)}/>Debug mode — show Chrome during refresh</label><p className="footnote">Off runs headlessly. On opens Chrome for you to follow the refresh. Applies to manual and scheduled runs.</p><Button type="submit" loading={busy} disabled={busy||connection.state==='refreshing'}>Save browser mode</Button></Form>
 <h3>Layout diagnostics (counts only)</h3><p className="footnote">Safe to share; no account details or credentials are included.</p>{Object.keys(connection.last_diagnostics||{}).length>0?<code className="banking-diagnostics">BALANCE_COUNTS: {JSON.stringify(connection.last_diagnostics)}</code>:<p className="muted">No counts returned by the last attempt.</p>}
 </div></details>
 <details className="banking-group"><summary><strong>Connection settings</strong><span>Credentials and disconnect</span></summary><div className="banking-group-body">
 {editing?credentialForm:<Button loading={busy} disabled={busy||connection.state==='refreshing'} onClick={()=>setEditing(true)}>Replace credentials</Button>}
 <div className="banking-disconnect"><p className="footnote">Disconnect removes stored credentials and stops refreshes. Account records remain stored.</p><Button variant="danger" loading={busy} disabled={busy||connection.state==='refreshing'} onClick={async()=>{if(await run(()=>api('/fnb','DELETE'),'FNB disconnected; stored credentials removed')){setEditing(false);setUsername('');setPassword('');await load();refresh()}}}>Disconnect</Button></div>
 </div></details>
 </>}
 </section>
}
