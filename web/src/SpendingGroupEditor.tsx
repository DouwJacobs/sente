import {useState} from 'react'
import {api} from './api'
import {useTask,type PageProps,type Row} from './App'
import {Button,Field,Form,Modal} from './ui'

export function SpendingGroupEditor({group,notify,onClose,onDone}:{group:Row;notify:PageProps['notify'];onClose:()=>void;onDone:()=>void}){
 const [name,setName]=useState(group.name||''),[color,setColor]=useState(group.color||'blue'),[error,setError]=useState('')
 const {busy,run}=useTask(notify)
 return <Modal title={group.id?'Edit spending group':'Add spending group'} onClose={onClose}><Form onSubmit={async()=>{
  if(await run(()=>api('/spending-groups'+(group.id?'/'+group.id:''),group.id?'PUT':'POST',{name:name.trim(),color,version:group.version}),'Spending group saved',message=>{
   if(message==='Spending group already exists'){setError(message);return true}return false
  }))onDone()
 }}>
 <Field label="Name" serverError={error} validate={value=>{const length=Array.from(value.trim()).length;return length<2||length>80?'Use a name of 2–80 characters.':''}}><input autoFocus required value={name} onChange={e=>{setName(e.target.value);setError('')}}/></Field>
 <Field label="Color"><select value={color} onChange={e=>setColor(e.target.value)}>{['blue','amber','purple','orange','teal','slate','rose'].map(c=><option key={c} value={c}>{c[0].toUpperCase()+c.slice(1)}</option>)}</select></Field>
 <p className="muted">Groups and categories classify transactions independently. Budget limits apply to each chosen group and category.</p>
 <div className="editor-actions"><Button type="submit" variant="primary" loading={busy}>Save spending group</Button><Button onClick={onClose}>Cancel</Button></div>
 </Form></Modal>
}
