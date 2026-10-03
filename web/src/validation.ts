import {cents} from './api'
export type Validator=(value:string)=>string
export const passwordError:Validator=value=>{
 if(!value)return 'Enter a password.'
 const length=new TextEncoder().encode(value).length
 if(length<12)return 'Use at least 12 characters.'
 if(length>72)return 'This password is too long. Try a shorter one.'
 return ''
}
export const usernameError:Validator=value=>{
 const length=new TextEncoder().encode(value.trim()).length
 if(!length)return 'Enter a username.'
 if(length<2)return 'Use at least 2 characters.'
 if(length>80)return 'This username is too long.'
 return ''
}
export const moneyError=(value:string,nonnegative=false)=>{
 try{const amount=cents(value);return nonnegative&&amount<0?'Use zero or a positive amount.':''}
 catch(e){return (e as Error).message}
}
