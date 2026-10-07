import fs from 'node:fs';
import path from 'node:path';
const forbidden=/^(?:react|react-dom|@types\/react|@types\/react-dom)$/;
const visited=new Set();
function walk(dir){
 if(!fs.existsSync(dir))return;
 const real=fs.realpathSync(dir);if(visited.has(real))return;visited.add(real);
 const manifest=path.join(dir,'package.json');if(fs.existsSync(manifest)){const pkg=JSON.parse(fs.readFileSync(manifest,'utf8'));if(forbidden.test(pkg.name??''))throw new Error('React package installed: '+pkg.name)}
 for(const item of fs.readdirSync(dir,{withFileTypes:true})){
 if(item.name==='.bin')continue;
 const child=path.join(dir,item.name);
 if(item.isDirectory()||item.isSymbolicLink())walk(child);
 }
}
walk('node_modules');console.log('Installed documentation dependency graph is React-free');
