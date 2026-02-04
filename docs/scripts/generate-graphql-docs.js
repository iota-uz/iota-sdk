#!/usr/bin/env node

/**
 * GraphQL Schema to MDX Documentation Generator
 * 
 * This script parses GraphQL schema files and generates MDX documentation.
 * Run this script after schema changes to update API docs.
 * 
 * Usage: node scripts/generate-graphql-docs.js
 */

const fs = require('fs');
const path = require('path');
const glob = require('glob');

// Configuration
const SCHEMA_PATTERN = '../../modules/**/interfaces/graph/*.graphql';
const OUTPUT_DIR = '../../docs/content/api';

/**
 * Parse a GraphQL schema file and extract types
 */
function parseSchema(content) {
  const types = [];
  
  // Parse type definitions
  const typeRegex = /type\s+(\w+)\s*\{([^}]+)\}/g;
  let match;
  
  while ((match = typeRegex.exec(content)) !== null) {
    const typeName = match[1];
    const fieldsBlock = match[2];
    
    // Parse fields
    const fields = [];
    const fieldLines = fieldsBlock.split('\n').filter(line => line.trim());
    
    for (const line of fieldLines) {
      const fieldMatch = line.match(/^\s*(\w+)\s*:\s*(.+)$/);
      if (fieldMatch) {
        fields.push({
          name: fieldMatch[1],
          type: fieldMatch[2].trim(),
        });
      }
    }
    
    types.push({
      name: typeName,
      fields,
      kind: 'type'
    });
  }
  
  // Parse queries
  const queryMatch = content.match(/extend\s+type\s+Query\s*\{([^}]+)\}/s);
  if (queryMatch) {
    const queries = [];
    const queryLines = queryMatch[1].split('\n').filter(line => line.trim());
    
    for (const line of queryLines) {
      const queryFieldMatch = line.match(/^\s*(\w+)\s*\(\s*([^)]*)\s*\)\s*:\s*(.+)$/);
      if (queryFieldMatch) {
        queries.push({
          name: queryFieldMatch[1],
          args: queryFieldMatch[2].trim(),
          returnType: queryFieldMatch[3].trim()
        });
      } else {
        const simpleQueryMatch = line.match(/^\s*(\w+)\s*:\s*(.+)$/);
        if (simpleQueryMatch) {
          queries.push({
            name: simpleQueryMatch[1],
            args: '',
            returnType: simpleQueryMatch[2].trim()
          });
        }
      }
    }
    
    if (queries.length > 0) {
      types.push({
        name: 'Query',
        fields: queries,
        kind: 'query'
      });
    }
  }
  
  return types;
}

/**
 * Generate MDX content from parsed types
 */
function generateMDXContent(moduleName, types) {
  const typesContent = types
    .filter(t => t.kind === 'type')
    .map(type => {
      const fields = type.fields.map(f => 
        `| ${f.name} | \`${f.type}\` | - |`
      ).join('\n');
      
      return `
### ${type.name}

${type.fields.length > 0 ? `
**Fields:**

| Name | Type | Description |
|------|------|-------------|
${fields}
` : 'No fields defined.'}
`;
    }).join('\n');
  
  const queries = types.find(t => t.kind === 'query');
  const queriesContent = queries ? `
## Queries

${queries.fields.map(q => {
  const args = q.args ? `(${q.args})` : '';
  return `
### ${q.name}

\`\`\`graphql
${q.name}${args}: ${q.returnType}
\`\`\`
`;
}).join('\n')}
` : '';
  
  return `---
title: '${capitalize(moduleName)} API'
description: 'GraphQL API reference for ${moduleName} module'
---

# ${capitalize(moduleName)} API

This section documents the GraphQL API for the ${moduleName} module.

## Types

${typesContent || 'No types defined.'}

${queriesContent}

## Mutations

Mutations follow the standard GraphQL mutation pattern. See the module documentation for specific mutation details.

## Subscriptions

Subscriptions are available for real-time updates. See the WebSocket documentation for connection details.

---

*This documentation is auto-generated from GraphQL schema files. Last updated: ${new Date().toISOString().split('T')[0]}*
`;
}

/**
 * Capitalize first letter
 */
function capitalize(str) {
  return str.charAt(0).toUpperCase() + str.slice(1);
}

/**
 * Main generation function
 */
function generateDocs() {
  console.log('🔍 Finding GraphQL schema files...');
  
  const schemaFiles = glob.sync(SCHEMA_PATTERN);
  console.log(`📁 Found ${schemaFiles.length} schema files`);
  
  // Group files by module
  const moduleFiles = {};
  for (const file of schemaFiles) {
    const moduleMatch = file.match(/modules\/([^/]+)/);
    if (moduleMatch) {
      const moduleName = moduleMatch[1];
      if (!moduleFiles[moduleName]) {
        moduleFiles[moduleName] = [];
      }
      moduleFiles[moduleName].push(file);
    }
  }
  
  // Process each module
  for (const [moduleName, files] of Object.entries(moduleFiles)) {
    console.log(`\n📝 Processing ${moduleName} module...`);
    
    // Only process modules with GraphQL schemas (core, warehouse)
    if (!['core', 'warehouse'].includes(moduleName)) {
      console.log(`  ⏭️  Skipping ${moduleName} (no GraphQL docs needed)`);
      continue;
    }
    
    // Parse all schema files for this module
    let allTypes = [];
    for (const file of files) {
      const content = fs.readFileSync(file, 'utf-8');
      const types = parseSchema(content);
      allTypes = allTypes.concat(types);
    }
    
    if (allTypes.length === 0) {
      console.log(`  ⚠️  No types found in ${moduleName}`);
      continue;
    }
    
    // Generate MDX content
    const mdxContent = generateMDXContent(moduleName, allTypes);
    
    // Write to file
    const outputPath = path.join(OUTPUT_DIR, moduleName, 'index.mdx');
    const outputDir = path.dirname(outputPath);
    
    if (!fs.existsSync(outputDir)) {
      fs.mkdirSync(outputDir, { recursive: true });
    }
    
    fs.writeFileSync(outputPath, mdxContent);
    console.log(`  ✅ Generated ${outputPath}`);
  }
  
  // Generate main API index
  const apiIndexContent = `---
title: 'API Reference'
description: 'GraphQL and REST API documentation'
---

# API Reference

IOTA SDK provides both GraphQL and REST APIs for integration and custom development.

## GraphQL API

GraphQL is available for flexible data queries and mutations:

| Module | Endpoint | Description |
|--------|----------|-------------|
| **Core** | "/graphql" | Users, uploads, authentication |
| **Warehouse** | "/warehouse/graphql" | Products, inventory, orders |

### GraphQL Features

- **Strongly typed** - Schema defines all available operations
- **Flexible queries** - Request exactly what you need
- **Real-time** - Subscriptions for live updates
- **Introspection** - Self-documenting API

### Making GraphQL Requests

GraphQL endpoint accepts POST requests with JSON body:

\`\`\`json
{
  "query": "query { users(limit: 10) { id firstName email } }"
}
\`\`\`

## REST API

REST endpoints are available for traditional HTTP operations:

### Authentication

All API requests require authentication via:
- **Session cookie** - For browser clients
- **API token** - For programmatic access (if enabled)

### Common Patterns

| Operation | Method | Pattern |
|-----------|--------|---------|
| List | GET | "/resource" |
| Get one | GET | "/resource/:id" |
| Create | POST | "/resource" |
| Update | PUT | "/resource/:id" |
| Delete | DELETE | "/resource/:id" |

## Module APIs

Select a module to view its API documentation:

- [Core API](/api/core) - Users, authentication, uploads
- [Warehouse API](/api/warehouse) - Products, inventory, orders

## API Explorer

Use the GraphQL Playground to explore the API:

1. Start the development server
2. Navigate to "/graphql" (or module-specific endpoint)
3. Use the interactive explorer to build queries

## SDK Clients

While you can use raw HTTP requests, consider:

- **Generated clients** - Use GraphQL codegen for type-safe clients
- **Apollo Client** - Feature-rich GraphQL client
- **Fetch/Axios** - Simple HTTP clients for REST

---

*Note: Not all modules expose GraphQL schemas. Modules without GraphQL use REST endpoints exclusively.*
`;
  
  fs.writeFileSync(path.join(OUTPUT_DIR, 'index.mdx'), apiIndexContent);
  console.log(`\n✅ Generated ${path.join(OUTPUT_DIR, 'index.mdx')}`);
  
  // Generate _meta.js for API section
  const metaContent = `export default {
  index: 'Overview',
  core: 'Core API',
  warehouse: 'Warehouse API'
}`;
  
  fs.writeFileSync(path.join(OUTPUT_DIR, '_meta.js'), metaContent);
  console.log(`✅ Generated ${path.join(OUTPUT_DIR, '_meta.js')}`);
  
  console.log('\n🎉 API documentation generation complete!');
  console.log('\nNext steps:');
  console.log('  1. Review generated docs in docs/content/api/');
  console.log('  2. Run "pnpm run build" to test documentation build');
  console.log('  3. Run this script again after GraphQL schema changes');
}

// Run if called directly
if (require.main === module) {
  generateDocs();
}

module.exports = { generateDocs, parseSchema };
