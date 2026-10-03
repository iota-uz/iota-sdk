import { test, expect } from '@playwright/test'
import { createProcessEnvironmentDriver, DriverError } from '../src/index.js'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'

// Falsely green if the process exits on stdin EOF or if only an unowned PID is killed.
test('close escalates an owned driver that ignores EOF and SIGINT within a bound', async () => {
  const directory = await mkdtemp(path.join(tmpdir(), 'sdk-driver-close-'))
  const marker = path.join(directory, 'sigint')
  const driver = createProcessEnvironmentDriver({
    command: process.execPath,
    args: ['--input-type=module', '-e', `
      import { createInterface } from 'node:readline';
      import { writeFileSync } from 'node:fs';
      process.on('SIGINT',()=>writeFileSync(${JSON.stringify(marker)},'received'));
      setInterval(()=>{},1000);
      createInterface({input:process.stdin}).on('line',line=>{
        const r=JSON.parse(line);
        console.log(JSON.stringify({id:r.id,descriptor:{environmentId:String(process.pid),baseURL:'http://fixture.test',artifactDirectory:'/tmp/owned'}}));
      });
    `], spec: () => ({}), closeTimeoutMs: 100, killTimeoutMs: 100,
  })
  const environment = await driver.lifecycle.start(0)
  let timer: ReturnType<typeof setTimeout> | undefined
  try {
    const result = await Promise.race([
      driver.close().catch(error => error),
      new Promise(resolve => { timer = setTimeout(() => resolve('unbounded close'), 1500) }),
    ])
    expect(result).toMatchObject({ code: 'close_timeout', operation: 'close' })
    expect(await readFile(marker, 'utf8')).toBe('received')
    await expect.poll(() => {
      try { process.kill(Number(environment.environmentId), 0); return 'alive' } catch { return 'gone' }
    }).toBe('gone')
  } finally {
    if (timer) clearTimeout(timer)
    try { process.kill(Number(environment.environmentId), 'SIGKILL') } catch {}
    await driver.close().catch(() => {})
    await rm(directory, { recursive: true, force: true })
  }
})

// Falsely green if errors are reduced to message text and partial startup ownership is lost.
test('driver errors retain startup ownership, artifacts and cleanup causes', async () => {
  const driver = createProcessEnvironmentDriver({
    command: process.execPath,
    args: ['--input-type=module', '-e', `
      import{createInterface}from'node:readline';
      createInterface({input:process.stdin}).on('line',line=>{
        const r=JSON.parse(line);
        console.log(JSON.stringify({id:r.id,descriptor:{environmentId:'partial',artifactDirectory:'/tmp/partial'},error:{code:'startup_failed',message:'startup',causes:[{code:'cleanup_failed',message:'drop'}]}}));
      });
    `], spec: () => ({}),
  })
  try {
    const error = await driver.lifecycle.start(0).catch(error => error)
    expect(error).toBeInstanceOf(DriverError)
    expect(error).toMatchObject({ code: 'startup_failed', operation: 'start', environmentId: 'partial', artifactDirectory: '/tmp/partial', causes: [{ code: 'cleanup_failed' }] })
  } finally { await driver.close() }
})

// Characterisation: falsely green if the child never parses stdin or no stop is sent.
test('correlates a persistent subprocess and surfaces driver errors', async () => {
  const driver = createProcessEnvironmentDriver({
    command: process.execPath,
    args: ['--input-type=module', '-e', `
      import { createInterface } from 'node:readline';
      let stopped = false;
      const lines = createInterface({input:process.stdin});
      lines.on('line', line => {
        const r = JSON.parse(line);
        const response = r.operation === 'start'
          ? {id:r.id, descriptor:{environmentId:'env-'+r.spec.worker,baseURL:'http://127.0.0.1:1234',artifactDirectory:'/tmp/owned'}}
          : stopped ? {id:r.id,error:{code:'missing',message:'already stopped'}} : {id:r.id};
        if(r.operation === 'stop') stopped = true;
        console.log(JSON.stringify(response));
      });
    `],
    spec: worker => ({ worker }),
  })
  try {
    const environment = await driver.lifecycle.start(7)
    expect(environment.runId).toBe('env-7')
    expect(environment.artifactDir).toBe('/tmp/owned')
    await driver.lifecycle.stop(environment)
    await expect(driver.lifecycle.stop(environment)).rejects.toThrow('missing: already stopped')
  } finally { await driver.close() }
})
// Characterisation: falsely green if a pending start is never awaited.
test('unexpected driver exit rejects pending start', async () => {
  const driver = createProcessEnvironmentDriver({ command: process.execPath, args: ['-e', 'process.exit(2)'], spec: () => ({}) })
  await expect(driver.lifecycle.start(0)).rejects.toThrow('exited')
  await expect(driver.close()).rejects.toThrow('exited')
})

// Regression: falsely green if malformed protocol data merely waits for the normal request deadline.
test('malformed response envelopes reject pending requests without crashing the host', async () => {
  for (const value of ['null', '17', '{"id":q.id,"error":{"code":"bad","message":"bad","causes":{}}}']) {
    const driver = createProcessEnvironmentDriver({ command: process.execPath, args: ['-e', `require('node:readline').createInterface({input:process.stdin}).on('line',line=>{const q=JSON.parse(line);console.log(JSON.stringify(${value}))})`], spec: () => ({}), timeoutMs: 2000, closeTimeoutMs: 100, killTimeoutMs: 100 })
    try { await expect(driver.lifecycle.start(0)).rejects.toMatchObject({ code: 'invalid_response', operation: 'start' }) }
    finally { await driver.close() }
  }
})

// Regression: falsely green if the child exits before a real pipe write observes its closed stdin.
test('closed child stdin rejects a large request without crashing the host', async () => {
  const directory = await mkdtemp(path.join(tmpdir(), 'sdk-driver-epipe-'))
  const marker = path.join(directory, 'pid')
  const driver = createProcessEnvironmentDriver({
    command: process.execPath,
    args: ['-e', `const fs=require('node:fs');fs.writeFileSync(${JSON.stringify(marker)},String(process.pid));fs.closeSync(0);setInterval(()=>{},1000)`],
    spec: () => ({ payload: 'x'.repeat(8 * 1024 * 1024) }),
    timeoutMs: 2000, closeTimeoutMs: 100, killTimeoutMs: 100,
  })
  let pid: number | undefined
  try {
    await expect(driver.lifecycle.start(0)).rejects.toMatchObject({ code: 'write_failed', operation: 'start' })
    pid = Number(await readFile(marker, 'utf8'))
    await driver.close().catch(() => {})
    await expect.poll(() => {
      try { process.kill(pid!, 0); return 'alive' } catch { return 'gone' }
    }).toBe('gone')
  } finally {
    pid ??= await readFile(marker, 'utf8').then(Number).catch(() => undefined)
    if (pid) { try { process.kill(pid, 'SIGKILL') } catch (error) { if ((error as NodeJS.ErrnoException).code !== 'ESRCH') throw error } }
    await driver.close().catch(() => {})
    await rm(directory, { recursive: true, force: true })
  }
})
