const { CodeIndexer, MilvusVectorDatabase, OpenAIEmbedding } = require('@code-indexer/core');

async function indexCodebase() {
  const embedding = new OpenAIEmbedding({
    apiKey: process.env.OPENAI_API_KEY,
    model: 'text-embedding-3-large'
  });
  
  const vectorDatabase = new MilvusVectorDatabase({
    address: process.env.MILVUS_ADDRESS,
    token: process.env.MILVUS_TOKEN
  });
  
  const indexer = new CodeIndexer({
    embedding,
    vectorDatabase
  });
  
  console.log('Starting codebase indexing...');
  const stats = await indexer.indexCodebase('.', (progress) => {
    console.log(`${progress.phase} - ${progress.percentage}%`);
  });
  
  console.log(`Indexed ${stats.indexedFiles} files, ${stats.totalChunks} chunks`);
}

indexCodebase().catch(console.error);