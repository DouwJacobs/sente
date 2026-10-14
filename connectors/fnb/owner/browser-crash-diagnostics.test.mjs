import test from 'node:test';
import assert from 'node:assert/strict';
import {classifyBrowserStderr} from './browser-crash-diagnostics.mjs';

test('recognizes a bounded sandbox syscall without exporting browser text',()=>{
 assert.deepEqual(classifyBrowserStderr('seccomp-bpf failure in syscall 0x148; https://secret.invalid/?password=private'),{browser_sandbox_failure:1,browser_sandbox_syscall:328});
 assert.deepEqual(classifyBrowserStderr('seccomp-bpf failure in syscall 9999999999'),{browser_sandbox_failure:1});
});
test('recognizes fixed crash categories and drops unrelated sensitive strings',()=>{
 assert.deepEqual(classifyBrowserStderr('GPU process exited unexpectedly; WebGL2 blocklisted; No space left on device; Out of memory'),{browser_gpu_failure:1,browser_webgl_unavailable:1,browser_storage_full:1,browser_memory_failure:1});
 assert.deepEqual(classifyBrowserStderr('username=private password=private https://secret.invalid'),{});
});
