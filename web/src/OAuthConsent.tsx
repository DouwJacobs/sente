import {useEffect,useState} from 'react'
import {api} from './api'
import {Button,Form,Loading,Empty} from './ui'
import {useTask} from './shared/useTask'
import {type PageProps,type Row} from './shared/types'
import {MCPPermissionFields,reviewPermissions} from './MCPPermissions'

export function OAuthConsent({notify,onSignOut}:Pick<PageProps,'notify'>&{onSignOut:()=>Promise<void>}){
 const[state,setState]=useState<Row|null>(null),[permissions,setPermissions]=useState<Row>(reviewPermissions),[error,setError]=useState('')
 const{busy,run}=useTask(notify)
 const id=new URLSearchParams(window.location.search).get('request')||''
 useEffect(()=>{let alive=true;api('/mcp/authorization/'+encodeURIComponent(id)).then(v=>alive&&setState(v)).catch(e=>{if(alive){setError(e.message);notify(e.message,true)}});return()=>{alive=false}},[id])
 const decide=(allow:boolean)=>run(async()=>{const result=await api('/mcp/authorization/'+encodeURIComponent(id),'POST',{allow,permissions:allow?permissions:undefined});window.location.assign(result.redirect)})
 return <main className="login"><section className="login-panel"><h1>Approve agent connection</h1>
 {error?<Empty title="Connection unavailable">{error}</Empty>:!state?<Loading>Loading connection request</Loading>:<>
 <p><strong>{state.client_name}</strong> wants to connect to Finance Tracker.</p><p className="muted">Signed in as <strong>{state.username}</strong>. The agent name is supplied by its client. Check that you started this connection.</p>
 <p className="footnote">Approval returns to {state.redirect_origin}. This request expires at {new Date(state.expires_at*1000).toLocaleTimeString('en-ZA')}.</p>
 <Form onSubmit={()=>decide(true)}><p>The agent can read the transactions and accounts you can access, plus your permitted categories, rules and budget. Bank identifiers, credentials, notes and import source details are excluded. Personal text in merchant descriptions may remain.</p>
 <MCPPermissionFields value={permissions} onChange={setPermissions} accounts={state.accounts} canPropose={state.can_propose}/>
 <p className="footnote">Connection access expires after 90 days and can be revoked in Settings → MCP.</p>
 <div className="editor-actions"><Button type="submit" variant="primary" disabled={busy} loading={busy}>Approve connection</Button><Button disabled={busy} onClick={()=>decide(false)}>Deny</Button></div></Form>
 <div className="editor-actions"><Button disabled={busy} onClick={()=>run(onSignOut)}>Sign out</Button></div>
 </>}
 </section></main>
}
