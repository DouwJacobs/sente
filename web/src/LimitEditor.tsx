import {useEffect,useState,useRef} from 'react'
import {Plus} from 'lucide-react'
import {api,cents,decimal} from './api'
import {Field,Form,Button,Empty,validateFields} from './ui'
import {moneyError} from './validation'
import {CreateCategory} from './Choices'
import {usePagedList,ListStatus,ListNavigation} from './PagedList'
import {useTask,type PageProps,type Row} from './App'
export function LimitEditor({period,notify,refresh,revision,onDone}:{period:Row;notify:PageProps['notify'];refresh:()=>void;revision:number;onDone:()=>void}){
 const list=usePagedList('/periods/'+period.id+'/targets',revision),[amounts,setAmounts]=useState<Record<string,string>>({}),[creating,setCreating]=useState(false),[extra,setExtra]=useState<Row|null>(null)
 const form=useRef<HTMLFormElement>(null)
 const{busy,run}=useTask(notify)
 useEffect(()=>{setAmounts(old=>{const next={...old};for(const c of list.items)if(next[c.id]===undefined)next[c.id]=decimal(c.amount_cents);return next})},[list.items])
 const rows=[...list.items,...(extra&&!list.items.some(c=>c.id===extra.id)?[extra]:[])]
 if(creating)return <><CreateCategory name="" expenseOnly notify={notify} done={(id,name)=>{setCreating(false);setAmounts(old=>({...old,[id]:'0.00'}));setExtra({id,name:name||'New expense category',amount_cents:0});refresh()}}/><Button onClick={()=>setCreating(false)}>Back to limits</Button></>
 return <><Button onClick={()=>setCreating(true)}><Plus size={16}/>Add expense category</Button><Field label="Search expense categories"><input value={list.query} onChange={e=>(!form.current||validateFields(form.current))&&list.setQuery(e.target.value)}/></Field><ListStatus list={list}/><Form ref={form} onSubmit={async()=>{if(await run(()=>api('/targets/'+period.id,'PUT',{version:period.version,merge:true,targets:Object.entries(amounts).map(([id,value])=>({category_id:Number(id),amount_cents:cents(value)}))}),'Category limits saved')){refresh();onDone()}}}>{rows.map(c=><Field key={c.id} label={c.name} validate={value=>moneyError(value,true)}><input autoFocus={c.id===extra?.id} inputMode="decimal" required value={amounts[c.id]??decimal(c.amount_cents)} onChange={e=>setAmounts({...amounts,[c.id]:e.target.value})}/></Field>)}{!list.loading&&!rows.length&&<Empty title={list.query?'No expense categories found':'Add expense categories first'}>Try another search or add an expense category.</Empty>}<ListNavigation list={{...list,setPage:page=>{if(!form.current||validateFields(form.current))list.setPage(page)}}}/><p className="footnote">Save applies to the categories you have loaded and edited. Other limits are retained.</p><Button type="submit" variant="primary" loading={busy} disabled={list.loading||!!list.error||Object.keys(amounts).length===0}>Save limits</Button></Form></>
}
