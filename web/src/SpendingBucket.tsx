import {DashboardTransactions,SpendingTransactions} from './DashboardTransactions'
import {BudgetActual} from './BudgetActual'
import {useEffect,useState} from 'react'
import {Pencil} from 'lucide-react'
import {api,money} from './api'
import {CategoryBudgetModal} from './features/budgets/CategoryBudgetModal'
import {GroupDot} from './Choices'
import {Pagination,Modal,Button} from './ui'
import type {Row,PageProps} from './shared/types'

export function SpendingBucket({group,period,account,groupPage,hasTargets,notify,revision,sort,periodName,canEditBudget,refresh}:{periodName?:string;sort?:string;group:Row;period:string;account:string;groupPage:number;hasTargets:boolean;notify:PageProps['notify'];revision:number;canEditBudget?:boolean;refresh?:()=>void}){
 const[page,setPage]=useState(0),[categories,setCategories]=useState<Row[]>(group.categories),[loading,setLoading]=useState(false)
 useEffect(()=>{if(page===0){setCategories(group.categories);return}let alive=true;setLoading(true)
  api('/dashboard?period='+period+'&account='+account+'&category_page=0&balance_page=0&group_page='+groupPage+'&group_id='+group.id+'&group_category_page='+page+'&sort='+(sort||'alphabetical')).then(d=>{if(alive)setCategories(d.spending_groups.find((g:Row)=>g.id===group.id)?.categories||[])}).catch(e=>{if(alive)notify(e.message,true)}).finally(()=>{if(alive)setLoading(false)})
  return()=>{alive=false}
 },[page,group,period,account,groupPage,sort])
 return <details className="spending-bucket"><summary><span className="bucket-name"><GroupDot color={group.color}/><span><strong>{group.name}</strong><small>{group.category_total} {group.category_total===1?'category':'categories'}</small></span></span><BudgetActual name={group.name} target={group.target_cents||0} spent={group.spent_cents} hasBudget={hasTargets}/></summary>
  <div className="bucket-categories" aria-busy={loading}><SpendingTransactions label="Group transactions" scope={{group:group.id?String(group.id):'unassigned',period,account}} revision={revision} notify={notify}/>{categories.map(c=><CategorySpending key={c.id} category={c} group={group} period={period} periodName={periodName} account={account} hasTargets={hasTargets} revision={revision} notify={notify} canEditBudget={canEditBudget} refresh={refresh}/>)}<Pagination page={page} total={group.category_total} loading={loading} onChange={setPage}/></div>
 </details>
}

function CategorySpending({category:c,group,period,account,hasTargets,revision,notify,periodName,canEditBudget,refresh}:{periodName?:string;category:Row;group:Row;period:string;account:string;hasTargets:boolean;revision:number;notify:PageProps['notify'];canEditBudget?:boolean;refresh?:()=>void}){
 const[open,setOpen]=useState(false),[editingBudget,setEditingBudget]=useState(false),[localTarget,setLocalTarget]=useState<number|null>(null)
 useEffect(()=>setLocalTarget(null),[c.target_cents])
 const targetCents=localTarget!==null?localTarget:(c.target_cents||0)
 return <><button type="button" className="bucket-category" aria-label={c.name+' transactions'} aria-haspopup="dialog" onClick={()=>setOpen(true)}><span className="bucket-category-name"><strong>{c.name}</strong>{c.pending_cents!==0&&<small>{money(c.pending_cents)} pending</small>}{c.id!==0&&<small className="category-global-total">All groups: {money(c.total_spent_cents||0)} spent{hasTargets?' · '+money(c.total_target_cents||0)+' combined budget':''}</small>}</span><BudgetActual name={group.name+' · '+c.name} target={c.target_cents||0} spent={c.spent_cents} hasBudget={hasTargets}/></button>{open&&<Modal size="wide" title={c.name+' · '+group.name} onClose={()=>setOpen(false)}><section className="spending-detail-overview" aria-label="Category summary"><div className="spending-detail-heading"><div className="category-summary-title"><h3>Category summary</h3>{canEditBudget&&c.id!==0&&<Button variant="quiet" className="category-budget-edit-button" aria-label={'Edit budget for '+c.name} title={'Edit budget for '+c.name} onClick={()=>setEditingBudget(true)}><Pencil size={15} aria-hidden="true"/></Button>}</div><span className="muted">{periodName||'Selected budget period'}</span></div><BudgetActual name={group.name+' · '+c.name} target={targetCents} spent={c.spent_cents} hasBudget={hasTargets||targetCents>0}/></section><h3 className="spending-transactions-heading">Transactions</h3><DashboardTransactions scope={{category:c.id?String(c.id):'uncategorized',group:group.id?String(group.id):'unassigned',period,account}} revision={revision} notify={notify}/></Modal>}{editingBudget&&<CategoryBudgetModal category={c} group={group} period={period} periodName={periodName} notify={notify} onClose={()=>setEditingBudget(false)} alertAvailable={!Number(account)} onSaved={newAmount=>{setLocalTarget(newAmount);refresh?.()}}/>}</>
}
