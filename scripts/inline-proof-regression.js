// Creates a two-cent proof in the selected campaign through the real Go API.
(async()=>{
 const check=(v,s)=>{if(!v)throw new Error(s);};
 const wait=async f=>{for(let i=0;i<300;i++){if(f())return;await new Promise(r=>setTimeout(r,100));}throw new Error('Inline proof timeout');};
 location.hash='assurance';await wait(()=>document.querySelector('#assurance-campaign') && document.querySelector('#inline-proof'));
 const select=document.querySelector('#assurance-campaign');select.value='cmp-northstar';select.dispatchEvent(new Event('change',{bubbles:true}));
 await wait(()=>document.querySelector('.assurance-verdict'));
 const before=await (await fetch('/api/v1/campaigns/cmp-northstar/assurance')).json();
 document.querySelector('#inline-proof [data-proof-run]').click();
 check(select.disabled,'Campaign selection locked during run');
 await wait(()=>document.querySelectorAll('#inline-proof .proof-step.complete').length===6 && !document.querySelector('#inline-proof [data-proof-run]').disabled);
 await wait(()=>document.querySelector('.assurance-verdict'));
 const saved=JSON.parse(sessionStorage.getItem('afterglow.integration-proof.v1'));
 check(location.hash==='#assurance','Run stayed on assurance page');
 check(saved.plan.campaign==='cmp-northstar','Run uses selected campaign');
 check(document.querySelectorAll('#inline-proof [data-proof-evidence]').length===4,'Receipt evidence appears inline');
 check(!select.disabled,'Campaign selector re-enabled');
 const after=await (await fetch('/api/v1/campaigns/cmp-northstar/assurance')).json();
 check(after.campaign.spent_micros===before.campaign.spent_micros+20000,'Selected campaign settled exactly two cents');
 let exported;const original=URL.createObjectURL;
 try{URL.createObjectURL=b=>{exported=b;return original(b);};document.querySelector('[data-assurance-export]').click();}finally{URL.createObjectURL=original;}
 const report=JSON.parse(await exported.text());
 check(report.campaign.spent_micros===after.campaign.spent_micros,'Assurance report refreshed after completion');
 return {passed:7,campaign:saved.plan.campaign,route:location.hash};
})();
