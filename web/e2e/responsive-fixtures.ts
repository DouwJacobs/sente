import { type Page } from '@playwright/test';
import { reviewPermissions } from '../src/MCPPermissions';

export const groupName='Synthetic household essentials and exceptionally long spending group';
export const categoryName='Synthetic groceries and supplies with an exceptionally long category';
export const agentName='SyntheticAgent'+ 'LongName'.repeat(14);
export const accountName='SyntheticAccount'+ 'LongName'.repeat(14);
export const amount=98765432100;

export async function mockResponsiveBudget(page:Page) {
  let empty=false;
  await page.route('**/api/dashboard?**',async route=>{
   const response=await route.fetch(),body=await response.json();
   const privateScope=!!new URL(route.request().url()).searchParams.get('account');
   const category={id:1,kind:'expense',name:categoryName,target_cents:amount,spent_cents:amount+12345,
    total_target_cents:amount*2,total_spent_cents:amount*2+12345,pending_cents:0};
   await route.fulfill({json:{...body,has_targets:!privateScope,budget_cents:amount*2,spent_cents:empty?0:amount*3,
    remaining_cents:-amount,income_cents:empty?0:amount,pending_spend_cents:0,
    group_total:empty?0:3,categories:empty?[]:[category],category_total:empty?0:1,
    income_categories:empty?[]:[{id:4,name:categoryName,income_cents:amount}],income_category_total:empty?0:1,
    spending_groups:empty?[]:[
     {id:1,name:groupName,color:'teal',target_cents:amount,spent_cents:amount+12345,category_total:1,categories:[category]},
     {id:4,name:groupName+' second',color:'rose',target_cents:amount,spent_cents:-amount,category_total:1,categories:[{...category,spent_cents:-amount}]},
     {id:0,name:'No spending group',color:'slate',target_cents:0,spent_cents:12345,category_total:0,categories:[]},
    ]}});
  });
  await page.route('**/api/periods?**',async route=>{
   const response=await route.fetch(),body=await response.json();
   await route.fulfill({json:{...body,items:body.items.map((p:any)=>({...p,name:groupName+' period',target_total:amount}))}});
  });
  await page.route('**/api/periods/1/targets?**',async route=>{
   const response=await route.fetch(),body=await response.json();
   await route.fulfill({json:{...body,items:body.items.map((c:any)=>({...c,name:c.id===1?categoryName:c.name}))}});
  });
  // Response overrides exercise extreme labels, while the builder uses real fixture IDs/amounts.
  await page.route('**/api/periods/1/budget-groups?**',async route=>{
   const response=await route.fetch(),body=await response.json();
   await route.fulfill({json:{...body,items:body.items.map((g:any)=>({...g,name:groupName}))}});
  });
  return {setEmpty:()=>{empty=true;}};
}

export async function mockResponsiveSettings(page:Page) {
  await page.route('**/api/users?**',async route=>{
   const response=await route.fetch(),body=await response.json();
   await route.fulfill({json:{...body,items:body.items.map((u:any)=>({...u,username:'SyntheticUser'+'LongName'.repeat(12)}))}});
  });
  await page.route('**/api/accounts?**',async route=>{
   const response=await route.fetch(),body=await response.json();
   await route.fulfill({json:{...body,items:body.items.map((a:any)=>({...a,name:accountName,balance_cents:-amount}))}});
  });
  await page.route('**/api/configuration',async route=>{
   const response=await route.fetch(),body=await response.json();
   await route.fulfill({json:{...body,default_source:{...body.default_source,repository_url:'https://example.com/'+'long-path'.repeat(25)},
    sources:[{id:999,name:agentName,kind:'repository',repository_url:'https://example.com/'+'long-path'.repeat(25),ref:'main',path:'sente.json',version:1,last_revision:'long-revision'.repeat(20)}]}});
  });
  await page.route('**/api/fnb',route=>route.fulfill({json:{connection:null,accounts:[]}}));
  await page.route('**/api/mcp/settings',route=>route.fulfill({json:{endpoint:'https://example.com/'+'endpoint'.repeat(30),
   accounts:[{id:1,name:accountName,can_edit:true}],
   connections:[{id:'synthetic-layout-agent',name:agentName,permission_version:1,can_write:true,expires_at:2000000000,
    permissions:{...reviewPermissions(),preset:'custom',capabilities:{assign_missing:true,create_rule:true},constraints:{...reviewPermissions().constraints,account_ids:[1],selected_accounts:true}}}],
   proposals:[{id:'synthetic-layout-proposal',operation:'edit_transactions',token_name:agentName,status:'pending',expires_at:2000000000,
    payload:{change:{description:'updated-description'.repeat(40),amount_cents:-amount},before:{description:'long-description'.repeat(40),amount_cents:-amount},after:{description:'updated-description'.repeat(40),amount_cents:-amount}}}]}}));
}
