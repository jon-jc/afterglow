// Read-only report checks. Delays one real response to test campaign-switch races.
(async()=>{
 const checks=[],check=(v,s)=>{if(!v)throw new Error(s);checks.push(s);};
 const wait=async f=>{for(let i=0;i<100;i++){if(f())return;await new Promise(r=>setTimeout(r,50));}throw new Error('Report timeout');};
 const original=window.fetch;
 try {
  location.hash='assurance';await wait(()=>document.querySelector('.assurance-verdict'));
  check(document.querySelector('#assurance-report .assurance-scope').textContent.includes('not a measured audience'),'Report distinguishes operations from audience outcomes');
  const select=document.querySelector('#assurance-campaign');
  let release,slow=true;
  window.fetch=async(url,opts)=>{const r=await original(url,opts);if(String(url).includes('/assurance')&&slow){slow=false;await new Promise(resolve=>release=resolve);}return r;};
  select.value='cmp-cascade';select.dispatchEvent(new Event('change',{bubbles:true}));await wait(()=>release);
  select.value='cmp-northstar';select.dispatchEvent(new Event('change',{bubbles:true}));
  await wait(()=>document.querySelector('.assurance-verdict'));
  const text=document.querySelector('#assurance-report').textContent;
  release();await new Promise(r=>setTimeout(r,150));
  check(document.querySelector('#assurance-report').textContent===text,'Older request cannot overwrite newly selected campaign');
  window.fetch=original;
  let blob;const create=URL.createObjectURL;
  try {URL.createObjectURL=b=>{blob=b;return create(b);};document.querySelector('[data-assurance-export]').click();}finally{URL.createObjectURL=create;}
  const report=JSON.parse(await blob.text());
  check(report.campaign.id==='cmp-northstar' && report.budget_consistent,'Export matches selected campaign and contains reconciliation result');
  check(report.screens.reduce((n,s)=>n+s.settled_micros,0)===report.campaign.spent_micros,'Per-screen settled totals match campaign balance');
  location.hash='overview';await wait(()=>!document.querySelector('#campaign-assurance'));
  location.hash='assurance';await wait(()=>document.querySelector('#campaign-assurance'));
  check(!document.querySelector('[data-assurance-export]').disabled,'Returning to a report preserves export availability');
  window.fetch=async(url,opts)=>{if(String(url).includes('/assurance'))throw new Error('Simulated report outage');return original(url,opts);};
  document.querySelector('[data-assurance-refresh]').click();await wait(()=>document.querySelector('#assurance-report .error-banner'));
  check(document.querySelector('[data-assurance-export]').disabled,'Failed refresh cannot export a stale report');
  window.fetch=original;document.querySelector('#assurance-report [data-assurance-refresh]').click();await wait(()=>document.querySelector('.assurance-verdict'));
  check(!document.querySelector('[data-assurance-export]').disabled,'Retry restores report and export');
  check(document.documentElement.scrollWidth<=innerWidth,'Report stays inside viewport');
  return {passed:checks.length,checks};
 }finally{window.fetch=original;}
})();
