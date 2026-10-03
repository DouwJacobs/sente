import {describe,it,expect} from 'vitest'
import {parseAccountDiscovery,accountNumberError,accountNameError} from './account-discovery'
const report=(accounts:any[])=>JSON.stringify({schema_version:1,source:'fnb-account-discovery',accounts})
describe('account-only discovery boundary',()=>{
 it('preserves leading zeros and masked numbers for explicit correction',()=>{
  expect(parseAccountDiscovery(report([{name:' Test ',bank_id:'001234'}]))).toEqual([{name:'Test',bank_id:'001234'}])
  expect(parseAccountDiscovery(report([{name:'Masked',bank_id:'******1234'}]))[0].bank_id).toBe('******1234')
  expect(accountNumberError('******1234')).not.toBe('');expect(accountNumberError('001234')).toBe('')
 })
 it('rejects credentials, balances, transactions, duplicates and wrong report types',()=>{
  for(const extra of [{password:'synthetic'},{balance_cents:1},{transactions:[]}])expect(()=>parseAccountDiscovery(report([{name:'Test',bank_id:'123',...extra}]))).toThrow()
  expect(()=>parseAccountDiscovery(report([{name:'A',bank_id:'123'},{name:'B',bank_id:'123'}]))).toThrow()
  expect(()=>parseAccountDiscovery(report([]))).toThrow()
  expect(()=>parseAccountDiscovery('{"source":"other"}')).toThrow()
  expect(accountNameError('   ')).not.toBe('')
 })
})
