export default {
  index: 'Overview',
  'getting-started': {
    title: 'Getting Started',
    type: 'menu',
    items: {
      index: 'Introduction',
      installation: 'Installation',
      quickstart: 'Quick Start',
      'project-structure': 'Project Structure'
    }
  },
  architecture: {
    title: 'Architecture',
    type: 'menu',
    items: {
      index: 'Overview',
      'domain-driven-design': 'Domain-Driven Design',
      'module-system': 'Module System',
      'multi-tenancy': 'Multi-Tenancy',
      'frontend-stack': 'Frontend Stack'
    }
  },
  '-- Modules': {
    type: 'separator',
    title: 'Modules'
  },
  core: 'Core Module',
  finance: 'Finance',
  warehouse: 'Warehouse',
  projects: 'Projects',
  hrm: 'HRM',
  billing: 'Billing',
  superadmin: 'SuperAdmin',
  bichat: 'BiChat',
  '-- API': {
    type: 'separator',
    title: 'API & Reference'
  },
  api: {
    title: 'API Reference',
    type: 'menu',
    items: {
      index: 'Overview',
      core: 'Core API',
      warehouse: 'Warehouse API'
    }
  },
  advanced: 'Advanced Topics',
  '-- Infrastructure': {
    type: 'separator',
    title: 'Infrastructure'
  },
  logging: 'Logging',
  testkit: 'Testkit'
}