export default {
  index: 'Overview',
  'getting-started': 'Getting Started',
  architecture: 'Architecture',
  modules: {
    title: 'Modules',
    type: 'menu',
    items: {
      core: { title: 'Core Module', href: '/core' },
      finance: { title: 'Finance', href: '/finance' },
      warehouse: { title: 'Warehouse', href: '/warehouse' },
      projects: { title: 'Projects', href: '/projects' },
      hrm: { title: 'HRM', href: '/hrm' },
      billing: { title: 'Billing', href: '/billing' },
      superadmin: { title: 'SuperAdmin', href: '/superadmin' },
      bichat: { title: 'BiChat', href: '/bichat' }
    }
  },
  '-- API': {
    type: 'separator',
    title: 'API & Reference'
  },
  api: 'API Reference',
  advanced: 'Advanced Topics',
  '-- Infrastructure': {
    type: 'separator',
    title: 'Infrastructure'
  },
  logging: 'Logging',
  testkit: 'Testkit'
}
