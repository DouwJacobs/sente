import {SyntheticEvent,useEffect,useState} from 'react'
import {Button,Field} from './ui'

export function MerchantAvatar({name,logo}:{name:string;logo?:string}){
 const[failed,setFailed]=useState(false),[contain,setContain]=useState(false)
 useEffect(()=>{setFailed(false);setContain(false)},[logo])
 const checkRatio=(img:HTMLImageElement)=>{
  if(img.naturalWidth&&img.naturalHeight){
   const ratio=img.naturalWidth/img.naturalHeight
   if(ratio>=1.2||ratio<=0.8)setContain(true)
  }
 }
 const initials=name.trim().split(/\s+/).slice(0,2).map(word=>word[0]).join('').toLocaleUpperCase()||'•'
 return <span className={'merchant-avatar'+(contain?' merchant-avatar-contain':'')} aria-hidden="true">{logo&&!failed?<img ref={(el:HTMLImageElement|null)=>{if(el?.complete)checkRatio(el)}} src={logo} alt="" onError={()=>setFailed(true)} onLoad={(e:SyntheticEvent<HTMLImageElement>)=>checkRatio(e.currentTarget)}/>:initials}</span>
}
export function MerchantLogoField({name,value,onChange,onBusy}:{name:string;value:string;onChange:(value:string)=>void;onBusy?:(busy:boolean)=>void}){
 const[error,setError]=useState(''),[loading,setLoading]=useState(false)
 const upload=async(file?:File)=>{
  if(!file)return
  setError('');setLoading(true);onBusy?.(true)
  try{
   if(!['image/png','image/jpeg'].includes(file.type))throw new Error('Choose a PNG or JPEG image.')
   if(file.size>5*1024*1024)throw new Error('Choose an image smaller than 5 MB.')
   const bitmap=await createImageBitmap(file)
   try{
    const scale=Math.min(1,256/Math.max(bitmap.width,bitmap.height)),canvas=document.createElement('canvas')
    canvas.width=Math.max(1,Math.round(bitmap.width*scale));canvas.height=Math.max(1,Math.round(bitmap.height*scale))
    canvas.getContext('2d')!.drawImage(bitmap,0,0,canvas.width,canvas.height)
    const data=canvas.toDataURL('image/png')
    if(data.length>174790)throw new Error('This image has too much detail. Choose a simpler logo.')
    onChange(data)
   }finally{bitmap.close()}
  }catch(e){setError(e instanceof Error?e.message:'Choose a valid image.')}
  finally{setLoading(false);onBusy?.(false)}
 }
 return <div className="merchant-logo-editor"><MerchantAvatar name={name} logo={value}/><div><Field label="Merchant logo (optional)" hint="PNG or JPEG. Images are resized and stored locally." serverError={error} validate={()=>error}><input type="file" accept="image/png,image/jpeg" disabled={loading} onChange={e=>void upload(e.target.files?.[0])}/></Field>{value&&<Button variant="quiet" onClick={()=>{onChange('');setError('')}}>Remove logo</Button>}</div></div>
}
