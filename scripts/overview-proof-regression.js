// Runs a real two-cent proof from Overview without navigating away.
(async () => {
  const check = (ok, message) => { if (!ok) throw new Error(message); };
  const wait = async predicate => { for (let i=0;i<300;i++) { if(predicate()) return; await new Promise(r=>setTimeout(r,100)); } throw new Error('Overview proof timeout'); };
  location.hash='overview';
  await wait(()=>document.querySelector('[data-overview-proof]'));
  const button=document.querySelector('[data-overview-proof]');
  check(button.tagName==='BUTTON' && !button.hasAttribute('href'),'Overview control is an action, not navigation');
  button.click();
  await wait(()=>document.querySelectorAll('#inline-proof .proof-step.complete').length===6 && !document.querySelector('#inline-proof [data-proof-run]').disabled);
  check(location.hash==='#overview','Completion stays on Overview');
  check(document.querySelector('#breadcrumb-view').textContent==='Overview','Overview remains selected');
  check(document.querySelectorAll('#inline-proof [data-proof-evidence]').length===4,'Results appear directly on Overview');
  await new Promise(r=>setTimeout(r,2300));
  check(document.querySelectorAll('#inline-proof .proof-step.complete').length===6,'Polling preserves completed evidence');
  document.querySelector('#inline-proof [data-proof-evidence]').click();
  check(document.querySelector('#detail-dialog').open,'Evidence can be inspected from Overview');
  document.querySelector('#detail-dialog').close();
  check(!document.querySelector('#campaign-assurance') && !document.querySelector('#integration-proof'),'No alternate page is rendered');
  return {passed:7,route:location.hash};
})();
