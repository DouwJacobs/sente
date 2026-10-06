// Keep a contiguous description fragment so the proposed contains rule can match.
// The reviewer confirms the merchant text; numeric references are never learned.
export function proposeRulePattern(description:string){
 const fragments=description.split(/\b\d[\d./:-]{3,}\b/g).map(text=>text.replace(/[\s*#:/-]+$/,'').trim())
 const text=fragments.find(fragment=>fragment.length>=2)||description.trim()
 let pattern=''
 for(const character of text){if(new TextEncoder().encode(pattern+character).length>200)break;pattern+=character}
 return pattern.trim()
}
