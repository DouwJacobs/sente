import {CreditCard,House,Landmark,PiggyBank,Wallet} from 'lucide-react'

export function AccountIcon({type}:{type?:string}){
 const key=(type||'').trim().toLowerCase().replace(/[ _-]+/g,' ')
 const [Icon,label]=['credit','credit card','creditcard','creditline'].includes(key)?[CreditCard,'Credit card account']:
  ['savings','saving'].includes(key)?[PiggyBank,'Savings account']:
  ['home loan','mortgage'].includes(key)?[House,'Home loan account']:
  ['cheque','checking','current','easy'].includes(key)?[Landmark,'Transactional account']:[Wallet,'Account type unavailable']
 return <span className="account-type-icon" role="img" aria-label={label as string}><Icon size={20} aria-hidden="true"/></span>
}
