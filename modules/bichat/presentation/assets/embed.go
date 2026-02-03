package assets

import "embed"

// DistFS embeds the built React application from the dist/ directory.
// This filesystem is served by the AppletController for static assets (JS, CSS, images).
//
// The React build process should output files to:
//
//	modules/bichat/presentation/assets/dist/
//
// Directory structure after build:
//
//	dist/
//	├── main.js       # Bundled JavaScript
//	├── main.css      # Bundled CSS
//	└── assets/       # Images, fonts, etc.
//
// Usage:
//   - Applet Config references this FS via Assets.FS
//   - AppletController serves files via http.FileServer
//   - URLs: /bichat/assets/main.js, /bichat/assets/main.css, etc.
//
//go:embed dist/*
var DistFS embed.FS
