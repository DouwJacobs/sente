import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
export default defineConfig({
 plugins:[react()],
 server:{
  strictPort:true,
  allowedHosts:['dev-finance.p-rex.co.za', ...(process.env.DEV_PUBLIC_URL ? [new URL(process.env.DEV_PUBLIC_URL).hostname] : [])],
  proxy:Object.fromEntries(['/api','/oauth','/.well-known'].map(path=>[path,process.env.DEV_API_TARGET || 'http://127.0.0.1:8080']))
 }
})
