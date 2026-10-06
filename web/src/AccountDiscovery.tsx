import {useRef,useState} from 'react'
import {api} from './api'
import {Button,Field,Form,Modal,Loading} from './ui'
import {parseAccountDiscovery,accountNameError,accountNumberError} from './account-discovery'
import {useTask,type PageProps} from './App'

type Draft={name:string;bank_id:string;household:boolean;imported:boolean;error:string}
export function AccountDiscovery({data,refresh,notify}:PageProps){
 const[open,setOpen]=useState(false),[drafts,setDrafts]=useState<Draft[]>([]),[fileError,setFileError]=useState('')
 const[reading,setReading]=useState(false)
 const{busy,run}=useTask(notify),fileRequest=useRef(0)
 const edit=(index:number,change:Partial<Draft>)=>setDrafts(rows=>rows.map((row,i)=>i===index?{...row,...change}:row))
 if(!data.user.admin)return null
 return <><Button onClick={()=>{fileRequest.current++;setOpen(true);setDrafts([]);setFileError('')}}>Import discovered accounts</Button>
 {open&&<Modal title="Import discovered accounts" onClose={()=>{if(!busy){fileRequest.current++;setOpen(false)}}}><p className="muted">Choose the account file created by the FNB discovery tool. Check each account before adding it. This file contains account names and numbers only.</p>
 <Field label="Discovered account file" serverError={fileError}><input type="file" accept=".json,application/json" disabled={busy} onChange={async e=>{
  const request=++fileRequest.current;setFileError('');setDrafts([]);const file=e.target.files?.[0];if(!file)return;setReading(true)
  try{if(file.size>65536)throw new Error('Choose an account file smaller than 64 KiB.');const accounts=parseAccountDiscovery(await file.text());if(request!==fileRequest.current)return;setDrafts(accounts.map(row=>({...row,household:false,imported:false,error:''})))}catch(err){if(request===fileRequest.current)setFileError((err as Error).message)}finally{if(request===fileRequest.current)setReading(false)}
 }}/></Field>
 {reading&&<Loading>Reading account file</Loading>}
 {drafts.map((row,index)=>{const existing=data.accounts.some(a=>a.bank_id===row.bank_id);const completed=row.imported||existing
 return <section className="panel" key={index}><h3>Account {index+1}</h3><Form onSubmit={async()=>{if(await run(()=>api('/accounts','POST',{name:row.name.trim(),bank_id:row.bank_id.trim(),household:row.household}),'Account added',message=>{if(message.startsWith('Bank account')){edit(index,{error:message});return true}return false})){edit(index,{imported:true,error:''});refresh()}}}>
 <Field label="Account name" validate={accountNameError}><input required disabled={completed||busy} value={row.name} onChange={e=>edit(index,{name:e.target.value})}/></Field>
 <Field label="FNB account number" validate={accountNumberError} serverError={row.error} hint="If FNB supplied a masked number, enter the full account number before adding it."><input required inputMode="numeric" disabled={completed||busy} value={row.bank_id} onChange={e=>edit(index,{bank_id:e.target.value,error:''})}/></Field>
 <label className="check"><input type="checkbox" disabled={completed||busy} checked={row.household} onChange={e=>edit(index,{household:e.target.checked})}/>Share with household budget members</label>
 {!completed&&<p className="muted">Private by default. A household account gives every household budget member editor access.</p>}
 {completed?<p role="status">{row.imported?'Account added.':'This account already exists; its settings are unchanged.'}</p>:<Button type="submit" variant="primary" loading={busy} disabled={busy}>Add this account</Button>}
 </Form></section>})}
 <Button loading={busy} disabled={busy} onClick={()=>{fileRequest.current++;setOpen(false)}}>Done</Button></Modal>}
 </>
}
