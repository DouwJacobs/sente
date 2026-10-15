import {type Row} from '../../shared/types'
import {calendarSummary} from './fnbSchedule'

export function hasFNBSchedule(connection:Row) {
 return !!(connection.interval_hours||connection.daily_time||connection.schedule)
}

export function fnbScheduleLabel(connection:Row) {
 if(connection.schedule)return calendarSummary(connection.schedule)
 if(connection.daily_time)return `Daily at ${connection.daily_time} SAST`
 return connection.interval_hours?`Every ${connection.interval_hours} hours`:'Off — update manually'
}

export function fnbNextRefresh(connection:Row) {
 if(connection.state==='refreshing')return 'Update in progress'
 if(!hasFNBSchedule(connection))return 'Off — update manually'
 if(connection.state==='action_required')return 'Paused'
 if(connection.next_due)return new Date(connection.next_due*1000).toLocaleString('en-ZA',{timeZone:'Africa/Johannesburg'})+' SAST'
 return 'Not yet scheduled'
}

const reasons:Record<string,string> = {
 INTERRUPTED:'The previous bank update was interrupted when Sente stopped or restarted.',
 APPROVAL_REQUIRED:'The last sign-in needed approval in your FNB app.',
 SESSION_CONFLICT:'FNB reported another active banking session.',
 LOGOUT_REQUIRED:'Sente could not confirm that the previous FNB session signed out.',
 CONNECTOR_START_FAILED:'The bank connection browser could not start.',
 BROWSER_NOT_FOUND:'The bank connection browser is unavailable on the server.',
 CONNECTOR_TIMEOUT:'The last bank update took too long and timed out.',
 CONNECTOR_RESPONSE_INVALID:'The bank connection returned a response Sente could not verify.',
 KEY_UNAVAILABLE:'Sente cannot unlock the saved bank login details because its encryption key is unavailable.',
 LOGIN_LAYOUT_CHANGED:'Sente could not verify the FNB sign-in page.',
 ACCOUNT_LAYOUT_CHANGED:'Sente could not verify the account details returned by FNB.',
 BALANCE_LAYOUT_CHANGED:'Sente could not verify the balances returned by FNB.',
 TRANSACTION_LAYOUT_CHANGED:'Sente could not verify the transaction history returned by FNB.',
 TRANSACTION_ACCOUNT_MISMATCH:'The downloaded transactions did not match the expected account.',
 TRANSACTION_FEE_REVIEW_REQUIRED:'Sente could not verify the fees in the downloaded transactions.',
 TRANSACTION_ACCOUNT_UNSUPPORTED:'The transaction download contained an unsupported account type.',
 REFRESH_FAILED:'The last bank update did not finish successfully.',
}

export function fnbPauseExplanation(code:string|undefined) {
 return code&&Object.hasOwn(reasons,code)?reasons[code]:'The bank connection needs attention before automatic updates can continue.'
}

export function fnbRecoveryInstruction(code:string|undefined) {
 switch(code) {
 case 'KEY_UNAVAILABLE': return 'Ask the server administrator to restore the FNB encryption key, then refresh the connection.'
 case 'SESSION_CONFLICT': case 'LOGOUT_REQUIRED': return 'Sign out of any open FNB session, then choose Refresh now.'
 case 'BROWSER_NOT_FOUND': return 'Ask the server administrator to check the bank connection browser, then choose Refresh now.'
 default: return 'Choose Refresh now and approve the sign-in in your FNB app if asked.'
 }
}
