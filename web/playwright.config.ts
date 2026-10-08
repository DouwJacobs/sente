import {defineConfig} from '@playwright/test'
const port=Number(process.env.E2E_TEST_PORT||18080)
const baseURL=`http://127.0.0.1:${port}`
export default defineConfig({
 testDir:'./e2e',timeout:60000,workers:1,fullyParallel:false,
 use:{baseURL,trace:'retain-on-failure',screenshot:'only-on-failure',headless:true},
 webServer:[
  {command:`E2E_PORT=${port} python3 ../scripts/e2e-server.py`,url:baseURL+'/api/health',timeout:240000,reuseExistingServer:false},
  {command:`E2E_EMPTY=1 E2E_PORT=${port+1} python3 ../scripts/e2e-server.py`,url:`http://127.0.0.1:${port+1}/api/health`,timeout:240000,reuseExistingServer:false},
  {command:`E2E_EMPTY=1 E2E_PORT=${port+2} python3 ../scripts/e2e-server.py`,url:`http://127.0.0.1:${port+2}/api/health`,timeout:240000,reuseExistingServer:false}
 ]
})
