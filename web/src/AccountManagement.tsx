import {useEffect,useState,useRef} from 'react'
import {Menu,Pencil,RefreshCw,Eye,EyeOff} from 'lucide-react'
import {usePagedList,ListStatus,ListNavigation} from './PagedList'
import {api} from './api'
import {Button,Field,Form,Modal,Loading} from './ui'
import {useTask,type PageProps,type Row} from './App'

export function useAccountManagement({data,revision,refresh,notify}:PageProps){
 const shown=usePagedList(data.user.admin?'/accounts/manage?hidden=0':'',revision),hidden=usePagedList(data.user.admin?'/accounts/manage?hidden=1':'',revision)
 const accounts=[...shown.items,...hidden.items]
 const[status,setStatus]=useState<Row|null>(null),[loading,setLoading]=useState(false),[editing,setEditing]=useState<Row|null>(null),[bankError,setBankError]=useState(''),[pending,setPending]=useState<number|null>(null)
 const{busy,run}=useTask(notify)
 useEffect(()=>{if(!data.user.admin)return;let alive=true;setLoading(true)
 api('/fnb?summary=1').then(s=>{if(alive)setStatus(s)}).catch(e=>alive&&notify(e.message,true)).finally(()=>alive&&setLoading(false));return()=>{alive=false}
 },[revision,data.user.admin])
 useEffect(()=>{if(!data.user.admin)return;let alive=true;const timer=setInterval(()=>{api('/fnb?summary=1').then(next=>{if(!alive)return;setStatus(next);if(status?.connection?.last_success!==next.connection?.last_success)refresh()}).catch(()=>{})},2000);return()=>{alive=false;clearInterval(timer)}},[data.user.admin,revision,status?.connection?.last_success])
 const connected=(id:number)=>!!(accounts.find(a=>a.id===id)||data.accounts.find(a=>a.id===id))?.fnb_connected
 const refreshing=(id?:number,mapped?:boolean)=>id!==undefined&&!(mapped??connected(id))?false:pending!==null?(pending===0||id===undefined||pending===id):status?.connection?.state==='refreshing'&&status.refresh_kind!=='transactions'&&(!status.refreshing_account_id||id===undefined||status.refreshing_account_id===id)
 const refreshBank=async(id=0)=>{if(pending!==null||status?.connection?.state==='refreshing')return;setPending(id);try{if(await run(()=>api('/fnb/refresh','POST',id?{account_id:id}:{}),'Accounts refreshed'))refresh()}finally{setPending(null)}}
 const visibility=async(a:Row)=>{if(await run(()=>api('/accounts/'+a.id+'/visibility','PUT',{hidden:!a.sync_hidden,version:a.version}),a.sync_hidden?'Account shown':'Account hidden'))refresh()}
 const edit=(a:Row)=>{setBankError('');setEditing({...a})}
 const editor=editing&&<Modal title={editing.id?'Edit account':'Add FNB account'} onClose={()=>setEditing(null)}><Form onSubmit={async()=>{const payload={name:editing.name,bank_id:editing.bank_id,household:!!editing.household,version:editing.version||0};if(await run(()=>api('/accounts'+(editing.id?'/'+editing.id:''),editing.id?'PUT':'POST',payload),'Account saved',message=>{if(message.startsWith('Bank account')){setBankError(message);return true}return false})){setEditing(null);refresh()}}}><Field label="Account name"><input required maxLength={100} value={editing.name} onChange={e=>setEditing({...editing,name:e.target.value})}/></Field><Field label="FNB account number" serverError={bankError} hint="Must match the account number in your CSV or OFX export."><input required minLength={3} maxLength={64} inputMode="numeric" value={editing.bank_id} onChange={e=>{setEditing({...editing,bank_id:e.target.value});setBankError('')}}/></Field><label className="check"><input type="checkbox" checked={!!editing.household} onChange={e=>setEditing({...editing,household:e.target.checked})}/>Include in the household budget</label><p className="muted">Household accounts give household members editor access. Access to private accounts is managed in Settings → Users & access.</p><Button type="submit" variant="primary" loading={busy}>Save account</Button></Form></Modal>
 return{shown,hidden,accounts,status,loading,edit,editor,visibility,refreshBank,refreshing,connected,busy,canRefresh:!!status?.connection&&pending===null&&status.connection.state!=='refreshing'}
}
export function AccountActions({account:a,management:m}:{account:Row;management:ReturnType<typeof useAccountManagement>}){
 const menuRef=useRef<HTMLDetailsElement>(null)
 useEffect(()=>{
  const closeOutside=(event:Event)=>{const menu=menuRef.current;if(menu?.open&&event.target instanceof Node&&!menu.contains(event.target))menu.open=false}
  document.addEventListener('pointerdown',closeOutside,true)
  document.addEventListener('focusin',closeOutside)
  return()=>{document.removeEventListener('pointerdown',closeOutside,true);document.removeEventListener('focusin',closeOutside)}
 },[])
 return <details ref={menuRef} className="account-actions" onKeyDown={event=>{if(event.key==='Escape'){event.preventDefault();event.currentTarget.open=false;event.currentTarget.querySelector('summary')?.focus()}}}><summary aria-label={'Actions for '+a.name}><Menu size={18}/></summary><div className="account-action-menu"><Button variant="quiet" disabled={m.busy||m.refreshing()} onClick={e=>{e.currentTarget.closest('details')?.removeAttribute('open');m.edit(a)}}><Pencil size={16}/>Edit account</Button>{!a.sync_hidden&&(a.fnb_connected||m.connected(a.id))&&<Button variant="quiet" loading={m.refreshing(a.id,!!a.fnb_connected)} disabled={!m.canRefresh||m.busy} onClick={()=>m.refreshBank(a.id)}><RefreshCw size={16}/>Refresh balance</Button>}<Button variant="quiet" loading={m.busy} disabled={m.busy||m.refreshing()||a.visibility_allowed===0||a.role==='viewer'} onClick={()=>m.visibility(a)}>{a.sync_hidden?<Eye size={16}/>:<EyeOff size={16}/>} {a.sync_hidden?'Show account':'Hide account'}</Button></div></details>
}
export function HiddenAccounts({management:m}:{management:ReturnType<typeof useAccountManagement>}){
 const[show,setShow]=useState(false),hidden=m.accounts.filter(a=>a.sync_hidden)
 return <><label className="check"><input type="checkbox" checked={show} onChange={e=>setShow(e.target.checked)}/>Show hidden accounts ({m.hidden.total})</label>{show&&<section className="panel account-settings"><h2>Hidden accounts</h2><p className="footnote">Hiding an account keeps its history and access settings. Show it again to include it in account lists and balance updates.</p><ListStatus list={m.hidden}/>{hidden.map(a=><div className="line" key={a.id}><div><strong>{a.name}</strong><small>Ending {a.bank_id.slice(-4)} · Hidden</small></div><AccountActions account={a} management={m}/></div>)}<ListNavigation list={m.hidden}/>{!m.hidden.loading&&!hidden.length&&<p className="muted">No hidden accounts.</p>}</section>}</>
}
