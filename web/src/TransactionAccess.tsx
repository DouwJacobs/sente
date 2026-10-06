import {createContext,useContext,useEffect,useRef,useState,type ReactNode} from 'react'
import {api} from './api'
import {Modal,Loading} from './ui'
import {TransactionEditor} from './Transactions'
import type {PageProps,Row} from './App'

export type TransactionScope={income?:boolean;account?:string;period?:string;category?:string;group?:string;dateFrom?:string;dateTo?:string;excluded?:boolean}
type Access={openTransaction:(id:number,onSaved?:()=>void,query?:string)=>void;viewTransactions:(scope:TransactionScope)=>void}
const Context=createContext<Access|null>(null)
export function useTransactionAccess(){const value=useContext(Context);if(!value)throw new Error('Transaction access provider missing');return value}

// Every entry point hydrates the same authorized ledger record. Nested references
// keep the parent editor/draft mounted until the child editor is closed.
export function TransactionAccess({children,viewTransactions,...props}:PageProps&{children:ReactNode;viewTransactions:Access['viewTransactions']}){
 const active=useRef(new Set<number>()),openers=useRef(new Map<number,HTMLElement>())
 useEffect(()=>()=>{active.current.clear();openers.current.clear()},[])
 const sequence=useRef(0),[editors,setEditors]=useState<{key:number;transaction:Row|null;onSaved?:()=>void;query?:string;processed?:number[];navigation?:Row;complete?:boolean}[]>([])
 const restoreFocus=(key:number)=>{const opener=openers.current.get(key);openers.current.delete(key);requestAnimationFrame(()=>{const top=Array.from(document.querySelectorAll('dialog[open]')).at(-1);if(opener?.isConnected&&(!top||top.contains(opener)))opener.focus()})}
 const close=(key:number)=>{for(const entry of active.current)if(entry>=key){active.current.delete(entry);if(entry!==key)openers.current.delete(entry)}setEditors(old=>old.filter(e=>e.key<key));restoreFocus(key)}
 const openTransaction=(id:number,onSaved?:()=>void,query?:string)=>{
  const key=++sequence.current
  active.current.add(key)
  if(document.activeElement instanceof HTMLElement)openers.current.set(key,document.activeElement)
  setEditors(old=>[...old,{key,transaction:null,onSaved,query,processed:[]}])
  api('/transactions?id='+encodeURIComponent(id)).then(result=>{
   if(!active.current.has(key))return
   if(!result.items.length)throw new Error('This transaction was not found, or you do not have access to its account.')
   setEditors(old=>old.map(e=>e.key===key?{...e,transaction:result.items[0]}:e))
  }).catch(error=>{if(!active.current.has(key))return;active.current.delete(key);setEditors(old=>old.filter(e=>e.key!==key));restoreFocus(key);props.notify(error.message,true)})
 }
 const navigate=async(key:number,direction:'previous'|'next',saved=false)=>{
  const entry=editors.find(e=>e.key===key);if(!entry?.transaction||entry.query===undefined)return
  const params=new URLSearchParams(entry.query),processed=saved?[...(entry.processed||[]),entry.transaction.id]:(entry.processed||[])
  params.set('anchor_id',String(entry.transaction.id));params.set('anchor_date',entry.transaction.date);params.set('processed',processed.join(','))
  try{
   const nav=await api('/transactions/navigation?'+params),target=nav[direction]||(saved?nav.previous:null)
   if(!target){if(saved)setEditors(old=>old.map(e=>e.key===key?{...e,complete:true}:e));return}
   const result=await api('/transactions?id='+target.id)
   if(!result.items.length)throw new Error('Transaction no longer accessible')
   if(active.current.has(key))setEditors(old=>old.map(e=>e.key===key?{...e,transaction:result.items[0],processed}:e))
  }catch(error){props.notify((error as Error).message,true)}
 }
 return <Context.Provider value={{openTransaction,viewTransactions}}>{children}{editors.map(e=>e.complete?<Modal key={e.key} title="Review complete" onClose={()=>close(e.key)}><p>No transactions remain in this review.</p></Modal>:e.transaction?<TransactionEditor key={e.key+':'+e.transaction.id} transaction={e.transaction} {...props} onSaved={()=>e.onSaved?.()} reviewQuery={e.query} processed={e.processed} onNavigate={direction=>navigate(e.key,direction)} onSaveNext={()=>navigate(e.key,'next',true)} onClose={()=>close(e.key)}/>:<Modal key={e.key} title="Transaction details" onClose={()=>close(e.key)}><Loading>Loading transaction</Loading></Modal>)}</Context.Provider>
}
