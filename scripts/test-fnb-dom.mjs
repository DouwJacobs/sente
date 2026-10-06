// Synthetic bank-shaped pages only; no bank navigation or credentials.
import {chromium} from '../web/node_modules/playwright/index.mjs'
import assert from 'node:assert/strict'
import {readTransactionDOM,normalizeTransactionTable,clickSuccessfulDOM,readCreditIdentityDOM} from '../connectors/fnb/owner/transactions.mjs'
const browser=await chromium.launch({headless:true})
try{
 const page=await browser.newPage()
 const details='<div class="dlTitle">Account Number</div><div>00123456</div><div class="dlTitle">Type</div><div>Fusion</div>'
 const row=()=>'<div class="tableRow">'+['03 Oct 2026','Synthetic shop','synthetic-ref','0.00','-12.34','1,234.56'].map(value=>'<div class="tableCell"><div class="tableCellItem">'+value+'</div></div>').join('')+'</div>'
 const header='<div class="new-bank-header">'+['Date','Description','Reference','Service Fee','Amount','Balance'].map(value=>'<div><span>'+value+'</span></div>').join('')+'</div>'
 const tabs='<button id="successful" class="toggleButtonSelected"><span>Successful</span></button><button id="pending"><span>Pending</span></button>'
 await page.setContent(details+tabs+'<h2>Search Results</h2><div class="transaction-table">'+header+row()+'<div style="display:none">'+row()+'</div></div>')
 let result=await page.evaluate(readTransactionDOM)
 assert.deepEqual(result.headers,['Date','Description','Reference','Service Fee','Amount','Balance'])
 assert.equal(result.rows.length,1);assert.equal(result.posted_history,true);assert.equal(result.selected_successful,1)
 assert.equal(normalizeTransactionTable(result,'00123456','synthetic').transactions[0].amount_decimal,'-12.34')
 // Merely showing Successful and Pending labels must never import pending rows.
 await page.evaluate(()=>{document.querySelector('#successful').className='';document.querySelector('#pending').className='toggleButtonSelected'})
 result=await page.evaluate(readTransactionDOM);assert.equal(result.posted_history,false)
 assert.throws(()=>normalizeTransactionTable(result,'00123456','synthetic'),{code:'TRANSACTION_LAYOUT_CHANGED'})
 await page.evaluate(()=>{document.querySelector('#successful').onclick=()=>{document.querySelector('#successful').className='toggleButtonSelected';document.querySelector('#pending').className=''}})
 assert.equal(await page.evaluate(clickSuccessfulDOM),true)
 assert.equal((await page.evaluate(readTransactionDOM)).posted_history,true)
 await page.setContent(details+tabs+'<div class="transaction-table"><div class="tableRow">'+['Date','Description','Reference','Service Fee','Amount','Balance'].map(v=>'<div class="tableCell"><span class="tableCellItem">'+v+'</span></div>').join('')+'</div>'+row()+'</div>')
 result=await page.evaluate(readTransactionDOM);assert.equal(result.headers.length,6);assert.equal(result.rows.length,1)
 await page.setContent(details+tabs+'<div class="transaction-table">'+header.replace('Service Fee','Service<br>Fee')+row()+'</div>')
 result=await page.evaluate(readTransactionDOM);assert.equal(normalizeTransactionTable(result,'00123456','synthetic').transactions.length,1)
 // Old semantic headers work, and a second independent header is ambiguous.
 await page.setContent(details+tabs+'<div class="transaction-table">'+header+header+row()+'</div>')
 result=await page.evaluate(readTransactionDOM);assert.equal(result.headers.length,0)
 await page.setContent(details+tabs+'<div class="transaction-table">'+header.replace('Amount','Available amount')+row()+'</div>')
 result=await page.evaluate(readTransactionDOM);assert.equal(result.headers.length,0)
 await page.setContent(details+tabs+'<div class="transaction-table">'+header+Array.from({length:150},row).join('')+'</div>')
 result=await page.evaluate(readTransactionDOM);assert.equal(result.rows.length,150);assert.equal(normalizeTransactionTable(result,'00123456','synthetic').transactions.length,150)
 await page.setContent(details+'<label><input type="radio" name="state" checked>Successful</label><label><input type="radio" name="state">Pending</label><table><thead><tr>'+['Date','Description','Amount'].map(v=>'<th>'+v+'</th>').join('')+'</tr></thead><tbody><tr class="tableRow">'+['03 Oct 2026','Synthetic credit','1.23'].map(v=>'<td class="tableCell"><span class="tableCellItem">'+v+'</span></td>').join('')+'</tr></tbody></table>')
 result=await page.evaluate(readTransactionDOM);assert.equal(result.posted_history,true);assert.equal(result.headers.length,3)
 // Owner-observed four-column loan ledger; all values here are invented.
 const loanDetails=details.replace('Fusion','FNB Home Loan')
 const loanHeader='<div class="new-bank-header">'+['Effective Date','Description','Amount','Balance'].map(v=>'<div><span>'+v+'</span></div>').join('')+'</div>'
 const loanRows=[['03 Oct 2026','Synthetic adjustment','0.00','-1,000.00'],['02 Oct 2026','Synthetic service charge','-5.00','-1,000.00'],['01 Oct 2026','Synthetic interest','-10.00','-995.00'],['30 Sep 2026','Synthetic repayment','50.00','-985.00']]
 const renderLoan=()=>loanRows.map(values=>'<div class="tableRow">'+values.map(v=>'<div class="tableCell"><span class="tableCellItem">'+v+'</span></div>').join('')+'</div>').join('')
 await page.setContent(loanDetails+'<div class="transaction-table">'+loanHeader+renderLoan()+'</div>')
 result=await page.evaluate(readTransactionDOM)
 assert.deepEqual(result.headers,['Effective Date','Description','Amount','Balance']);assert.equal(result.rows.length,4);assert.equal(result.posted_history,true)
 const loan=normalizeTransactionTable(result,'00123456','synthetic')
 assert.deepEqual(loan.transactions.map(r=>r.amount_decimal),['0.00','-5.00','-10.00','50.00'])
 assert.ok(loan.transactions.every(r=>r.service_fee_decimal==='0.00'))
 // Loan-only evidence must not promote other account or pending layouts.
 await page.setContent(details+'<div class="transaction-table">'+loanHeader+renderLoan()+'</div>')
 result=await page.evaluate(readTransactionDOM);assert.equal(result.posted_history,false)
 assert.throws(()=>normalizeTransactionTable(result,'00123456','synthetic'),{code:'TRANSACTION_LAYOUT_CHANGED'})
 await page.setContent(loanDetails+'<h2>Pending transactions</h2><div class="transaction-table">'+loanHeader+renderLoan()+'</div>')
 result=await page.evaluate(readTransactionDOM);assert.equal(result.posted_history,false)
 await page.setContent(loanDetails+tabs+'<div class="transaction-table">'+loanHeader+renderLoan()+'</div>')
 await page.evaluate(()=>{document.querySelector('#successful').className='';document.querySelector('#pending').className='toggleButtonSelected'})
 result=await page.evaluate(readTransactionDOM);assert.equal(result.posted_history,false)
 await page.setContent(loanDetails+'<div class="transaction-table">'+loanHeader.replace('Balance','Available balance')+renderLoan()+'</div>')
 result=await page.evaluate(readTransactionDOM);assert.equal(result.posted_history,false)
 assert.throws(()=>normalizeTransactionTable(result,'00123456','synthetic'),{code:'TRANSACTION_LAYOUT_CHANGED'})
 await page.setContent(details.replace('Fusion','FNB Premier Credit Card'))
 let identity=await page.evaluate(readCreditIdentityDOM)
 assert.deepEqual(identity,{bank_id:'00123456',credit:true,currency:''})
 await page.setContent(details.replace('Fusion','FNB Premier Credit Card')+'<div class="dlTitle">Account Number</div><div>99887766</div>')
 assert.equal(await page.evaluate(readCreditIdentityDOM),null)
 const masked='123456******7890'
 await page.setContent(details.replace('00123456',masked).replace('Fusion','FNB Premier Credit Card')+tabs+'<div class="transaction-table">'+header+row()+'</div>')
 result=await page.evaluate(readTransactionDOM)
 assert.equal(normalizeTransactionTable(result,masked,'synthetic').bank_id,masked)
 assert.equal((await page.evaluate(readCreditIdentityDOM)).bank_id,masked)
 console.log('PASS: masked credit identity/table; unique credit detail identity; home-loan effective-date ledger and pending/unknown-layout rejection; synthetic Chromium header mapping, selected Successful/Pending, explicit selection, hidden rows, ambiguous/unknown headers, 150 rows and checked radio/semantic headers')
}finally{await browser.close()}
