import {type ReactNode} from 'react'
import {Button,Form} from '../../ui'
import {type Row} from '../../shared/types'

type Props = {
 connection:Row
 busy:boolean
 debug:boolean
 onDebug:(value:boolean)=>void
 onSaveDebug:()=>Promise<void>
 editing:boolean
 credentialForm:ReactNode
 onEdit:()=>void
 onDisconnect:()=>Promise<void>
}

export function FNBConnectionOptions({connection,busy,debug,onDebug,onSaveDebug,editing,credentialForm,onEdit,onDisconnect}:Props) {
 const locked=busy||connection.state==='refreshing'
 return <details className="banking-group">
  <summary><strong>Connection options</strong><span>Login details, browser and diagnostics</span></summary>
  <div className="banking-group-body">
   {editing?credentialForm:<Button disabled={locked} onClick={onEdit}>Update login details</Button>}
   <Form onSubmit={onSaveDebug}>
    <label className="check"><input type="checkbox" checked={debug} disabled={locked} onChange={event=>onDebug(event.target.checked)}/>Show Chrome during bank updates</label>
    <p className="footnote">Show the connection browser during manual and automatic updates.</p>
    <Button type="submit" loading={busy} disabled={locked}>Save browser mode</Button>
   </Form>
   <details className="banking-diagnostic-details">
    <summary>Troubleshooting details</summary>
    <div className="banking-diagnostic-body">
     <p className="footnote">Last attempt: {connection.last_attempt?new Date(connection.last_attempt).toLocaleString():'Not yet'}</p>
     {connection.last_error&&<p className="footnote">Error code: <code>{connection.last_error}</code></p>}
     <h3>Layout diagnostics (counts only)</h3>
     <p className="footnote">No account details or login details are included.</p>
     {Object.keys(connection.last_diagnostics||{}).length>0
      ?<code className="banking-diagnostics">BALANCE_COUNTS: {JSON.stringify(connection.last_diagnostics)}</code>
      :<p className="muted">No diagnostics from the last attempt.</p>}
    </div>
   </details>
   <details className="banking-disconnect">
    <summary>Disconnect FNB</summary>
    <div className="banking-diagnostic-body">
     <p className="footnote">Removes saved login details and automatic updates. Your accounts and transaction history stay.</p>
     <Button variant="danger" loading={busy} disabled={locked} onClick={onDisconnect}>Disconnect</Button>
    </div>
   </details>
  </div>
 </details>
}
