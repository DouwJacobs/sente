import {describe,it,expect} from 'vitest'
import {cents,decimal} from './api'
describe('exact money inputs',()=>{
 it('converts without floating point rounding',()=>{expect(cents('-123.45')).toBe(-12345);expect(cents('0.29')).toBe(29);expect(cents('12.3')).toBe(1230);expect(cents('12')).toBe(1200)})
 it('rejects partial and over-precise inputs',()=>{for(const input of ['12abc','1.001','1e2','NaN','Infinity',''])expect(()=>cents(input)).toThrow()})
 it('formats signed cents back to edit values',()=>{expect(decimal(-29)).toBe('-0.29')})
})
