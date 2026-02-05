#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NAME="${1:-}"
BASE_PATH="${2:-}"

if [ -z "$NAME" ] || [ -z "$BASE_PATH" ]; then
  echo "Usage: scripts/applet/new.sh <name> <base_path>" >&2
  exit 2
fi

if [[ ! "$NAME" =~ ^[a-z][a-z0-9-]*$ ]]; then
  echo "Invalid name: $NAME (expected kebab-case)" >&2
  exit 2
fi

if [[ ! "$BASE_PATH" =~ ^/[A-Za-z0-9/_~.-]*$ ]]; then
  echo "Invalid base path: $BASE_PATH (expected /path with [A-Za-z0-9/_~.-])" >&2
  exit 2
fi

cd "$ROOT"

MODULE_DIR="$ROOT/modules/$NAME"
if [ -e "$MODULE_DIR" ]; then
  echo "Module already exists: $MODULE_DIR" >&2
  exit 1
fi

VITE_DIR="modules/$NAME/presentation/web"
VITE_PORT="$(
  node - <<'NODE' "scripts/applets.json" "$NAME"
const fs = require('fs')
const [file, name] = process.argv.slice(2)
const data = JSON.parse(fs.readFileSync(file, 'utf8'))
if ((data.applets || []).some((a) => a.name === name)) {
  console.error(`Applet already exists in registry: ${name}`)
  process.exit(2)
}
const ports = (data.applets || []).map((a) => a.vitePort).filter((p) => typeof p === 'number')
const max = ports.length ? Math.max(...ports) : 5172
process.stdout.write(String(max + 1))
NODE
)"

PASCAL="$(
  node - <<'NODE' "$NAME"
const name = process.argv[2] || ''
const pascal = name
  .split(/[-_]/g)
  .filter(Boolean)
  .map((p) => p[0].toUpperCase() + p.slice(1))
  .join('')
process.stdout.write(pascal)
NODE
)"

UPPER="$(echo "$NAME" | tr '[:lower:]-' '[:upper:]_')"
WINDOW_GLOBAL="__${UPPER}_CONTEXT__"
APPLET_ENTRY_MODULE="/src/main.tsx"

mkdir -p "$MODULE_DIR"/{infrastructure/persistence/schema,presentation/assets/dist,presentation/locales,rpc,presentation/web/src/dev,presentation/web/dist}

cat > "$MODULE_DIR/module.go" <<EOF
package ${NAME//-/_}

import (
	"embed"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

//go:embed infrastructure/persistence/schema/*.sql
var MigrationFiles embed.FS

func NewModule() application.Module {
	return &Module{}
}

type Module struct{}

func (m *Module) Register(app application.Application) error {
	app.Migrations().RegisterSchema(&MigrationFiles)
	app.RegisterLocaleFiles(&LocaleFiles)

	const op serrors.Op = "${PASCAL}Module.Register"
	if err := app.RegisterApplet(New${PASCAL}Applet()); err != nil {
		return serrors.E(op, "failed to register ${NAME} applet", err)
	}
	return nil
}

func (m *Module) Name() string { return "${NAME}" }
EOF

cat > "$MODULE_DIR/applet.go" <<EOF
package ${NAME//-/_}

import (
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/${NAME}/presentation/assets"
	${NAME//-/_}rpc "github.com/iota-uz/iota-sdk/modules/${NAME}/rpc"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/pkg/applet"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

var distFS fs.FS

func init() {
	var err error
	distFS, err = fs.Sub(assets.DistFS, "dist")
	if err != nil {
		panic("failed to create distFS: " + err.Error())
	}
}

type ${PASCAL}Applet struct{}

func New${PASCAL}Applet() *${PASCAL}Applet { return &${PASCAL}Applet{} }

func (a *${PASCAL}Applet) Name() string     { return "${NAME}" }
func (a *${PASCAL}Applet) BasePath() string { return "${BASE_PATH}" }

func (a *${PASCAL}Applet) Config() applet.Config {
	return applet.Config{
		WindowGlobal: "${WINDOW_GLOBAL}",

		Assets: applet.AssetConfig{
			FS:           distFS,
			BasePath:     "/assets",
			ManifestPath: "manifest.json",
			Entrypoint:   "index.html",
			Dev:          devAssets(),
		},

		Mount: applet.MountConfig{
			Tag: "${NAME}-root",
			Attributes: map[string]string{
				"base-path":   "${BASE_PATH}",
				"router-mode": "url",
				"shell-mode":  "embedded",
				"style":       "display: flex; flex: 1; flex-direction: column; min-height: 0; height: 100%; width: 100%;",
			},
		},

		Middleware: a.middleware(),

		Shell: applet.ShellConfig{
			Mode: applet.ShellModeEmbedded,
			Layout: func(title string) templ.Component {
				return layouts.Authenticated(layouts.AuthenticatedProps{
					BaseProps: layouts.BaseProps{Title: title},
				})
			},
			Title: "${PASCAL}",
		},

		RPC: ${NAME//-/_}rpc.Router().Config(),
	}
}

func (a *${PASCAL}Applet) middleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		a.provideLocalizerFromContext(),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}
}

func (a *${PASCAL}Applet) provideLocalizerFromContext() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			app, err := application.UseApp(r.Context())
			if err != nil {
				panic("app not found in context")
			}
			middleware.ProvideLocalizer(app)(next).ServeHTTP(w, r)
		})
	}
}

func devAssets() *applet.DevAssetConfig {
	enabled := envBool("IOTA_APPLET_DEV_${UPPER}")
	target := strings.TrimSpace(os.Getenv("IOTA_APPLET_VITE_URL_${UPPER}"))
	if target == "" {
		target = "http://localhost:${VITE_PORT}"
	}
	entry := strings.TrimSpace(os.Getenv("IOTA_APPLET_ENTRY_${UPPER}"))
	if entry == "" {
		entry = "${APPLET_ENTRY_MODULE}"
	}
	client := strings.TrimSpace(os.Getenv("IOTA_APPLET_CLIENT_${UPPER}"))
	if client == "" {
		client = "/@vite/client"
	}
	return &applet.DevAssetConfig{
		Enabled:      enabled,
		TargetURL:    target,
		EntryModule:  entry,
		ClientModule: client,
	}
}

func envBool(key string) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return false
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
EOF

cat > "$MODULE_DIR/presentation/assets/embed.go" <<EOF
package assets

import "embed"

//go:embed dist/*
var DistFS embed.FS
EOF

cat > "$MODULE_DIR/presentation/locales/en.json" <<EOF
{
  "${PASCAL}.Title": "${PASCAL}"
}
EOF
cp "$MODULE_DIR/presentation/locales/en.json" "$MODULE_DIR/presentation/locales/ru.json"
cp "$MODULE_DIR/presentation/locales/en.json" "$MODULE_DIR/presentation/locales/uz.json"

cat > "$MODULE_DIR/infrastructure/persistence/schema/${NAME}-schema.sql" <<EOF
-- placeholder schema for ${NAME}
EOF

cat > "$MODULE_DIR/rpc/router.go" <<EOF
package rpc

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/applet"
)

type PingParams struct{}

type PingResult struct {
	Ok bool \`json:"ok"\`
}

func Router() *applet.TypedRPCRouter {
	r := applet.NewTypedRPCRouter()
	applet.AddProcedure(r, "${NAME}.ping", applet.Procedure[PingParams, PingResult]{
		Handler: func(ctx context.Context, _ PingParams) (PingResult, error) {
			return PingResult{Ok: true}, nil
		},
	})
	return r
}
EOF

cat > "$MODULE_DIR/presentation/web/package.json" <<EOF
{
  "name": "${NAME}-web",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "pnpm run dev:standalone",
    "build:css": "tailwindcss -i ./src/index.css -o ./dist/style.css --minify",
    "build:css:watch": "tailwindcss -i ./src/index.css -o ./dist/style.css --watch",
    "dev:embedded": "bash -lc 'trap \"kill 0\" EXIT; pnpm run build:css:watch & vite'",
    "dev:standalone": "bash -lc 'trap \"kill 0\" EXIT; pnpm run build:css:watch & APPLET_ASSETS_BASE=/ vite'",
    "build": "pnpm run build:css && tsc && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "@iota-uz/sdk": "file:../../../../",
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "react-router-dom": "^6.26.0"
  },
  "devDependencies": {
    "@types/react": "^18.3.3",
    "@types/react-dom": "^18.3.0",
    "@vitejs/plugin-react": "^4.3.1",
    "typescript": "^5.5.3",
    "vite": "^5.4.2",
    "@tailwindcss/cli": "4.1.18",
    "tailwindcss": "4.1.18",
    "autoprefixer": "^10.4.20",
    "postcss": "^8.4.41"
  }
}
EOF

cat > "$MODULE_DIR/presentation/web/tsconfig.json" <<'EOF'
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "Bundler",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true
  },
  "include": ["src"]
}
EOF

cat > "$MODULE_DIR/presentation/web/postcss.config.js" <<'EOF'
export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
}
EOF

cat > "$MODULE_DIR/presentation/web/tailwind.config.js" <<'EOF'
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  darkMode: "class",
  theme: { extend: {} },
  plugins: [],
}
EOF

cat > "$MODULE_DIR/presentation/web/vite.config.ts" <<EOF
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  base: (() => {
    const base = process.env.APPLET_ASSETS_BASE || '${BASE_PATH}/assets/'
    return base.endsWith('/') ? base : base + '/'
  })(),
  server: {
    port: Number(process.env.APPLET_VITE_PORT) || ${VITE_PORT},
    strictPort: true,
  },
  assetsInclude: ['**/*.css'],
  build: {
    outDir: '../assets/dist',
    emptyOutDir: true,
    manifest: true,
    cssCodeSplit: false,
    rollupOptions: {
      output: {
        entryFileNames: 'assets/[name]-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash].[ext]',
      },
    },
  },
})
EOF

cat > "$MODULE_DIR/presentation/web/index.html" <<EOF
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>${PASCAL}</title>
  </head>
  <body>
    <${NAME}-root base-path="" router-mode="url" style="display: flex; flex: 1; flex-direction: column; min-height: 0; height: 100%; width: 100%;"></${NAME}-root>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
EOF

cat > "$MODULE_DIR/presentation/web/src/index.css" <<'EOF'
@import "@iota-uz/sdk/tailwind/main.css";
EOF

cat > "$MODULE_DIR/presentation/web/src/vite-env.d.ts" <<'EOF'
/// <reference types="vite/client" />

declare module '*.css?raw' {
  const content: string
  export default content
}
EOF

cat > "$MODULE_DIR/presentation/web/src/dev/mockIotaContext.ts" <<EOF
export function injectMockContext(): void {
  if (import.meta.env.DEV && !(window as any).${WINDOW_GLOBAL}) {
    ;(window as any).${WINDOW_GLOBAL} = {
      user: { id: 1, email: 'dev@example.com', firstName: 'Dev', lastName: 'User', permissions: [] },
      tenant: { id: '00000000-0000-0000-0000-000000000000', name: 'Dev Tenant' },
      locale: { language: 'en', translations: {} },
      config: {
        basePath: '',
        assetsBasePath: '${BASE_PATH}/assets',
        rpcUIEndpoint: '${BASE_PATH}/rpc',
        shellMode: 'standalone',
      },
      route: { path: '/', params: {}, query: {} },
      session: { expiresAt: Date.now() + 3600_000, refreshURL: '/auth/refresh', csrfToken: '' },
      error: null,
    }
  }
}
EOF

cat > "$MODULE_DIR/presentation/web/src/Root.tsx" <<EOF
import { AppletProvider, AppletDevtoolsOverlay, shouldEnableAppletDevtools } from '@iota-uz/sdk'
import App from './App'

export interface RootProps {
  windowKey: string
  basePath: string
  routerMode: 'url' | 'memory'
}

export default function Root(props: RootProps) {
  return (
    <AppletProvider windowKey={props.windowKey}>
      <App basePath={props.basePath} routerMode={props.routerMode} />
      {shouldEnableAppletDevtools() ? <AppletDevtoolsOverlay /> : null}
    </AppletProvider>
  )
}
EOF

cat > "$MODULE_DIR/presentation/web/src/App.tsx" <<'EOF'
import { BrowserRouter, MemoryRouter, Routes, Route } from 'react-router-dom'

export interface AppProps {
  basePath: string
  routerMode: 'url' | 'memory'
}

export default function App({ basePath, routerMode }: AppProps) {
  const Router = routerMode === 'memory' ? MemoryRouter : BrowserRouter

  return (
    <Router {...(routerMode === 'url' ? { basename: basePath } : {})}>
      <Routes>
        <Route path="/" element={<div style={{ padding: 16 }}>Hello from applet</div>} />
      </Routes>
    </Router>
  )
}
EOF

cat > "$MODULE_DIR/presentation/web/src/main.tsx" <<EOF
import { defineReactAppletElement } from '@iota-uz/sdk'
import Root from './Root'
import { injectMockContext } from './dev/mockIotaContext'

injectMockContext()

async function main() {
  let compiledStyles = ''
  try {
    compiledStyles = (await import('../dist/style.css?raw')).default
  } catch {
    // CSS not built yet (e.g. fresh checkout). Dev scripts will build it.
  }

  defineReactAppletElement({
    tagName: '${NAME}-root',
    styles: compiledStyles,
    render: (host) => <Root windowKey='${WINDOW_GLOBAL}' basePath={host.basePath} routerMode={host.routerMode} />,
  })
}

void main()
EOF

cat > "$MODULE_DIR/presentation/web/src/rpc.generated.ts" <<EOF
// Code generated by applet-rpc-typegen. DO NOT EDIT.
export type ${PASCAL}RPC = {}
EOF

cat > "$MODULE_DIR/presentation/assets/dist/.keep" <<'EOF'

EOF

node - <<'NODE' "$NAME" "$BASE_PATH" "$VITE_DIR" "$VITE_PORT" "$APPLET_ENTRY_MODULE" "$ROOT/scripts/applets.json"
const [name, basePath, viteDir, vitePort, entryModule, file] = process.argv.slice(2)
const fs = require('fs')
const data = JSON.parse(fs.readFileSync(file, 'utf8'))
data.applets = data.applets || []
if (data.applets.some((a) => a.name === name)) {
  console.error(`Applet already exists in registry: ${name}`)
  process.exit(2)
}
data.applets.push({ name, basePath, viteDir, vitePort: Number(vitePort), entryModule })
data.applets.sort((a, b) => a.name.localeCompare(b.name))
fs.writeFileSync(file, JSON.stringify(data, null, 2) + '\n', 'utf8')
NODE

./scripts/applet/rpc-gen.sh "$NAME" >/dev/null

gofmt -w "$MODULE_DIR/module.go" "$MODULE_DIR/applet.go" "$MODULE_DIR/presentation/assets/embed.go" "$MODULE_DIR/rpc/router.go"

echo "Created applet module: modules/$NAME"
echo "Registry updated: scripts/applets.json"
echo "Next: add module registration in your app bootstrap (app.RegisterModule(${NAME//-/_}.NewModule()))"
