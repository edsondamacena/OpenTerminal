package session

import "unicode/utf8"

// utf8Decoder converte bytes em texto válido entre leituras: um caractere partido fica guardado
// até a próxima leitura e bytes inválidos viram U+FFFD (como o TextDecoder do navegador).
type utf8Decoder struct {
	pending []byte
}

func (d *utf8Decoder) Decode(p []byte) string {
	data := p
	if len(d.pending) > 0 {
		data = append(d.pending, p...)
		d.pending = nil
	}
	end := len(data)
	// Só os últimos 3 bytes podem ser o começo de um caractere ainda incompleto.
	for i := len(data) - 1; i >= 0 && i >= len(data)-utf8.UTFMax+1; i-- {
		if utf8.RuneStart(data[i]) {
			if !utf8.FullRune(data[i:]) {
				end = i
			}
			break
		}
	}
	if end < len(data) {
		d.pending = append([]byte(nil), data[end:]...)
	}
	return toValid(data[:end])
}

// Flush entrega o que sobrou de um caractere incompleto como U+FFFD.
func (d *utf8Decoder) Flush() string {
	if len(d.pending) == 0 {
		return ""
	}
	d.pending = nil
	return string(utf8.RuneError)
}

func toValid(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	out := make([]byte, 0, len(b)+8)
	for len(b) > 0 {
		r, n := utf8.DecodeRune(b)
		if r == utf8.RuneError && n == 1 {
			out = utf8.AppendRune(out, utf8.RuneError)
		} else {
			out = append(out, b[:n]...)
		}
		b = b[n:]
	}
	return string(out)
}
