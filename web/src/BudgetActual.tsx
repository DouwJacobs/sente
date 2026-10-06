import {money} from './api'

export function BudgetActual({name,target,spent,hasBudget=true}:{name:string;target:number;spent:number;hasBudget?:boolean}){
 const remaining=target-spent,over=remaining<0
 return <span className="budget-actual">
  {hasBudget&&<span className="budget-figure"><small>Budget</small><strong>{target>0?money(target):'—'}</strong></span>}
  <span className="budget-figure"><small>Spent</small><strong>{money(spent)}</strong></span>
  {hasBudget&&<span className={'budget-figure '+(target>0&&over?'negative':'')}><small>{target>0?(over?'Over budget':'Left'):'Budget status'}</small><strong>{target>0?money(Math.abs(remaining)):'No budget set'}</strong></span>}
  {hasBudget&&target>0&&<progress className={over?'over-budget':undefined} aria-label={name+' budget used'} max={target} value={Math.max(0,spent)}/>}
 </span>
}
