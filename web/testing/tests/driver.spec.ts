import { test, expect } from '@playwright/test'
import { createProcessEnvironmentDriver } from '../src/index.js'

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
