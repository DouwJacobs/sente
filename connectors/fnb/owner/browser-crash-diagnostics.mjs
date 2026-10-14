// Fixed crash counters for the worker and owner-run diagnostic probe. Browser stderr is private and never exported.
export function classifyBrowserStderr(text) {
 const counts={};
 if(/seccomp-bpf failure/i.test(text)) {
  counts.browser_sandbox_failure=1;
  const syscall=text.match(/seccomp-bpf failure[^\n]*syscall(?: number)?[ :=]+(0x[0-9a-f]+|[0-9]+)/i);
  if(syscall){const value=Number(syscall[1]);if(Number.isSafeInteger(value)&&value>=0&&value<=10000)counts.browser_sandbox_syscall=value;}
 }
 if(/GPU process exited unexpectedly|GPU process isn't usable|GPU process crashed/i.test(text))counts.browser_gpu_failure=1;
 if(/WebGL(?:2)? blocklisted/i.test(text))counts.browser_webgl_unavailable=1;
 if(/No space left on device/i.test(text))counts.browser_storage_full=1;
 if(/Out of memory|OOM killer|Cannot allocate memory/i.test(text))counts.browser_memory_failure=1;
 if(/Received signal 11|SIGSEGV/.test(text))counts.browser_segv=1;
 if(/Received signal 31|SIGSYS/.test(text))counts.browser_sigsys=1;
 if(/Received signal 6\b|SIGABRT/.test(text))counts.browser_aborted=1;
 return counts;
}

export function observeBrowserStderr(browser,counts) {
 const stream=browser.process()?.stderr;
 let tail='';
 const receive=chunk=>{
  tail=(tail+chunk.toString()).slice(-4096);
  Object.assign(counts,classifyBrowserStderr(tail));
 };
 stream?.on('data',receive);
 return ()=>{stream?.off('data',receive);tail='';};
}
