import {useEffect,useRef,useId,useState,Children,cloneElement,isValidElement,type ReactNode,type ButtonHTMLAttributes,type ReactElement,type FormHTMLAttributes,type ChangeEvent,type FocusEvent,type InvalidEvent,type Ref} from 'react'
import {X} from 'lucide-react'
import {createPortal} from 'react-dom'
export function Spinner(){return <span className="spinner" aria-hidden="true"/>}
export function Loading({children='Loading'}:{children?:ReactNode}){return <span className="loading-status" role="status"><Spinner/>{children}</span>}
export function Button({children,variant='secondary',loading=false,type='button',...props}:ButtonHTMLAttributes<HTMLButtonElement>&{variant?:'primary'|'secondary'|'quiet'|'danger';loading?:boolean}){return <button {...props} type={type} disabled={props.disabled||loading} aria-busy={loading||undefined} className={'button '+variant+' '+(props.className||'')}>{loading&&<Spinner/>}{children}</button>}
export function PageHeader({title,description,workspace,loading=false}:{title:string;description:string;workspace:string;loading?:boolean}){
 return <div className="page-head">
  <div><p className="eyebrow">{workspace} finances</p><h1 id="page-title">{title}</h1><p className="muted">{description}</p></div>
  <div className="page-head-meta">{loading&&<Loading>Refreshing workspace</Loading>}<Badge>ZAR</Badge></div>
 </div>
}
export function Pagination({page,total,size=20,loading,onChange,range=false}:{page:number;total:number;size?:number;loading?:boolean;range?:boolean;onChange:(page:number)=>void}){
 const pages=Math.max(1,Math.ceil(total/size))
 const label=range?`${total?page*size+1:0}–${Math.min((page+1)*size,total)} of ${total} transactions`:`Page ${page+1} of ${pages} · ${total} items`
 return <nav className="pagination" aria-label="List pages">
  <span role="status">{label}</span>
  <div><Button disabled={page===0||loading} onClick={()=>onChange(page-1)}>Previous</Button><Button disabled={page+1>=pages||loading} onClick={()=>onChange(page+1)}>Next</Button></div>
 </nav>
}

type Control=HTMLInputElement|HTMLSelectElement|HTMLTextAreaElement
function controlError(control:Control,label:string,validate?:((value:string)=>string)){
 control.setCustomValidity('')
 const value=control.value
 if(control.required&&!value.trim())return control.tagName==='SELECT'?'Choose '+label.toLowerCase()+'.':'Enter '+label.toLowerCase()+'.'
 const result=validate?.(value)
 if(result)return result
 const minimum=control.getAttribute('minlength'),maximum=control.getAttribute('maxlength')
 if(minimum&&value&&value.length<Number(minimum))return 'Use at least '+minimum+' characters.'
 if(maximum&&value.length>Number(maximum))return 'Use no more than '+maximum+' characters.'
 if(control.validity.badInput)return 'Enter a valid value.'
 if(control.validity.typeMismatch)return 'Enter a valid '+(control.getAttribute('type')||'value')+'.'
 if(control.validity.patternMismatch)return 'Use the requested format.'
 if(control.validity.rangeUnderflow)return 'Use '+control.getAttribute('min')+' or more.'
 if(control.validity.rangeOverflow)return 'Use '+control.getAttribute('max')+' or less.'
 if(control.validity.stepMismatch)return 'Enter a valid increment.'
 return ''
}
export function validateFields(container:HTMLElement){
 const controls=Array.from(container.querySelectorAll<Control>('input,select,textarea')).filter(c=>!c.matches(':disabled'))
 const invalid=controls.filter(c=>!c.checkValidity())
 invalid[0]?.focus()
 return !invalid.length
}
export function Form({onSubmit,children,...props}:FormHTMLAttributes<HTMLFormElement>&{ref?:Ref<HTMLFormElement>}){
 return <form {...props} noValidate onSubmit={e=>{e.preventDefault();if(validateFields(e.currentTarget))onSubmit?.(e)}}>{children}</form>
}
export function Field({label,children,hint,validate,serverError}: {label:string;children:ReactNode;hint?:string;validate?:((value:string)=>string);serverError?:string}){
 const id=useId(),control=useRef<Control|null>(null)
 const[touched,setTouched]=useState(false),[error,setError]=useState('')
 const check=(element:Control)=>{const message=serverError||controlError(element,label,validate);element.setCustomValidity(message);setError(message)}
 useEffect(()=>{if(control.current)check(control.current)},[children,validate,serverError,label])
 const visible=touched||!!serverError
 const message=visible?error:''
 return <div className="field"><label htmlFor={id}>{label}</label>{Children.map(children,child=>{
  if(!isValidElement(child)||typeof child.type!=='string'||!['input','select','textarea'].includes(child.type))return child
  const element=child as ReactElement<Record<string,any>>
  return cloneElement(element,{
   id,ref:(el:Control|null)=>{control.current=el;const ref=element.props.ref;if(typeof ref==='function')ref(el);else if(ref)ref.current=el},
   'aria-invalid':message?true:undefined,
   'aria-describedby':[element.props['aria-describedby'],hint?id+'-hint':'',message?id+'-error':''].filter(Boolean).join(' ')||undefined,
   onChange:(event:ChangeEvent<Control>)=>{element.props.onChange?.(event);check(event.currentTarget)},
   onBlur:(event:FocusEvent<Control>)=>{element.props.onBlur?.(event);setTouched(true);check(event.currentTarget)},
   onInvalid:(event:InvalidEvent<Control>)=>{event.preventDefault();element.props.onInvalid?.(event);setTouched(true);check(event.currentTarget)}
  })
 })}{hint&&<small id={id+'-hint'}>{hint}</small>}{message?<small className="field-error" id={id+'-error'} role="alert">{message}</small>:<span className="field-error-space" aria-hidden="true"/>}</div>
}
export function Empty({title,children}: {title:string;children?:ReactNode}){return <div className="empty"><h3>{title}</h3>{children&&<p>{children}</p>}</div>}
export function Badge({children,tone='neutral'}: {children:ReactNode;tone?:'neutral'|'pending'|'good'|'bad'}){return <span className={'badge '+tone}>{children}</span>}
export function Modal({title,children,onClose}: {title:string;children:ReactNode;onClose:()=>void}){
 const ref=useRef<HTMLDialogElement>(null)
 useEffect(()=>{ref.current?.showModal();return()=>{ref.current?.close()}},[])
 return <dialog ref={ref} className="modal" aria-label={title} onCancel={event=>{event.preventDefault();event.stopPropagation();onClose()}}><div className="modal-head"><h2>{title}</h2><Button variant="quiet" aria-label="Close" onClick={onClose}><X size={20}/></Button></div><div className="modal-body">{children}</div></dialog>
}
export function Toast({message,error,onDismiss}:{message:string;error:boolean;onDismiss:()=>void}){
 const ref=useRef<HTMLDivElement>(null),[paused,setPaused]=useState(false)
 const [host,setHost]=useState<Element>(document.body)
 useEffect(()=>{
  const update=()=>setHost(document.querySelector('dialog[open]')||document.body)
  update();const observer=new MutationObserver(update)
  observer.observe(document.body,{subtree:true,childList:true,attributes:true,attributeFilter:['open']})
  return()=>observer.disconnect()
 },[])
 useEffect(()=>{ref.current?.showPopover()},[host])
 useEffect(()=>{if(paused)return;const timer=window.setTimeout(onDismiss,error?12000:7000);return()=>clearTimeout(timer)},[paused,message,error,onDismiss])
 return createPortal(<div ref={ref} popover="manual" className={'toast '+(error?'error':'')} role={error?'alert':'status'} aria-atomic="true" onMouseEnter={()=>setPaused(true)} onMouseLeave={()=>setPaused(false)} onFocusCapture={()=>setPaused(true)} onBlurCapture={event=>{if(!event.currentTarget.contains(event.relatedTarget as Node))setPaused(false)}}><span>{message}</span><Button variant="quiet" aria-label="Dismiss message" onClick={onDismiss}><X size={18}/></Button></div>,host)
}

export function Tabs({id,label,items,value,onChange}:{id:string;label:string;items:{id:string;label:string}[];value:string;onChange:(value:string)=>void}){
 return <div className="settings-tabs" role="tablist" aria-label={label}>{items.map((item,index)=><Button key={item.id} id={id+'-tab-'+item.id} role="tab" aria-selected={value===item.id} aria-controls={id+'-panel-'+item.id} tabIndex={value===item.id?0:-1} variant={value===item.id?'primary':'quiet'} onClick={()=>onChange(item.id)} onKeyDown={event=>{
  const next=event.key==='ArrowRight'?(index+1)%items.length:event.key==='ArrowLeft'?(index+items.length-1)%items.length:event.key==='Home'?0:event.key==='End'?items.length-1:-1
  if(next<0)return
  event.preventDefault();onChange(items[next].id)
  document.getElementById(id+'-tab-'+items[next].id)?.focus()
 }}>{item.label}</Button>)}</div>
}
