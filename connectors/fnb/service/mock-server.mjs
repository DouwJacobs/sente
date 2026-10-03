import {createServer} from 'node:http'
import {timingSafeEqual} from 'node:crypto'
const report={run_id:'synthetic-run',bank_id:'12345678901',currency:'ZAR',account_type:'Cheque',balance_decimal:'912.34',balance_date:'2026-10-02',transactions:[{date:'2026-10-01',amount_decimal:'-0.29',description:'Synthetic purchase',source_reference:'synthetic-reference',status:'posted'}]}
export function createMockServer({mode,token}){
 if(mode!=='mock')throw new Error('Only explicit mock mode is supported')
 if(typeof token!=='string'||Buffer.byteLength(token)<32||Buffer.byteLength(token)>4096)throw new Error('A dedicated transport token is required')
 const expected=Buffer.from('Bearer '+token)
 return createServer((req,res)=>{
  const send=(status,body)=>{res.writeHead(status,{'Content-Type':'application/json','Cache-Control':'no-store','X-Content-Type-Options':'nosniff'});res.end(JSON.stringify(body))}
  // Every route is authenticated, including health; no request body or secrets accepted.
  const supplied=Buffer.from(req.headers.authorization||'')
  if(supplied.length!==expected.length||!timingSafeEqual(supplied,expected)){send(401,{error:'Unauthorized'});return}
  if(req.method!=='GET'){send(405,{error:'Read-only mock service'});return}
  switch(req.url){
   case '/v1/status':send(200,{mode:'mock',live_enabled:false,credential_input_enabled:false});break
   case '/v1/accounts':send(200,[{bank_id:report.bank_id,name:'Synthetic account',currency:'ZAR',account_type:'Cheque'}]);break
   case '/v1/report':send(200,report);break
   default:send(404,{error:'Not found'})
  }
 })
}
