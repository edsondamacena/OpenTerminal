// Package frontend expõe a interface compilada (dist/) para binários fora do main da raiz, como o
// núcleo do Android. O app desktop continua com o próprio go:embed em main.go.
package frontend

import "embed"

//go:embed all:dist
var Assets embed.FS
