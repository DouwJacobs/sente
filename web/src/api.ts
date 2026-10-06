let csrf = ''
export function setCSRF(value: string) { csrf = value }
export async function api<T = any>(path: string, method = 'GET', body?: unknown): Promise<T> {
 const form = body instanceof FormData
 const response = await fetch('/api' + path, {
  method, credentials: 'same-origin',
  headers: { ...(form || body === undefined ? {} : {'Content-Type':'application/json'}), ...(method === 'GET' ? {} : {'X-CSRF-Token':csrf}) },
  body: body === undefined ? undefined : form ? body : JSON.stringify(body)
 })
 const result = await response.json().catch(() => ({error:'The server returned an invalid response'}))
 if (!response.ok) { if(response.status === 401 && path !== '/login' && (path !== '/me' || csrf !== '')) window.dispatchEvent(new Event('session-expired')); throw new Error(result.error || 'Request failed') }
 return result
}
export const money = (cents: number) => new Intl.NumberFormat('en-ZA',{style:'currency',currency:'ZAR'}).format(cents / 100)
export const decimal = (cents: number) => (cents / 100).toFixed(2)
export function cents(value: string): number {
 if (!/^[+-]?\d+(\.\d{1,2})?$/.test(value.trim())) throw new Error('Use an amount with at most two decimal places')
 const negative = value.trim().startsWith('-')
 const [whole, fraction = ''] = value.trim().replace(/^[+-]/,'').split('.')
 const amount = Number(whole) * 100 + Number(fraction.padEnd(2,'0'))
 if (!Number.isSafeInteger(amount) || amount > 900000000000000) throw new Error('Amount is too large')
 return negative ? -amount : amount
}

export async function download(path:string,filename:string){
 const response=await fetch('/api'+path,{credentials:'same-origin'})
 if(!response.ok){const result=await response.json().catch(()=>({}));throw new Error(result.error||'Export failed')}
 const url=URL.createObjectURL(await response.blob()),link=document.createElement('a');link.href=url;link.download=filename;link.click();setTimeout(()=>URL.revokeObjectURL(url),1000)
}
