import {describe,it,expect} from 'vitest'
import {publicURLError,proxyError} from './NetworkSettings'
describe('network validation',()=>{
 it('requires an HTTP origin with no credentials or subpath',()=>{
  for(const value of ['https://finance.example.com','http://127.0.0.1:5173','https://[::1]:8080/'])expect(publicURLError(value)).toBe('')
  for(const value of ['ftp://example.com','https://example.com/path','https://user:secret@example.com','https://example.com?x=1','https://example.com#x',''])expect(publicURLError(value)).not.toBe('')
 })
 it('accepts exact proxy addresses/ranges and rejects universal trust',()=>{
  for(const value of ['', '127.0.0.1,::1','10.0.0.0/24,2001:db8::/64'])expect(proxyError(value)).toBe('')
  for(const value of ['*','proxy.example.com','0.0.0.0/0','::/0','999.0.0.1','10.0.0.0/33','::1/129','127.0.0.1,'])expect(proxyError(value)).not.toBe('')
 })
})
