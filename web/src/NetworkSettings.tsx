import {useEffect,useState} from 'react'
import {api} from './api'
import {Badge,Button,Field,Form,Loading} from './ui'
import {useTask} from './shared/useTask'
import {type PageProps,type Row} from './shared/types'
export function publicURLError(value:string){
 try{const u=new URL(value.trim());if(!['http:','https:'].includes(u.protocol)||!u.hostname||u.username||u.password||u.pathname!=='/'||u.search||u.hash||value.includes('?')||value.includes('#')||value.length>2048)throw Error();return ''}catch{return 'Enter a full http:// or https:// address without a folder path, login details, or URL parameters.'}
}
export function proxyError(value:string){
 if(!value.trim())return ''
 if(value.length>4096)return 'Trusted proxies must be at most 4096 characters'
 for(const item of value.split(',')){
  const parts=item.trim().split('/'),ip=parts[0],bits=parts[1]
  const v4=/^(?:\d{1,3}\.){3}\d{1,3}$/.test(ip)&&ip.split('.').every(part=>Number(part)<=255&&(part==='0'||!part.startsWith('0')))
  let v6=false;try{v6=ip.includes(':')&&!ip.includes('%')&&new URL('http://['+ip+']/').hostname.startsWith('[')}catch{}
  if((!v4&&!v6)||parts.length>2||(bits!==undefined&&(!/^\d+$/.test(bits)||Number(bits)<1||Number(bits)>(v4?32:128))))return 'Enter proxy IP addresses or network ranges separated by commas.'
 }
 return ''
}
export function NetworkSettings({notify}:PageProps){
 const[status,setStatus]=useState<Row|null>(null),[url,setURL]=useState(''),[proxies,setProxies]=useState(''),[urlError,setURLError]=useState(''),[trustError,setTrustError]=useState('')
 const[restarting,setRestarting]=useState(false),[restartURL,setRestartURL]=useState('')
 const[loading,setLoading]=useState(true),[retry,setRetry]=useState(0)
 const{busy,run}=useTask(notify)
 const update=(next:Row)=>{setStatus(next);setURL(next.saved.enabled?next.saved.public_url:next.environment.public_url);setProxies(next.saved.enabled?next.saved.trusted_proxies:next.environment.trusted_proxies);setURLError('');setTrustError('')}
 useEffect(()=>{let alive=true;setLoading(true);api('/network').then(next=>alive&&update(next)).catch(e=>alive&&notify(e.message,true)).finally(()=>alive&&setLoading(false));return()=>{alive=false}},[retry])
 if(!status)return <section className="panel"><h2>Network</h2>{loading?<Loading>Loading network settings</Loading>:<p role="alert">Unable to load network settings. <Button onClick={()=>setRetry(v=>v+1)}>Retry</Button></p>}</section>
 return <section className="panel network-settings"><div className="section-head"><h2>Network</h2><Badge tone={status.restart_required?'pending':'good'}>{status.restart_required?'Restart required':'In use'}</Badge></div>
 <p className="muted">Configure the tracker behind your reverse proxy. Domain forwarding and SSL certificates are managed in Nginx Proxy Manager.</p>
 <div className="line"><div><span>Active public URL</span><strong>{status.active.public_url}</strong></div><Badge>{status.active.source==='settings'?'Saved settings':'Environment'}</Badge></div>
 <div className="line"><div><span>Active trusted proxies</span><strong>{status.active.trusted_proxies||'None'}</strong></div></div>
 <Form onSubmit={async()=>{await run(async()=>update(await api('/network','PUT',{enabled:true,public_url:url,trusted_proxies:proxies,version:status.saved.version})),'Network settings saved; restart the tracker to apply',message=>{if(message.includes('public URL')){setURLError(message);return true}if(/proxies/i.test(message)){setTrustError(message);return true}return false})}}>
 <Field label="Public URL" validate={publicURLError} serverError={urlError} hint="The exact address you open, for example https://finance.example.com. Use its own web address without a folder path."><input disabled={restarting} required maxLength={2048} value={url} onChange={e=>{setURL(e.target.value);setURLError('')}}/></Field>
 <Field label="Trusted proxy addresses" validate={proxyError} serverError={trustError} hint="Comma-separated proxy IPs or CIDRs. Leave empty to ignore forwarded client addresses."><input disabled={restarting} maxLength={4096} value={proxies} onChange={e=>{setProxies(e.target.value);setTrustError('')}}/></Field>
 <p className="footnote">Save applies after a restart and overrides environment values. Configure forwarding and HTTPS first, then restart and open the saved URL. Your current session keeps working until restart.</p>
 <div className="editor-actions"><Button type="submit" variant="primary" loading={busy} disabled={busy||restarting}>Save network settings</Button><Button type="button" loading={busy} disabled={busy||restarting} onClick={()=>run(async()=>update(await api('/network')))}>Reload saved settings</Button></div>
 </Form>
 <div className="editor-actions"><Button variant="primary" loading={restarting} disabled={busy||restarting||!status.restart_available||url!==(status.saved.enabled?status.saved.public_url:status.environment.public_url)||proxies!==(status.saved.enabled?status.saved.trusted_proxies:status.environment.trusted_proxies)} onClick={()=>run(async()=>{
 await api('/network/restart','POST',{version:status.saved.version})
 setRestarting(true);setRestartURL(status.saved.enabled?status.saved.public_url:status.environment.public_url)
 },'Restart requested. Open the configured URL after the tracker restarts.')}>Restart tracker</Button></div>
 <p className="footnote">Save changes before restarting. Restart briefly disconnects all users and applies the saved public URL and proxy settings.</p>
 {restarting&&<p role="status">Tracker is restarting. <a href={restartURL}>Open tracker at {restartURL}</a></p>}
 <p className="footnote">Use environment settings to restore PUBLIC_URL and TRUSTED_PROXIES at the next restart. If you cannot sign in after changing the URL, stop the service and run finance reset-network.</p>
 <Button loading={busy} disabled={busy||restarting||!status.saved.enabled} onClick={()=>run(async()=>update(await api('/network','PUT',{enabled:false,version:status.saved.version})),'Environment settings selected; restart the tracker to apply')}>Use environment settings</Button>
 </section>
}
