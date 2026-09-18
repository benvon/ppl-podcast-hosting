package site

import "embed"

// pageTemplates keeps deployable markup inside the site package while making
// it editable independently from the rendering code.
//
//go:embed templates/*.html
var pageTemplates embed.FS
