import {useEffect,useState,useRef} from 'react'
import {api} from './api'
import {Field,Button,Loading,Pagination} from './ui'
import {type Row} from './App'
export function usePagedList(url:string,revision=0,size=20){
 const stamp=useRef({version:'',revision,url,query:''})
 const[page,setPage]=useState(0),[query,setQuery]=useState(''),[items,setItems]=useState<Row[]>([]),[total,setTotal]=useState(0),[loading,setLoading]=useState(true),[error,setError]=useState(''),[retry,setRetry]=useState(0)
 useEffect(()=>{if(!url){setLoading(false);return}let alive=true;setLoading(true);setError('');api(url+(url.includes('?')?'&':'?')+new URLSearchParams({page:String(page),page_size:String(size),q:query,...(page>0&&stamp.current.revision===revision&&stamp.current.url===url&&stamp.current.query===query?{list_version:stamp.current.version}:{})})).then(v=>{if(alive){if(page>0&&page*size>=v.total){setPage(Math.max(0,Math.ceil(v.total/size)-1));return}stamp.current={version:v.list_version||'',revision,url,query};setItems(v.items);setTotal(v.total)}}).catch(e=>{if(alive){window.dispatchEvent(new CustomEvent('finance-request-error',{detail:e.message}));if(e.message==='This list changed. Start from the first page.'){stamp.current.version='';setPage(0);setRetry(v=>v+1)}else setError(e.message)}}).finally(()=>alive&&setLoading(false));return()=>{alive=false}},[url,revision,page,size,query,retry])
 return{page,setPage,query,setQuery:(q:string)=>{setQuery(q);setPage(0)},items,total,loading,error,retry:()=>setRetry(v=>v+1)}
}
export function ListStatus({list}:{list:ReturnType<typeof usePagedList>}){return <>{list.loading&&<Loading>Loading</Loading>}{list.error&&<p role="alert">{list.error} <Button onClick={list.retry}>Retry</Button></p>}</>}
export function ListNavigation({list}:{list:ReturnType<typeof usePagedList>}){return <Pagination page={list.page} total={list.total} loading={list.loading} onChange={list.setPage}/>}
const noOptions:Row[]=[]
// Bounded option pages with server search; current selection remains available across pages.
export function PagedSelect({url,label,value,onChange,options=noOptions,empty='Choose '+label.toLowerCase(),required=false,revision=0,nameKey='name',hint,optionLabel,onSelectRow}:{url:string;label:string;value:string|number;onChange:(value:string)=>void;options?:Row[];empty?:string;required?:boolean;revision?:number;nameKey?:string;hint?:string;optionLabel?:(row:Row)=>string;onSelectRow?:(row:Row|undefined)=>void}){
 const list=usePagedList(url,revision,50),[selected,setSelected]=useState<Row|null>(null)
 useEffect(()=>{if(!value){setSelected(null);return}const row=options.find(o=>String(o.id)===String(value))||list.items.find(o=>String(o.id)===String(value));if(row){setSelected(row);return}let alive=true;api(url+(url.includes('?')?'&':'?')+'page=0&id='+value).then(v=>{if(alive)setSelected(v.items[0]||null)}).catch(e=>{if(alive)window.dispatchEvent(new CustomEvent('finance-request-error',{detail:e.message}))});return()=>{alive=false}},[value,url,list.items,options])
 const rows=[...new Map([...(list.query?[]:options),...list.items,...(selected?[selected]:[])].map(o=>[o.id,o])).values()]
 return <><Field label={label} hint={hint}><select required={required} value={value} onChange={e=>{onChange(e.target.value);onSelectRow?.(rows.find(row=>String(row.id)===e.target.value))}}><option value="">{empty}</option>{rows.map(o=><option key={o.id} value={o.id}>{optionLabel?optionLabel(o):o[nameKey]}</option>)}</select></Field>{list.total>50||list.query?<details className="details"><summary>Find more {label.toLowerCase()} options</summary><Field label={'Search '+label.toLowerCase()}><input value={list.query} onChange={e=>list.setQuery(e.target.value)}/></Field><ListStatus list={list}/><ListNavigation list={list}/></details>:<ListStatus list={list}/>}</>
}
