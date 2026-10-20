import {test} from 'node:test';
import assert from 'node:assert/strict';
import {webcrypto} from 'node:crypto';
test('no identity or events before consent; withdrawal failure stops tracking and can retry',async()=>{
 const store=new Map(),calls=[];let failErase=false;
 globalThis.localStorage={getItem:k=>store.get(k)||null,setItem:(k,v)=>store.set(k,v),removeItem:k=>store.delete(k)};
 if(!globalThis.crypto)globalThis.crypto=webcrypto;
 globalThis.window={go:{main:{Desktop:{SetUsageIdentity:async id=>calls.push(['identity',id]),TrackUsage:async e=>calls.push(['event',e]),WithdrawUsage:async id=>{calls.push(['erase',id]);if(failErase)throw Error('offline')}}}}};
 const u=await import('./usage.js');await u.trackUsage('app_open');assert.equal(calls.length,0);assert.equal(store.size,0);
 await u.acceptUsage();assert.equal(u.participating(),true);assert.equal(calls[0][0],'identity');assert.deepEqual(calls.slice(1).map(c=>c[1]),['app_open','page_stats']);
 failErase=true;await assert.rejects(u.withdrawUsage());assert.equal(u.participating(),false);const n=calls.length;await u.trackUsage('page_stats');assert.equal(calls.length,n);assert.equal(u.hasConsentRecord(),true);
 failErase=false;await u.withdrawUsage();assert.equal(u.hasConsentRecord(),false);assert.equal(store.size,0);
});
