import {test,expect} from './fixtures';

test('sync receipts stay separate from local save status and conflicts are actionable',async({page,appURL})=>{
 let status='Saved on Mac · pending sync';let action='';
 await page.route('**/sync',async route=>{
  if(route.request().method()==='POST'){
   action=route.request().postData()||'';
   await route.fulfill({json:{queued:true}});return;
  }
  await route.fulfill({json:{enabled:true,message:'Connected · local files saved',files:{'notes.txt':status},lastSync:'2026-09-05T20:00:00Z'}});
 });
 await page.goto(appURL+'/?file=notes.txt');
 const panel=page.locator('#sync-status-panel');
 await expect(panel).toContainText('Saved on Mac · pending sync');
 await expect(page.locator('#save-status')).toHaveText('Saved');
 status='Uploaded · iPhone pending';
 await expect(panel).toContainText(status);
 status='Synced with iPhone';
 await expect(panel).toContainText(status);
 status='Conflict · both versions kept';
 await expect(panel.getByRole('button',{name:'Keep both versions'})).toBeVisible();
 await panel.getByRole('button',{name:'Keep both versions'}).click();
 await expect.poll(()=>action).toBe('keepBoth=notes.txt');
});

test('ordinary workspaces do not show sync controls',async({page,appURL})=>{
 await page.goto(appURL+'/?file=notes.txt');
 await expect(page.locator('#sync-status-panel')).toBeHidden();
});
