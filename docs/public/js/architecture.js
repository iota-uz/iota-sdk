// EAI System Architecture - Interactive Cytoscape.js Diagram
document.addEventListener('DOMContentLoaded', function() {
  const container = document.getElementById('cy');
  if (!container) return;

  // Hide loading indicator once ready
  const loading = document.getElementById('cy-loading');

  // Register dagre layout extension
  if (typeof cytoscape !== 'undefined' && typeof cytoscapeDagre !== 'undefined') {
    cytoscape.use(cytoscapeDagre);
  }

  // ==================== LAYOUT POSITIONS ====================
  // Grid-based layout with pipeline flow:
  // Row 0: CI-CD (center)
  // Row 1: Feature | Staging | Pre-prod
  // Row 2: RAG | Production | Prod Services
  // Row 3: On-prem | External | Monitoring

  const nodePositions = {
    // Row 0 - CI/CD (center top)
    'github': { x: 550, y: 60 },

    // Row 1 - Development Environments (evenly spread horizontally)
    // Feature Environment (left)
    'fe-dev': { x: 120, y: 180 },
    'fe-next': { x: 180, y: 230 },
    'fe-go': { x: 130, y: 300 },
    'fe-db': { x: 230, y: 300 },

    // Staging Environment (center)
    'st-dev': { x: 420, y: 180 },
    'st-next': { x: 480, y: 230 },
    'st-go': { x: 430, y: 300 },
    'st-db': { x: 530, y: 300 },

    // Pre-prod Environment (right)
    'pp-next': { x: 750, y: 230 },
    'pp-go': { x: 700, y: 300 },
    'pp-db': { x: 800, y: 300 },

    // Dev/QA user (near pre-prod)
    'dev-qa': { x: 880, y: 260 },

    // Row 2 - Production Tier (main production area)
    // RAG Services (left side)
    'diffy-rag': { x: 100, y: 480 },
    'rag-redis': { x: 60, y: 560 },
    'rag-postgres': { x: 140, y: 560 },
    'rag-minio': { x: 180, y: 480 },

    // Production (center - main block)
    'back-office': { x: 380, y: 440 },
    'end-user-prod': { x: 500, y: 440 },
    'mobile-app': { x: 620, y: 440 },
    'traefik': { x: 500, y: 510 },
    // Next.js Replicas (row within production)
    'nr1': { x: 350, y: 600 },
    'nr2': { x: 410, y: 600 },
    'nr3': { x: 470, y: 600 },
    'nr4': { x: 530, y: 600 },
    'nr5': { x: 590, y: 600 },
    'nr6': { x: 650, y: 600 },
    'go-be': { x: 500, y: 690 },
    'prod-db': { x: 620, y: 690 },

    // Prod Services - Telegram (right side)
    'end-user-tg': { x: 820, y: 480 },
    'tg-bot': { x: 900, y: 540 },
    'redis': { x: 820, y: 600 },

    // Row 3 - External & Support Services
    // On-prem Legacy (bottom left)
    'abc': { x: 100, y: 830 },
    'oracle-db': { x: 200, y: 830 },

    // External Services (bottom center)
    'eppp': { x: 440, y: 840 },
    'posthog': { x: 560, y: 840 },

    // Monitoring Stack (bottom right)
    'grafana': { x: 900, y: 780 },
    'loki': { x: 800, y: 850 },
    'prometheus': { x: 880, y: 850 },
    'tempo': { x: 960, y: 850 },
    'analytics-db': { x: 1000, y: 780 },
  };

  // Define graph elements
  const elements = {
    nodes: [
      // ==================== COMPOUND NODES (Subgraphs) ====================

      // CI-CD Section
      { data: { id: 'cicd', label: 'CI-CD', type: 'compound' } },

      // Feature Environment
      { data: { id: 'feature-env', label: 'Feature Environment (Railway)', type: 'compound', env: 'railway' } },

      // Staging Environment
      { data: { id: 'staging-env', label: 'Staging Environment (Railway)', type: 'compound', env: 'railway' } },

      // Pre-Production Environment
      { data: { id: 'preprod-env', label: 'Pre-Production Environment (Railway)', type: 'compound', env: 'railway' } },

      // Production Environment
      { data: { id: 'production', label: 'On-Premise Production EAI', type: 'compound', env: 'onprem' } },

      // Next.js Replicas (nested inside production)
      { data: { id: 'next-replicas', label: 'Next.js Replicas', type: 'compound', parent: 'production' } },

      // Production Services
      { data: { id: 'prod-services', label: 'Production Services (Railway)', type: 'compound', env: 'railway' } },

      // RAG Services
      { data: { id: 'rag-services', label: 'RAG Services (Railway)', type: 'compound', env: 'railway' } },

      // On-premise Legacy
      { data: { id: 'onprem-legacy', label: 'On-premise (inside EAI building)', type: 'compound', env: 'onprem' } },

      // Monitoring Stack
      { data: { id: 'monitoring', label: 'Monitoring Stack (Railway)', type: 'compound', env: 'railway' } },

      // ==================== CI-CD NODES ====================
      { data: { id: 'github', label: 'GitHub Repository', parent: 'cicd', type: 'service' } },

      // ==================== FEATURE ENV NODES ====================
      { data: { id: 'fe-dev', label: 'Developer/QA', parent: 'feature-env', type: 'user' } },
      { data: { id: 'fe-next', label: 'Next.js Frontend', parent: 'feature-env', type: 'service' } },
      { data: { id: 'fe-go', label: 'Go Backend', parent: 'feature-env', type: 'service' } },
      { data: { id: 'fe-db', label: 'Feature DB', parent: 'feature-env', type: 'database' } },

      // ==================== STAGING ENV NODES ====================
      { data: { id: 'st-dev', label: 'Developer/QA', parent: 'staging-env', type: 'user' } },
      { data: { id: 'st-next', label: 'Next.js Frontend', parent: 'staging-env', type: 'service' } },
      { data: { id: 'st-go', label: 'Go Backend', parent: 'staging-env', type: 'service' } },
      { data: { id: 'st-db', label: 'Staging DB', parent: 'staging-env', type: 'database' } },

      // ==================== PRE-PROD ENV NODES ====================
      { data: { id: 'pp-next', label: 'Next.js Frontend', parent: 'preprod-env', type: 'service' } },
      { data: { id: 'pp-go', label: 'Go Backend', parent: 'preprod-env', type: 'service' } },
      { data: { id: 'pp-db', label: 'Pre-Prod DB', parent: 'preprod-env', type: 'database' } },

      // Dev/QA external to pre-prod
      { data: { id: 'dev-qa', label: 'Dev/QA', type: 'user' } },

      // ==================== PRODUCTION NODES ====================
      { data: { id: 'back-office', label: 'Back Office User', parent: 'production', type: 'user' } },
      { data: { id: 'end-user-prod', label: 'End User', parent: 'production', type: 'user' } },
      { data: { id: 'mobile-app', label: 'Mobile App', parent: 'production', type: 'user' } },
      { data: { id: 'traefik', label: 'Traefik Load Balancer', parent: 'production', type: 'service' } },

      // Next.js Replicas
      { data: { id: 'nr1', label: 'Next.js 1', parent: 'next-replicas', type: 'service' } },
      { data: { id: 'nr2', label: 'Next.js 2', parent: 'next-replicas', type: 'service' } },
      { data: { id: 'nr3', label: 'Next.js 3', parent: 'next-replicas', type: 'service' } },
      { data: { id: 'nr4', label: 'Next.js 4', parent: 'next-replicas', type: 'service' } },
      { data: { id: 'nr5', label: 'Next.js 5', parent: 'next-replicas', type: 'service' } },
      { data: { id: 'nr6', label: 'Next.js 6', parent: 'next-replicas', type: 'service' } },

      { data: { id: 'go-be', label: 'Go Backend - Granite ERP', parent: 'production', type: 'service' } },
      { data: { id: 'prod-db', label: 'Production DB', parent: 'production', type: 'database' } },

      // ==================== PRODUCTION SERVICES NODES ====================
      { data: { id: 'end-user-tg', label: 'End User', parent: 'prod-services', type: 'user' } },
      { data: { id: 'tg-bot', label: 'Telegram Bot', parent: 'prod-services', type: 'service' } },
      { data: { id: 'redis', label: 'Redis', parent: 'prod-services', type: 'database' } },

      // ==================== RAG SERVICES NODES ====================
      { data: { id: 'diffy-rag', label: 'Diffy RAG', parent: 'rag-services', type: 'service' } },
      { data: { id: 'rag-redis', label: 'Redis', parent: 'rag-services', type: 'database' } },
      { data: { id: 'rag-postgres', label: 'Postgres', parent: 'rag-services', type: 'database' } },
      { data: { id: 'rag-minio', label: 'Minio', parent: 'rag-services', type: 'database' } },

      // ==================== ON-PREM LEGACY NODES ====================
      { data: { id: 'abc', label: 'ABC - Oracle App (Legacy)', parent: 'onprem-legacy', type: 'service' } },
      { data: { id: 'oracle-db', label: 'Oracle DB', parent: 'onprem-legacy', type: 'database' } },

      // ==================== EXTERNAL SERVICES ====================
      { data: { id: 'eppp', label: 'EPPP (Gov Data Provider)', type: 'external' } },
      { data: { id: 'posthog', label: 'Posthog (Cloud)', type: 'external' } },

      // ==================== MONITORING NODES ====================
      { data: { id: 'loki', label: 'Loki', parent: 'monitoring', type: 'service' } },
      { data: { id: 'prometheus', label: 'Prometheus', parent: 'monitoring', type: 'service' } },
      { data: { id: 'tempo', label: 'Tempo', parent: 'monitoring', type: 'service' } },
      { data: { id: 'grafana', label: 'Grafana', parent: 'monitoring', type: 'service' } },
      { data: { id: 'analytics-db', label: 'Analytics DB', parent: 'monitoring', type: 'database' } },
    ],

    edges: [
      // ==================== CI-CD CONNECTIONS ====================
      // Connect to specific services in each environment
      { data: { source: 'github', target: 'fe-next', label: 'Deploy feature' } },
      { data: { source: 'github', target: 'st-next', label: 'Deploy staging' } },
      { data: { source: 'github', target: 'go-be', label: 'Deploy production' } },

      // ==================== FEATURE ENV CONNECTIONS ====================
      { data: { source: 'fe-next', target: 'fe-go', label: 'API Requests' } },
      { data: { source: 'fe-go', target: 'fe-db', label: 'Reads/Writes' } },

      // ==================== STAGING ENV CONNECTIONS ====================
      { data: { source: 'st-next', target: 'st-go', label: 'API Requests' } },
      { data: { source: 'st-go', target: 'st-db', label: 'Reads/Writes' } },

      // ==================== PRE-PROD ENV CONNECTIONS ====================
      { data: { source: 'pp-next', target: 'pp-go', label: 'API Requests' } },
      { data: { source: 'pp-go', target: 'pp-db', label: 'Reads/Writes' } },
      { data: { source: 'dev-qa', target: 'pp-db', label: 'Syncs Data' } },

      // ==================== PRODUCTION USER FLOWS ====================
      { data: { source: 'back-office', target: 'traefik' } },
      { data: { source: 'end-user-prod', target: 'traefik', label: 'Visits Website' } },
      { data: { source: 'mobile-app', target: 'traefik' } },

      // ==================== PRODUCTION ROUTING ====================
      { data: { source: 'traefik', target: 'nr3', label: 'eai.uz' } },
      { data: { source: 'traefik', target: 'go-be', label: 'erp.eai.uz' } },
      { data: { source: 'nr3', target: 'go-be', label: 'API Requests' } },
      { data: { source: 'go-be', target: 'prod-db', label: 'Reads/Writes' } },

      // ==================== PRODUCTION SERVICES CONNECTIONS ====================
      { data: { source: 'end-user-tg', target: 'tg-bot', label: 'Interacts with' } },
      { data: { source: 'tg-bot', target: 'go-be', label: 'erp.eai.uz' } },
      { data: { source: 'redis', target: 'go-be', label: 'Manages Sessions' } },

      // ==================== RAG SERVICES CONNECTIONS ====================
      { data: { source: 'diffy-rag', target: 'go-be', label: 'GraphQL API' } },
      { data: { source: 'diffy-rag', target: 'nr3', label: 'RAG for AI chat', style: 'dashed' } },

      // ==================== LEGACY CONNECTIONS ====================
      { data: { source: 'abc', target: 'oracle-db' } },
      { data: { source: 'go-be', target: 'abc', label: 'Legacy API' } },

      // ==================== EXTERNAL CONNECTIONS ====================
      { data: { source: 'go-be', target: 'eppp', label: 'Gov Data API' } },
      { data: { source: 'nr3', target: 'posthog', label: 'Client Events' } },

      // ==================== MONITORING CONNECTIONS ====================
      { data: { source: 'grafana', target: 'loki', label: 'Queries Logs' } },
      { data: { source: 'grafana', target: 'prometheus', label: 'Queries Metrics' } },
      { data: { source: 'grafana', target: 'tempo', label: 'Queries Traces' } },
      { data: { source: 'grafana', target: 'analytics-db' } },
      { data: { source: 'go-be', target: 'loki', label: 'Sends Logs' } },
      { data: { source: 'go-be', target: 'prometheus', label: 'Exposes Metrics' } },
      { data: { source: 'prod-db', target: 'analytics-db', label: 'Periodic Copy (30 min)' } },
    ]
  };

  // Stylesheet
  const style = [
    // Base node style
    {
      selector: 'node',
      style: {
        'label': 'data(label)',
        'text-valign': 'center',
        'text-halign': 'center',
        'font-size': '11px',
        'font-family': '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
        'text-wrap': 'wrap',
        'text-max-width': '100px',
        'background-color': '#666',
        'color': '#fff',
        'border-width': 1,
        'border-color': '#333',
        'padding': '10px',
        'min-width': '80px',
        'min-height': '40px',
        'shape': 'round-rectangle'
      }
    },

    // Compound nodes (subgraphs)
    {
      selector: 'node[type="compound"]',
      style: {
        'background-opacity': 0.1,
        'background-color': '#888',
        'border-width': 2,
        'border-style': 'solid',
        'text-valign': 'top',
        'text-halign': 'center',
        'font-size': '13px',
        'font-weight': 'bold',
        'color': '#333',
        'padding': '20px',
        'text-margin-y': -5
      }
    },

    // Railway environment compounds
    {
      selector: 'node[env="railway"]',
      style: {
        'background-color': '#1a1a2e',
        'border-color': '#16213e',
        'color': '#1a1a2e'
      }
    },

    // On-premise environment compounds
    {
      selector: 'node[env="onprem"]',
      style: {
        'background-color': '#2d3436',
        'border-color': '#636e72',
        'color': '#2d3436'
      }
    },

    // Service nodes
    {
      selector: 'node[type="service"]',
      style: {
        'background-color': '#3498db',
        'border-color': '#2980b9',
        'color': '#fff'
      }
    },

    // Database nodes
    {
      selector: 'node[type="database"]',
      style: {
        'background-color': '#00b894',
        'border-color': '#00a085',
        'color': '#fff',
        'shape': 'barrel'
      }
    },

    // User nodes
    {
      selector: 'node[type="user"]',
      style: {
        'background-color': '#fdcb6e',
        'border-color': '#f39c12',
        'color': '#333'
      }
    },

    // External service nodes
    {
      selector: 'node[type="external"]',
      style: {
        'background-color': '#6c5ce7',
        'border-color': '#a29bfe',
        'color': '#fff'
      }
    },

    // Edge styles
    {
      selector: 'edge',
      style: {
        'width': 2,
        'line-color': '#888',
        'target-arrow-color': '#888',
        'target-arrow-shape': 'triangle',
        'curve-style': 'taxi',
        'taxi-direction': 'auto',
        'taxi-turn': '50px',
        'label': 'data(label)',
        'font-size': '9px',
        'text-background-color': '#fff',
        'text-background-opacity': 0.9,
        'text-background-padding': '2px',
        'text-rotation': 'autorotate',
        'color': '#555'
      }
    },

    // Dashed edge style
    {
      selector: 'edge[style="dashed"]',
      style: {
        'line-style': 'dashed'
      }
    },

    // Highlighted state
    {
      selector: 'node.highlighted',
      style: {
        'border-width': 3,
        'border-color': '#e74c3c',
        'z-index': 999
      }
    },
    {
      selector: 'edge.highlighted',
      style: {
        'line-color': '#e74c3c',
        'target-arrow-color': '#e74c3c',
        'width': 3,
        'z-index': 999
      }
    }
  ];

  // Initialize Cytoscape
  const cy = cytoscape({
    container: container,
    elements: elements,
    style: style,
    layout: {
      name: 'preset',
      positions: function(node) {
        const id = node.id();
        if (nodePositions[id]) {
          return nodePositions[id];
        }
        // Fallback for any nodes not in the position map
        return { x: 500, y: 500 };
      },
      padding: 50,
      fit: true
    },
    minZoom: 0.2,
    maxZoom: 3,
    wheelSensitivity: 0.3
  });

  // Hide loading once rendered
  if (loading) {
    loading.style.display = 'none';
  }

  // Hover highlighting
  cy.on('mouseover', 'node', function(e) {
    const node = e.target;
    node.addClass('highlighted');
    node.connectedEdges().addClass('highlighted');
    node.neighborhood('node').addClass('highlighted');
  });

  cy.on('mouseout', 'node', function(e) {
    const node = e.target;
    node.removeClass('highlighted');
    node.connectedEdges().removeClass('highlighted');
    node.neighborhood('node').removeClass('highlighted');
  });

  // Initialize navigator (minimap)
  const navigatorContainer = document.getElementById('cy-navigator');
  if (navigatorContainer && typeof cy.navigator === 'function') {
    cy.navigator({
      container: navigatorContainer,
      viewLiveFramerate: 0,
      thumbnailEventFramerate: 30,
      thumbnailLiveFramerate: false,
      dblClickDelay: 200
    });
  }

  // Zoom controls
  const zoomIn = document.getElementById('zoom-in');
  const zoomOut = document.getElementById('zoom-out');
  const zoomReset = document.getElementById('zoom-reset');

  if (zoomIn) {
    zoomIn.addEventListener('click', function() {
      cy.zoom({
        level: cy.zoom() * 1.3,
        renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 }
      });
    });
  }

  if (zoomOut) {
    zoomOut.addEventListener('click', function() {
      cy.zoom({
        level: cy.zoom() / 1.3,
        renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 }
      });
    });
  }

  if (zoomReset) {
    zoomReset.addEventListener('click', function() {
      cy.fit(undefined, 30);
    });
  }

  // Initial fit
  cy.fit(undefined, 30);
});
