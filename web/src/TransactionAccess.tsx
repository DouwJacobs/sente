import {createContext,useContext,useEffect,useRef,useState,type ReactNode} from 'react'
import {api} from './api'
import {Modal,Loading,Button} from './ui'
import {TransactionEditor} from './features/transactions/TransactionEditor'
import type {PageProps,Row} from './shared/types'

export type TransactionScope={income?:boolean;account?:string;period?:string;category?:string;group?:string;dateFrom?:string;dateTo?:string;excluded?:boolean}
type Access={openTransaction:(id:number,onSaved?:()=>void,query?:string)=>void;viewTransactions:(scope:TransactionScope)=>void}
const Context=createContext<Access|null>(null)
export function useTransactionAccess(){const value=useContext(Context);if(!value)throw new Error('Transaction access provider missing');return value}

// Every entry point hydrates the same authorized ledger record. Nested references
// keep the parent editor/draft mounted until the child editor is closed.
export function TransactionAccess({children,viewTransactions,...props}:PageProps&{children:ReactNode;viewTransactions:Access['viewTransactions']}){
 const active=useRef(new Set<number>()),openers=useRef(new Map<number,HTMLElement>())
 useEffect(()=>()=>{active.current.clear();openers.current.clear()},[])
 const pending=useRef(new Set<number>())
 const sequence=useRef(0),[editors,setEditors]=useState<{key:number;transaction:Row|null;onSaved?:()=>void;query?:string;processed?:number[];navigation?:Row;complete?:boolean;remaining?:number;endError?:boolean;busy?:boolean}[]>([])
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
 const checkQueue=async(key:number,query:string,restart=false)=>{
  if(pending.current.has(key))return
  pending.current.add(key)
  setEditors(old=>old.map(e=>e.key===key?{...e,busy:true}:e))
  const params=new URLSearchParams(query)
  for(const name of ["processed","anchor_id","anchor_date","page","page_size","offset","list_version"])params.delete(name)
  params.set("offset","0")
  try {
   const result=await api("/transactions?"+params)
   if(!active.current.has(key))return
   setEditors(old=>old.map(e=>{
    if(e.key!==key)return e
    if(restart&&result.items.length)return {...e,transaction:result.items[0],processed:[],complete:false,endError:false}
    return {...e,complete:true,remaining:result.total,endError:false}
   }))
  } catch(error) {
   if(active.current.has(key)){
    setEditors(old=>old.map(e=>e.key===key?{...e,complete:true,endError:true}:e))
    props.notify((error as Error).message,true)
   }
  } finally {
   pending.current.delete(key)
   if(active.current.has(key))setEditors(old=>old.map(e=>e.key===key?{...e,busy:false}:e))
  }
 }
 const navigate=async(key:number,direction:'previous'|'next',saved=false)=>{
  const entry=editors.find(e=>e.key===key);if(!entry?.transaction||entry.query===undefined)return
  if(pending.current.has(key))return
  pending.current.add(key)
  setEditors(old=>old.map(e=>e.key===key?{...e,busy:true}:e))
  const params=new URLSearchParams(entry.query),processed=saved?[...(entry.processed||[]),entry.transaction.id]:(entry.processed||[])
  params.set('anchor_id',String(entry.transaction.id));params.set('anchor_date',entry.transaction.date);params.set('processed',processed.join(','))
  try{
   const nav=await api('/transactions/navigation?'+params),target=nav[direction]||(saved?nav.previous:null)
   if(!target){pending.current.delete(key);if(saved){if(new URLSearchParams(entry.query).get('acceptance')==='needs_category')await checkQueue(key,entry.query);else setEditors(old=>old.map(e=>e.key===key?{...e,complete:true}:e))}return}
   const result=await api('/transactions?id='+target.id)
   if(!result.items.length)throw new Error('Transaction no longer accessible')
   if(active.current.has(key))setEditors(old=>old.map(e=>e.key===key?{...e,transaction:result.items[0],processed}:e))
  }catch(error){if(active.current.has(key))props.notify((error as Error).message,true)}
  finally{
   pending.current.delete(key)
   if(active.current.has(key))setEditors(old=>old.map(e=>e.key===key?{...e,busy:false}:e))
  }
 }
 return (
  <Context.Provider value={{openTransaction, viewTransactions}}>
   {children}
   {editors.map(entry => {
    if (entry.complete) {
     const filtered = entry.query && Array.from(new URLSearchParams(entry.query)).some(
      ([key, value]) => value && !["acceptance", "page", "page_size", "offset", "list_version"].includes(key),
     );
     const title = entry.endError
      ? "Review status unavailable"
      : entry.remaining
       ? "End of this pass"
       : filtered ? "No matches remain" : "Review complete";
     return (
      <Modal key={entry.key} title={title} onClose={() => {if (!entry.busy) close(entry.key)}}>
       {entry.endError ? (
        <Button loading={entry.busy} onClick={() => checkQueue(entry.key, entry.query!)}>Retry</Button>
       ) : entry.remaining ? (
        <>
         <p>{entry.remaining} transactions still need categories in this review.</p>
         <Button loading={entry.busy} onClick={() => checkQueue(entry.key, entry.query!, true)}>Review remaining</Button>
        </>
       ) : (
        <p>{filtered ? "No transactions remain in this review's selected scope." : "No transactions remain in this review."}</p>
       )}
      </Modal>
     );
    }
    if (entry.transaction) {
     return (
      <TransactionEditor
       key={entry.key + ":" + entry.transaction.id}
       transaction={entry.transaction}
       navigationBusy={entry.busy}
       {...props}
       onSaved={() => entry.onSaved?.()}
       reviewQuery={entry.query}
       processed={entry.processed}
       onNavigate={direction => navigate(entry.key, direction)}
       onSaveNext={() => navigate(entry.key, "next", true)}
       onClose={() => close(entry.key)}
      />
     );
    }
    return (
     <Modal key={entry.key} title="Transaction details" onClose={() => close(entry.key)}>
      <Loading>Loading transaction</Loading>
     </Modal>
    );
   })}
  </Context.Provider>
 );
}
