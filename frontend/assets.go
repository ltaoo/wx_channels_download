package frontend

import (
	_ "embed"
)

//go:embed public/timeless/0.26.0/timeless.umd.min.js
var JSTimeless []byte

//go:embed public/timeless/0.26.0/timeless.dom.umd.min.js
var JSTimelessDOM []byte

//go:embed public/timeless/0.26.0/timeless.web.umd.min.js
var JSTimelessWeb []byte
