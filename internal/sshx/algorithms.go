package sshx

import (
	"slices"

	"golang.org/x/crypto/ssh"
)

// transportAlgorithms devolve somente os defaults seguros da biblioteca, a menos que o host
// habilite explicitamente os algoritmos legados. Os modernos permanecem na frente para serem
// preferidos sempre que o servidor oferecer uma alternativa segura.
func transportAlgorithms(legacy bool) (kex, ciphers, macs []string) {
	defaults := ssh.SupportedAlgorithms()
	kex = slices.Clone(defaults.KeyExchanges)
	ciphers = slices.Clone(defaults.Ciphers)
	macs = slices.Clone(defaults.MACs)
	if legacy {
		kex = append(kex,
			ssh.InsecureKeyExchangeDH14SHA1,
			ssh.InsecureKeyExchangeDH1SHA1,
		)
		ciphers = append(ciphers,
			ssh.InsecureCipherAES128CBC,
			ssh.InsecureCipherTripleDESCBC,
		)
	}
	return kex, ciphers, macs
}

// algorithmsFor devolve os algoritmos de assinatura aceitos para um tipo de chave. Para RSA,
// SHA-2 continua preferido e ssh-rsa (SHA-1) só é anunciado a hosts legados.
func algorithmsFor(keyType string, legacy bool) []string {
	if keyType == ssh.KeyAlgoRSA {
		out := []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256}
		if legacy {
			out = append(out, ssh.KeyAlgoRSA)
		}
		return out
	}
	return []string{keyType}
}

// hostKeyAlgorithms põe primeiro o tipo da chave já aceita, para o servidor não apresentar
// outra chave e parecer uma chave alterada.
func hostKeyAlgorithms(legacy bool, known string) []string {
	all := slices.Clone(ssh.SupportedAlgorithms().HostKeys)
	if legacy {
		all = append(all, ssh.KeyAlgoRSA)
	}
	if known == "" {
		return all
	}
	first := algorithmsFor(known, legacy)
	rest := slices.DeleteFunc(slices.Clone(all), func(a string) bool { return slices.Contains(first, a) })
	return append(first, rest...)
}
