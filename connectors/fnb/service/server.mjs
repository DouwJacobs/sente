import {readFileSync} from 'node:fs'
import {createMockServer} from './mock-server.mjs'
// Provision the transport token outside source/backup storage. No token value is logged.
try{
 if(!process.env.FNB_TRANSPORT_TOKEN_FILE)throw new Error()
 const token=readFileSync(process.env.FNB_TRANSPORT_TOKEN_FILE,'utf8').trim()
 const port=Number(process.env.PORT||19090)
 if(!Number.isInteger(port)||port<1||port>65535)throw new Error()
 const server=createMockServer({mode:process.env.FNB_CONNECTOR_MODE,token})
 server.listen(port,'127.0.0.1')
 server.on('error',()=>{process.stderr.write('Connector could not start. Check owner-provisioned configuration.\n');process.exitCode=1})
 for(const signal of ['SIGTERM','SIGINT'])process.on(signal,()=>server.close())
}catch{process.stderr.write('Connector requires explicit mock mode and owner-provisioned configuration.\n');process.exitCode=1}
