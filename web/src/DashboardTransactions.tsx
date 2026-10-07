import {MerchantAvatar} from './MerchantAvatar'
import {useEffect,useRef,useState} from 'react'
import {api,money} from './api'
import {Button,Empty,Loading,Pagination} from './ui'
import {useTransactionAccess,type TransactionScope} from './TransactionAccess'
import type {PageProps,Row} from './shared/types'

// Reads allocation spending in exactly the scope shown by its dashboard branch.
export function DashboardTransactions({scope,revision,notify}:{scope:TransactionScope;revision:number;notify:PageProps['notify']}){
 const {openTransaction}=useTransactionAccess()
 const[items,setItems]=useState<Row[]>([]),[total,setTotal]=useState(0),[page,setPage]=useState(0),[loading,setLoading]=useState(true),[error,setError]=useState(''),[retry,setRetry]=useState(0)
 const snapshot=useRef({version:'',revision:-1,query:''}),query=new URLSearchParams({...(scope.income?{dashboard_income:'1'}:{dashboard_spending:'1'}),period:scope.period||'',...(scope.account?{account:scope.account}:{}),...(scope.group?{spending_group:scope.group}:{}),...(scope.category?{category:scope.category}:{} )}).toString()
 useEffect(()=>{setPage(0);snapshot.current={version:'',revision:-1,query}},[query])
 useEffect(()=>{let alive=true;setLoading(true);setError('')
  const params=new URLSearchParams(query);params.set('offset',String(page*20))
  if(page>0&&snapshot.current.query===query&&snapshot.current.revision===revision&&snapshot.current.version)params.set('list_version',snapshot.current.version)
  api('/transactions?'+params).then(result=>{if(!alive)return
   snapshot.current={version:result.list_version,revision,query}
   if(page>0&&page*20>=result.total){setPage(Math.max(0,Math.ceil(result.total/20)-1));return}
   setItems(result.items);setTotal(result.total)
  }).catch(e=>{if(!alive)return
   if(e.message==='This list changed. Start from the first page.'){snapshot.current.version='';setPage(0);setRetry(v=>v+1)}else{setError(e.message);notify(e.message,true)}
  }).finally(()=>{if(alive)setLoading(false)})
  return()=>{alive=false}
 },[query,page,revision,retry])
 return <div className="dashboard-transactions" aria-busy={loading}>
  {loading?<Loading>Loading transactions</Loading>:error?<Button onClick={()=>setRetry(v=>v+1)}>Retry transactions</Button>:<>
   {!items.length?<Empty title={scope.income?"No income transactions":"No spending transactions"}>There are no transactions behind this amount yet.</Empty>:items.map(t=><button type="button" className="dashboard-transaction" key={t.id} onClick={()=>openTransaction(t.id,undefined,query)}><span className="merchant-identity"><MerchantAvatar name={t.merchant_name||t.description} logo={t.merchant_logo}/><span><strong>{t.merchant_name||t.description}</strong>{t.merchant_name&&t.merchant_name!==t.description&&<small>{t.description}</small>}<small>{t.date} · {t.account_name}{t.review_state==='pending_review'?' · Needs category':''}</small></span></span><span className="dashboard-transaction-amount"><strong>{money(scope.income?t.matched_income_cents:t.matched_spend_cents)}</strong><small>{scope.income?(t.matched_income_cents<0?'Income reversal':'Income'):(t.matched_spend_cents<0?'Refund':'Spending')}{scope.category?' in this category':' in this group'}</small>{t.allocations.length>1&&<small>Transaction total {money(t.amount_cents)}</small>}</span></button>)}
   <Pagination page={page} total={total} size={20} loading={loading} onChange={setPage}/>
  </>}
 </div>
}

export function SpendingTransactions({label,...props}:{label:string;scope:TransactionScope;revision:number;notify:PageProps['notify']}){
 const[open,setOpen]=useState(false)
 return <details className="spending-transactions" onToggle={e=>setOpen(e.currentTarget.open)}><summary>{label}</summary>{open&&<DashboardTransactions {...props}/>}</details>
}
