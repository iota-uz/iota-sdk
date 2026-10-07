export function docsMarkdown({base}) {
 return (tree,file)=>{
  const locale=/content[\\/](en|ru)[\\/]/.exec(file.path)?.[1];
  const href=value=>{
   if(!value?.startsWith('/')||value.startsWith('//')||value===base||value.startsWith(base+'/'))return value;
   if(locale&&!/^\/(en|ru)(\/|$)/.test(value)&&!/^\/(images|assets|evm-registration)(\/|$)/.test(value))value='/'+locale+value;
   return base+value;
  };
  const visit=node=>{
   if(node.type==='link'||node.type==='image')node.url=href(node.url);
   if(node.type==='mdxJsxFlowElement'||node.type==='mdxJsxTextElement'){
    if(node.name==='a'||node.name==='Cards.Card')for(const attribute of node.attributes??[])if(attribute.name==='href'&&typeof attribute.value==='string')attribute.value=href(attribute.value);
   }
   if(node.type==='code'&&node.lang==='mermaid'){
    node.type='html';node.value='<pre class="mermaid">'+node.value.replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;')+'</pre>';delete node.lang;
   }
   for(const child of node.children??[])visit(child);
  };visit(tree);
 };
}
