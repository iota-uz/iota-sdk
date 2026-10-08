import { readFile, writeFile } from 'node:fs/promises';
const source = new URL('../../../modules/bichat/presentation/web/src/rpc.generated.ts', import.meta.url);
const target = new URL('../src/data/rpc.generated.ts', import.meta.url);
const content = await readFile(source, 'utf8');
if (process.argv.includes('--check')) {
 if (content !== await readFile(target, 'utf8')) throw new Error('BiChat RPC schema drift: run pnpm --dir web/chat-ui rpc:sync');
} else await writeFile(target, content);
