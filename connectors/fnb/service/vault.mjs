import {createCipheriv,createDecipheriv,randomBytes} from 'node:crypto'

function identity(ownerId,connectionId,keyVersion){
 if (![ownerId,connectionId,keyVersion].every(v=>typeof v==='string'&&v.length>0&&v.length<=128)) throw new Error('Invalid secret identity')
 return Buffer.from(JSON.stringify(['fnb-credentials-v1',ownerId,connectionId,keyVersion]))
}
function validKey(key){if(!Buffer.isBuffer(key)||key.length!==32)throw new Error('A 32-byte runtime key is required')}
function validCredentials(credentials){
 if(!credentials||typeof credentials.username!=='string'||typeof credentials.password!=='string'||!credentials.username.trim()||!credentials.password||credentials.username.length>256||credentials.password.length>4096)throw new Error('Invalid credentials')
}
// Internal worker primitives only. Never expose unseal through HTTP or tools.
export function sealCredentials(credentials,{ownerId,connectionId,keyVersion,key}){
 validKey(key);validCredentials(credentials)
 const aad=identity(ownerId,connectionId,keyVersion),nonce=randomBytes(12)
 const plaintext=Buffer.from(JSON.stringify({username:credentials.username,password:credentials.password}))
 try{
  const cipher=createCipheriv('aes-256-gcm',key,nonce);cipher.setAAD(aad)
  const ciphertext=Buffer.concat([cipher.update(plaintext),cipher.final()])
  return {format:1,keyVersion,nonce:nonce.toString('base64'),tag:cipher.getAuthTag().toString('base64'),ciphertext:ciphertext.toString('base64')}
 }finally{plaintext.fill(0)}
}
export function unsealCredentials(record,{ownerId,connectionId,keys}){
 let plaintext
 try{
  if(!record||record.format!==1||typeof record.keyVersion!=='string')throw new Error()
  const key=keys.get(record.keyVersion);validKey(key)
  const aad=identity(ownerId,connectionId,record.keyVersion)
  if(!['nonce','tag','ciphertext'].every(k=>typeof record[k]==='string'&&record[k].length<=12000))throw new Error()
  const nonce=Buffer.from(record.nonce,'base64'),tag=Buffer.from(record.tag,'base64'),ciphertext=Buffer.from(record.ciphertext,'base64')
  if(nonce.length!==12||tag.length!==16||!ciphertext.length)throw new Error()
  const cipher=createDecipheriv('aes-256-gcm',key,nonce);cipher.setAAD(aad);cipher.setAuthTag(tag)
  plaintext=Buffer.concat([cipher.update(ciphertext),cipher.final()])
  const credentials=JSON.parse(plaintext.toString('utf8'));validCredentials(credentials)
  return credentials
 }catch{throw new Error('Credentials unavailable; owner action required')}
 finally{plaintext?.fill(0)}
}
export function rotateCredentials(record,previousIdentity,nextIdentity){
 const credentials=unsealCredentials(record,previousIdentity)
 return sealCredentials(credentials,{...nextIdentity,ownerId:previousIdentity.ownerId,connectionId:previousIdentity.connectionId})
}
