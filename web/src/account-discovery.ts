export const accountNameError=(value:string)=>!value.trim()?'Enter an account name.':new TextEncoder().encode(value).length>100?'Use a shorter account name.':''
export const accountNumberError=(value:string)=>/^[0-9]{3,64}$/.test(value.trim())?'':'Enter the full numeric FNB account number.'
export function parseAccountDiscovery(text:string):{name:string;bank_id:string}[]{
 const invalid=()=>{throw new Error('Choose a valid FNB account discovery file containing account names and numbers only.')}
 let report:any;try{report=JSON.parse(text)}catch{return invalid()}
 if(!report||report.schema_version!==1||report.source!=='fnb-account-discovery'||!Array.isArray(report.accounts)||!report.accounts.length||report.accounts.length>100||Object.keys(report).some(key=>!['schema_version','source','accounts'].includes(key)))return invalid()
 const seen=new Set<string>()
 return report.accounts.map((row:any)=>{
  if(!row||typeof row.name!=='string'||typeof row.bank_id!=='string'||Object.keys(row).some(key=>!['name','bank_id'].includes(key)))return invalid()
  const name=row.name.trim(),bank_id=row.bank_id.replace(/\s/g,'')
  if(accountNameError(name)||!bank_id||bank_id.length>64||!/^[0-9xX*•●]+$/.test(bank_id)||seen.has(bank_id))return invalid()
  seen.add(bank_id);return {name,bank_id}
 })
}
