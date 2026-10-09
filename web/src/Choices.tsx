import {usePagedList,ListStatus,ListNavigation} from './PagedList'
import {useId,useState,useEffect,useRef,type ReactNode} from 'react'
import {Check,ChevronRight,Plus} from 'lucide-react'
import {Button,Empty,Field,Form,Modal,Loading,Spinner} from './ui'
import {api} from './api'
import {type PageProps} from './shared/types'

export type Choice={id:number;name:string;detail?:string;color?:string;usage?:number;category_id?:number|null;spending_group_id?:number|null}
export function GroupDot({color}: {color?:string}){return <span aria-hidden="true" className={'group-dot '+(color||'slate')}/>}
export function ChoiceField({label,value,options,onChange,empty='Not set',disabled=false,mostUsed=false,create,quickCreate,onError,source,validationError,hint}: {label:string;value:number|null;options:Choice[];onChange:(value:number|null,choice?:Choice)=>void;empty?:string;disabled?:boolean;mostUsed?:boolean;create?:(name:string,done:(id:number)=>void)=>ReactNode;quickCreate?:(name:string,kind:'expense'|'income')=>Promise<number>;onError?:(message:string)=>void;source?:string;validationError?:string;hint?:string}){
 const id=useId(),[open,setOpen]=useState(false),[query,setQuery]=useState(''),[creating,setCreating]=useState(false),[saving,setSaving]=useState(false),[error,setError]=useState(''),[kind,setKind]=useState<'expense'|'income'>('expense')
 const search=useRef<HTMLInputElement>(null),trigger=useRef<HTMLButtonElement>(null),wasOpen=useRef(false),parentScroll=useRef<{element:HTMLElement;top:number}|null>(null)
 useEffect(()=>{
  if(open&&!creating)search.current?.focus()
  let frame=0
  if(!open&&wasOpen.current)frame=requestAnimationFrame(()=>{
   trigger.current?.focus({preventScroll:true})
   if(parentScroll.current)parentScroll.current.element.scrollTop=parentScroll.current.top
  })
  wasOpen.current=open
  return()=>cancelAnimationFrame(frame)
 },[open,creating])
 const list=usePagedList(open&&source?source:''),[hydrated,setHydrated]=useState<Choice|null>(null),[popular,setPopular]=useState<Choice[]>([]),[hydrating,setHydrating]=useState(false),[popularLoading,setPopularLoading]=useState(false)
 const convert=(o:Record<string,any>):Choice=>({id:o.id,name:o.name,color:o.color,detail:o.kind==='income'?'Income':o.kind==='expense'?'Expense':undefined,usage:o.usage_count,category_id:o.category_id,spending_group_id:o.spending_group_id})
 useEffect(()=>{list.setQuery(query)},[query])
 useEffect(()=>{if(!source||!value||options.some(o=>o.id===value)||hydrated?.id===value){setHydrating(false);return}let alive=true;setHydrating(true);api(source.replace('active=1','active=0')+(source.includes('?')?'&':'?')+'page=0&id='+value).then(v=>alive&&setHydrated(v.items[0]?convert(v.items[0]):null)).catch(e=>alive&&onError?.(e.message)).finally(()=>alive&&setHydrating(false));return()=>{alive=false}},[source,value,options,hydrated?.id])
 useEffect(()=>{if(!open||!mostUsed||!source)return;let alive=true;setPopularLoading(true);api(source+(source.includes('?')?'&':'?')+'page=0&page_size=5&sort=usage').then(v=>alive&&setPopular(v.items.filter((o:Record<string,any>)=>o.usage_count>0).map(convert))).catch(e=>alive&&onError?.(e.message)).finally(()=>alive&&setPopularLoading(false));return()=>{alive=false}},[open,source,mostUsed])
 const selected=options.find(o=>o.id===value)||list.items.map(convert).find(o=>o.id===value)||(hydrated?.id===value?hydrated:null)
 const filtered=source?list.items.map(convert):options.filter(o=>(o.name+' '+(o.detail||'')).toLocaleLowerCase().includes(query.trim().toLocaleLowerCase()))
 const exact=[...options,...filtered].find(o=>o.name.trim().toLowerCase()===query.trim().toLowerCase())
 const used=mostUsed&&!query.trim()?(source?popular:options).filter(o=>(o.usage||0)>0).sort((a,b)=>(b.usage||0)-(a.usage||0)||a.name.localeCompare(b.name)).slice(0,5):[]
 const choose=(value:number|null)=>{onChange(value,value===null?undefined:[...options,...filtered,...popular].find(o=>o.id===value));setOpen(false)}
 const submit=async()=>{
  if(exact){choose(exact.id);return}
  if(!query.trim()||saving)return
  if(quickCreate){setSaving(true);setError('');try{choose(await quickCreate(query.trim(),kind))}catch(e){const message=(e as Error).message;if(message==='Category already exists'||!onError)setError(message);else onError(message)}finally{setSaving(false)}}
  else if(create)setCreating(true)
 }
 const rows=(items:Choice[])=>items.map(o=><button type="button" className="choice-row" key={o.id} disabled={saving} aria-pressed={value===o.id} onClick={()=>choose(o.id)}><GroupDot color={o.color}/><span><strong>{o.name}</strong>{o.detail&&<small>{o.detail}</small>}</span>{value===o.id&&<Check size={18} aria-label="Selected"/>}</button>)
 return <div className="field"><label id={id+'-label'} htmlFor={id}>{label}</label><button ref={trigger} id={id} type="button" className="choice-control" aria-labelledby={id+'-label '+id+'-value'} aria-haspopup="dialog" aria-invalid={!!validationError||undefined} aria-describedby={validationError?id+'-error':hint?id+'-hint':undefined} disabled={disabled||hydrating} aria-busy={hydrating||undefined} onClick={()=>{const parent=trigger.current?.closest<HTMLElement>('.modal-body');parentScroll.current=parent?{element:parent,top:parent.scrollTop}:null;setQuery('');setCreating(false);setError('');setKind('expense');setOpen(true)}}>{selected?.color&&<GroupDot color={selected.color}/>}<span id={id+'-value'}>{selected?.name||(hydrating?'Loading':value?'Unavailable':empty)}</span>{hydrating?<Spinner/>:<ChevronRight size={16} aria-hidden="true"/>}</button>
 {hint&&<small id={id+'-hint'}>{hint}</small>}
 {validationError?<small className="field-error" id={id+'-error'} role="alert">{validationError}</small>:<span className="field-error-space" aria-hidden="true"/>}
 {open&&<Modal size="compact" stable title={creating?'Create category':'Select '+label.toLowerCase()} onClose={()=>{if(!saving)setOpen(false)}}>{creating&&create?<>{create(query,id=>choose(id))}<Button type="button" className="choice-create" onClick={()=>setCreating(false)}>Back to categories</Button></>:<Form onSubmit={submit}>
 <Field label={'Search '+label.toLowerCase()} hint={quickCreate?'Choose an existing category, or type a new name and press Enter.':undefined} serverError={error} validate={text=>quickCreate&&text.trim()&&!exact&&new TextEncoder().encode(text.trim()).length>80?'Use a shorter category name.':''}><input ref={search} autoFocus disabled={saving} placeholder={'Search '+label.toLowerCase()+'…'} value={query} onChange={e=>{setQuery(e.target.value);setError('')}}/></Field>
 <div className="choice-list"><button type="button" className="choice-row" disabled={saving} aria-pressed={value===null} onClick={()=>choose(null)}><span>{empty}</span>{value===null&&<Check size={18} aria-label="Selected"/>}</button>
 {popularLoading&&!query.trim()&&<Loading>Loading most used</Loading>}
 {used.length>0&&<><h3>Most used</h3>{rows(used)}<h3>All categories</h3></>}{source&&<ListStatus list={list}/>} {rows(filtered)} {source&&<ListNavigation list={list}/>}
 {!filtered.length&&<Empty kind="filtered" title="No matches">{quickCreate?'Create this category below.':'Try another search'+(create?' or create a category.':'.')}</Empty>}</div>
 {quickCreate&&query.trim()&&!exact&&<Field label="New category type"><select value={kind} disabled={saving} onChange={e=>setKind(e.target.value as 'expense'|'income')}><option value="expense">Expense (refunds reduce spending)</option><option value="income">Income</option></select></Field>}
 {(create||quickCreate)&&!exact&&<Button type={query.trim()?'submit':'button'} className="choice-create" loading={saving} disabled={saving} onClick={()=>{if(!query.trim())setCreating(true)}}><Plus size={16}/>{saving?'Creating':query.trim()?'Create “'+query.trim()+'”':'Create category'}</Button>}
 </Form>}</Modal>}</div>
}

export function CategoryChoice({data,value,onChange,refresh,notify,label='Category',disabled=false,validationError}: {data:PageProps['data'];value:number|null;onChange:(value:number|null)=>void;refresh:()=>void;notify:PageProps['notify'];label?:string;disabled?:boolean;validationError?:string}){
 const[created,setCreated]=useState<Choice[]>([])
 return <ChoiceField source="/categories?active=1" validationError={validationError} label={label} value={value} onChange={onChange} disabled={disabled} empty="Uncategorized" mostUsed onError={message=>notify(message,true)} options={[...data.categories.filter(c=>!c.archived||c.id===value).map(c=>({id:c.id,name:c.name,detail:c.kind==='income'?'Income':'Expense',usage:c.usage_count})),...created.filter(c=>!data.categories.some(existing=>existing.id===c.id))]} quickCreate={data.user.budget_member&&!disabled?async(name,kind)=>{
  try{
   const v=await api('/categories','POST',{name,kind})
   setCreated(old=>[...old,{id:v.id,name,detail:kind==='income'?'Income':'Expense'}]);refresh();return v.id
  }catch(e){
   // A concurrent creator may have saved the same category between search and submit.
   if((e as Error).message==='Category already exists'){
    const current=(await api('/categories?page=0&page_size=100&q='+encodeURIComponent(name))).items
    const existing=current.find((c:Record<string,unknown>)=>String(c.name).toLowerCase()===name.toLowerCase()&&c.kind===kind)
    if(existing){setCreated(old=>[...old,{id:existing.id,name:existing.name,detail:kind==='income'?'Income':'Expense'}]);refresh();return existing.id}
   }
   throw e
  }
 }:undefined} create={data.user.budget_member&&!disabled?(name,done)=><CreateCategory name={name} done={id=>{refresh();done(id)}} notify={notify}/>:undefined}/>
}
export function CreateCategory({name:initial,done,notify,expenseOnly=false}:{name:string;done:(id:number,name?:string)=>void;notify:PageProps['notify'];expenseOnly?:boolean}){
 const[name,setName]=useState(initial.trim()),[kind,setKind]=useState('expense'),[error,setError]=useState(''),[busy,setBusy]=useState(false)
 return <Form onSubmit={async()=>{setBusy(true);try{const body={name,kind};const v=await api('/categories','POST',body);done(v.id,name)}catch(e){const message=(e as Error).message;if(message==='Category already exists')setError(message);else notify(message,true)}finally{setBusy(false)}}}>
 <Field label="Category name" serverError={error} validate={value=>!value.trim()?'Enter a category name.':new TextEncoder().encode(value).length>80?'Use a shorter category name.':''}><input autoFocus required maxLength={80} value={name} onChange={e=>{setName(e.target.value);setError('')}}/></Field>
 <Field label="Type"><select disabled={expenseOnly} value={kind} onChange={e=>{setKind(e.target.value as 'expense'|'income')}}><option value="expense">Expense</option><option value="income">Income</option></select></Field>
 <Button variant="primary" type="submit" loading={busy} disabled={busy}>Create category</Button>
 </Form>
}
