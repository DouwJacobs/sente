import {DashboardTransactions,SpendingTransactions} from './DashboardTransactions'
import {BudgetActual} from './BudgetActual'
import {useEffect,useState} from 'react'
import {Pencil} from 'lucide-react'
import {api,money,decimal,cents} from './api'
import {moneyError} from './validation'
import {GroupDot} from './Choices'
import {Pagination,Modal,Button,Field,Form} from './ui'
import type {Row,PageProps} from './App'

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
 return <><button type="button" className="bucket-category" aria-label={c.name+' transactions'} aria-haspopup="dialog" onClick={()=>setOpen(true)}><span className="bucket-category-name"><strong>{c.name}</strong>{c.pending_cents!==0&&<small>{money(c.pending_cents)} pending</small>}{c.id!==0&&<small className="category-global-total">All groups: {money(c.total_spent_cents||0)} spent{hasTargets?' · '+money(c.total_target_cents||0)+' combined budget':''}</small>}</span><BudgetActual name={group.name+' · '+c.name} target={c.target_cents||0} spent={c.spent_cents} hasBudget={hasTargets}/></button>{open&&<Modal size="wide" title={c.name+' · '+group.name} onClose={()=>setOpen(false)}><section className="spending-detail-overview" aria-label="Category summary"><div className="spending-detail-heading"><div className="category-summary-title"><h3>Category summary</h3>{canEditBudget&&c.id!==0&&<Button variant="quiet" className="category-budget-edit-button" aria-label={'Edit budget for '+c.name} title={'Edit budget for '+c.name} onClick={()=>setEditingBudget(true)}><Pencil size={15} aria-hidden="true"/></Button>}</div><span className="muted">{periodName||'Selected budget period'}</span></div><BudgetActual name={group.name+' · '+c.name} target={targetCents} spent={c.spent_cents} hasBudget={hasTargets||targetCents>0}/></section><h3 className="spending-transactions-heading">Transactions</h3><DashboardTransactions scope={{category:c.id?String(c.id):'uncategorized',group:group.id?String(group.id):'unassigned',period,account}} revision={revision} notify={notify}/></Modal>}{editingBudget&&<CategoryBudgetModal category={c} group={group} period={period} notify={notify} onClose={()=>setEditingBudget(false)} onSaved={newAmount=>{setLocalTarget(newAmount);refresh?.()}}/>}</>
}

function CategoryBudgetModal({category:c,group,period,onClose,onSaved,notify}:{category:Row;group:Row;period:string;onClose:()=>void;onSaved:(newAmount:number)=>void;notify:PageProps['notify']}){
 const[amount,setAmount]=useState(c.target_cents?decimal(c.target_cents):'0.00'),[future,setFuture]=useState(false),[busy,setBusy]=useState(false)
 useEffect(()=>{
  let alive=true
  api('/periods/'+period+'/targets?group='+(group.id||0)+'&budget_only=1&page=0&id='+c.id).then(saved=>{
   if(!alive)return
   const item=saved.items?.[0]
   if(item){
    setAmount(decimal(item.amount_cents))
    setFuture(!!item.carry_forward)
   }
  }).catch(()=>{})
  return()=>{alive=false}
 },[period,group.id,c.id])
 const save=async(remove=false)=>{
  setBusy(true)
  try{
   const periodData=await api('/periods?page=0&id='+period)
   const version=periodData.items?.[0]?.version??1
   const targetCents=remove?0:cents(amount)
   await api('/periods/'+period+'/budget','PUT',{
    version,
    groups:[{
     group_id:group.id||0,
     remove:false,
     targets:[{
      category_id:c.id,
      amount_cents:targetCents,
      carry_forward:future,
      apply_upcoming:future,
      remove
     }]
    }]
   })
   notify(remove?'Budget limit removed':'Budget saved')
   onSaved(targetCents)
   onClose()
  }catch(e){
   notify((e as Error).message,true)
  }finally{
   setBusy(false)
  }
 }
 return <Modal size="compact" title={'Edit budget · '+c.name} onClose={onClose}>
  <Form onSubmit={e=>{e.preventDefault();void save(false)}}>
   <p className="muted">Set the budget limit for <strong>{c.name}</strong> in {group.name||'this group'}.</p>
   <Field label="Budget amount" validate={v=>moneyError(v,true)}>
    <input autoFocus inputMode="decimal" required value={amount} onChange={e=>setAmount(e.target.value)}/>
   </Field>
   <Field label="Use this category in">
    <select value={future?'future':'once'} onChange={e=>setFuture(e.target.value==='future')}>
     <option value="once">This budget only</option>
     <option value="future">This and upcoming budgets</option>
    </select>
   </Field>
   {future&&<p className="footnote">Use this amount in new periods and upcoming budgets where this category isn't already included.</p>}
   <div className="editor-actions">
    <Button type="submit" variant="primary" loading={busy} disabled={busy}>Save budget</Button>
    {c.target_cents>0&&<Button variant="quiet" disabled={busy} onClick={()=>void save(true)}>Remove limit</Button>}
    <Button onClick={onClose} disabled={busy}>Cancel</Button>
   </div>
  </Form>
 </Modal>
}

