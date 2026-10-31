//go:build bindings

package main

// O wails build compila com a tag bindings e roda o binário só para gerar frontend/wailsjs;
// nessa execução o app não pode abrir o navegador nem travar a pasta de dados.
const generatingBindings = true
