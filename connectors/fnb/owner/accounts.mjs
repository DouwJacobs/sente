// Account-only selectors adapted from the pinned GPL-3.0 fnb-api reference.
// See ../upstream/LICENSE and ../UPSTREAM.json. No upstream runtime is imported.
export class ProbeError extends Error {constructor(code,diagnostics){super(code);this.code=code;this.diagnostics=diagnostics}}
export function normalizeAccounts(raw,{skipUnsupported=false,onSkipped}={}){
 if(!Array.isArray(raw)||!raw.length||raw.length>100)throw new ProbeError('ACCOUNT_LIST_NOT_FOUND')
 const accounts=[],seen=new Set(),skipped=[]
 for(const [index,row] of raw.entries()){
  const name=typeof row?.name==='string'?row.name.trim():'',bank_id=typeof row?.bank_id==='string'?row.bank_id.replace(/\s/g,''):''
  if(!name||Buffer.byteLength(name)>100)throw new ProbeError('ACCOUNT_LAYOUT_CHANGED')
  if(!bank_id||bank_id.length>64||!/^[0-9xX*•●]+$/.test(bank_id)){
   if(!skipUnsupported)throw new ProbeError('ACCOUNT_LAYOUT_CHANGED')
   skipped.push(index+1);continue
  }
  if(seen.has(bank_id))throw new ProbeError('ACCOUNT_LAYOUT_CHANGED')
  seen.add(bank_id);accounts.push({name,bank_id})
 }
 if(!accounts.length)throw new ProbeError('ACCOUNT_LIST_NOT_FOUND')
 if(skipped.length)onSkipped?.(skipped)
 return {schema_version:1,source:'fnb-account-discovery',accounts}
}
// This function executes in the owner browser. Only two explicitly named text
// fields are read. Diagnostics contain fixed counters, never DOM strings.
export function readAccountDOM(){
 const visible=node=>{
  if(node.matches('input,textarea,select,[contenteditable]'))return false
  const style=getComputedStyle(node)
  return node.getClientRects().length>0&&style.visibility!=='hidden'&&style.visibility!=='collapse'
 }
 const allNames=[...document.querySelectorAll('[name="nickname"]')]
 const allNumbers=[...document.querySelectorAll('[name="accountNumber"]')]
 const names=allNames.filter(visible),numbers=allNumbers.filter(visible)
 const diagnostics={name_nodes:allNames.length,number_nodes:allNumbers.length,visible_names:names.length,visible_numbers:numbers.length,blank_names:0,long_names:0,invalid_numbers:0,duplicate_numbers:0}
 if(!names.length||names.length!==numbers.length)return {raw:null,diagnostics}
 const seen=new Set()
 const raw=names.map((node,i)=>{
  const name=(node.textContent||'').trim(),bank_id=(numbers[i].textContent||'').replace(/\s/g,'')
  if(!name)diagnostics.blank_names++
  if(new TextEncoder().encode(name).length>100)diagnostics.long_names++
  if(!bank_id||bank_id.length>64||!/^[0-9xX*•●]+$/.test(bank_id))diagnostics.invalid_numbers++
  if(seen.has(bank_id))diagnostics.duplicate_numbers++
  seen.add(bank_id)
  return {name,bank_id}
 })
 return {raw,diagnostics}
}
export async function extractAccounts(page,options){
 let url
 try{url=new URL(page.url())}catch{throw new ProbeError('BANK_PAGE_REQUIRED')}
 if(url.protocol!=='https:'||!(url.hostname==='fnb.co.za'||url.hostname.endsWith('.fnb.co.za')))throw new ProbeError('BANK_PAGE_REQUIRED')
 const result=await page.evaluate(readAccountDOM)
 try{return normalizeAccounts(result.raw,options)}catch(err){
  if(err instanceof ProbeError)err.diagnostics=result.diagnostics
  throw err
 }
}
export function diagnosticLine(details){
 // Rebuild an allowlisted numeric object; never serialize arbitrary errors.
 const safe={}
 for(const key of ['name_nodes','number_nodes','visible_names','visible_numbers','blank_names','long_names','invalid_numbers','duplicate_numbers']){
  if(Number.isSafeInteger(details?.[key])&&details[key]>=0)safe[key]=details[key]
 }
 return Object.keys(safe).length?'LAYOUT_COUNTS: '+JSON.stringify(safe):''
}
export const errorMessages={
 ACCOUNT_LIST_NOT_FOUND:'No account list was found. Open the FNB Accounts summary and try again.',
 ACCOUNT_LAYOUT_CHANGED:'The account layout could not be read safely. No account file was saved.',
 BANK_PAGE_REQUIRED:'Open the FNB Accounts summary in the temporary browser window.',
 BROWSER_NOT_FOUND:'Chrome or Edge was not found. Supply its executable path with --browser.',
 DEPENDENCY_MISSING:'Install the owner-test dependencies before running the live test.',
 OUTPUT_EXISTS:'The output file already exists. Choose another --output path.',
 OUTPUT_IN_WORKSPACE:'Choose an output location outside the source workspace.',
 TEST_FAILED:'The test failed. No raw browser details are printed; share only this error code.',
 CANCELLED:'Test cancelled.'
}
