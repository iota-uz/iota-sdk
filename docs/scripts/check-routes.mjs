import fs from 'node:fs';import path from 'node:path';
let count=0;
function walk(dir){for(const item of fs.readdirSync(dir,{withFileTypes:true})){const file=path.join(dir,item.name);if(item.isDirectory())walk(file);else if(item.name.endsWith('.mdx')){const slug=path.relative('content',file).replace(/(^|\/)index\.mdx$/,'').replace(/\.mdx$/,'');const output=path.join('out',slug?slug+'.html':'index.html');if(!fs.existsSync(output))throw new Error('Missing published route: '+output);count++}}}
walk('content');console.log('Verified static output for '+count+' MDX routes');
