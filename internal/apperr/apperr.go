// Package apperr define os códigos de erro da ponte (contracts/bridge.md, "ErrorCode") e a
// sanitização: nenhum texto de erro original chega à interface, só código e mensagem fixa.
package apperr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"runtime"
	"strings"
)

type Code string

const (
	InvalidInput            Code = "INVALID_INPUT"
	VaultLocked             Code = "VAULT_LOCKED"
	VaultNotFound           Code = "VAULT_NOT_FOUND"
	VaultExists             Code = "VAULT_EXISTS"
	VaultBadPassword        Code = "VAULT_BAD_PASSWORD"
	VaultUnreadable         Code = "VAULT_UNREADABLE"
	VaultUnsupportedVersion Code = "VAULT_UNSUPPORTED_VERSION"
	VaultMigrationFailed    Code = "VAULT_MIGRATION_FAILED"
	WeakPassword            Code = "WEAK_PASSWORD"
	NotFound                Code = "NOT_FOUND"
	GroupCycle              Code = "GROUP_CYCLE"
	KeyUnsupported          Code = "KEY_UNSUPPORTED"
	KeyBadPassphrase        Code = "KEY_BAD_PASSPHRASE"
	ConnectTimeout          Code = "CONNECT_TIMEOUT"
	ConnectRefused          Code = "CONNECT_REFUSED"
	HostUnreachable         Code = "HOST_UNREACHABLE"
	AuthFailed              Code = "AUTH_FAILED"
	HostKeyRejected         Code = "HOST_KEY_REJECTED"
	NoCommonAlgorithm       Code = "NO_COMMON_ALGORITHM"
	AgentUnavailable        Code = "AGENT_UNAVAILABLE"
	SessionClosed           Code = "SESSION_CLOSED"
	SftpError               Code = "SFTP_ERROR"
	PermissionDenied        Code = "PERMISSION_DENIED"
	PortInUse               Code = "PORT_IN_USE"
	SchemeInvalid           Code = "SCHEME_INVALID"
	AlreadyExists           Code = "ALREADY_EXISTS"
	InUse                   Code = "IN_USE"
	Cancelled               Code = "CANCELLED"
	Internal                Code = "INTERNAL"
)

// Codes lista todos os códigos, na ordem do contrato.
var Codes = []Code{
	InvalidInput, VaultLocked, VaultNotFound, VaultExists, VaultBadPassword, VaultUnreadable,
	VaultUnsupportedVersion, VaultMigrationFailed, WeakPassword, NotFound, GroupCycle,
	KeyUnsupported, KeyBadPassphrase, ConnectTimeout, ConnectRefused, HostUnreachable,
	AuthFailed, HostKeyRejected, NoCommonAlgorithm, AgentUnavailable, SessionClosed, SftpError,
	PermissionDenied, PortInUse, SchemeInvalid, AlreadyExists, InUse, Cancelled, Internal,
}

// Messages: texto pt-BR fixo por código (FR-005 da 001). Nunca contém dados do erro original.
var Messages = map[Code]string{
	InvalidInput:            "Dados inválidos.",
	VaultLocked:             "O cofre está bloqueado. Desbloqueie para continuar.",
	VaultNotFound:           "Nenhum cofre foi criado ainda.",
	VaultExists:             "Já existe um cofre neste computador.",
	VaultBadPassword:        "Senha incorreta ou cofre inválido.",
	VaultUnreadable:         "O arquivo do cofre está corrompido e não pode ser aberto.",
	VaultUnsupportedVersion: "O cofre foi criado por uma versão mais nova do OpenTerminal e não pode ser aberto.",
	VaultMigrationFailed:    "Não foi possível atualizar o cofre da versão anterior. O arquivo original foi mantido intacto.",
	WeakPassword:            "A senha mestra precisa ter pelo menos 8 caracteres.",
	NotFound:                "Item não encontrado.",
	GroupCycle:              "Um grupo não pode ser movido para dentro de si mesmo.",
	KeyUnsupported:          "Formato de chave não suportado. Use OpenSSH ou PEM (Ed25519, ECDSA ou RSA).",
	KeyBadPassphrase:        "A senha da chave está incorreta.",
	ConnectTimeout:          "O servidor não respondeu a tempo.",
	ConnectRefused:          "O servidor recusou a conexão. Verifique o endereço e a porta.",
	HostUnreachable:         "Não foi possível encontrar o servidor. Verifique o endereço e a rede.",
	AuthFailed:              "A autenticação falhou.",
	HostKeyRejected:         "A chave do servidor não foi aceita. Conexão cancelada.",
	NoCommonAlgorithm:       "O servidor só aceita algoritmos antigos. Ative \"Algoritmos legados (inseguros)\" no host para conectar.",
	AgentUnavailable:        "O agente SSH do Windows não está em execução.",
	SessionClosed:           "A sessão foi encerrada.",
	SftpError:               "A operação de arquivo falhou no servidor.",
	PermissionDenied:        "Permissão negada.",
	PortInUse:               "A porta local já está em uso.",
	SchemeInvalid:           "O arquivo de esquema de cores é inválido.",
	AlreadyExists:           "Já existe um item com esse nome.",
	InUse:                   "O item está em uso e não pode ser excluído.",
	Cancelled:               "Operação cancelada.",
	Internal:                "Ocorreu um erro inesperado.",
}

// Error é o único tipo de erro que atravessa a ponte. Error() devolve JSON
// {"code","message"}; o adaptador da interface o converte em Result {ok:false}.
type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Op      string `json:"op"`
	Detail  string `json:"detail"`
}

func (e *Error) Error() string {
	b, _ := json.Marshal(e)
	return string(b)
}

// New cria o erro de um código conhecido, com a mensagem fixa.
func New(code Code) *Error {
	msg, ok := Messages[code]
	if !ok {
		code, msg = Internal, Messages[Internal]
	}
	return &Error{Code: code, Message: msg, Op: bridgeOperation()}
}

// Sanitize converte qualquer erro em *Error. A mensagem continua fixa; o texto técnico fica
// isolado em Detail depois de remover padrões que possam conter segredos.
func Sanitize(err error) error {
	if err == nil {
		return nil
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		result := *appErr
		if result.Op == "" {
			result.Op = bridgeOperation()
		}
		result.Detail = redact(result.Detail)
		if err != appErr && result.Detail == "" {
			result.Detail = redact(err.Error())
		}
		return &result
	}
	result := New(CodeOf(err))
	result.Detail = redact(err.Error())
	return result
}

// Panic converte o valor recuperado de um panic no mesmo envelope seguro usado pelos erros.
func Panic(op string, value any) *Error {
	return &Error{Code: Internal, Message: Messages[Internal], Op: op, Detail: redact(fmt.Sprint(value))}
}

// Recover deve ser usado em defer por fronteiras que executam callbacks potencialmente falhos.
func Recover(op string, target *error) {
	if value := recover(); value != nil {
		*target = Panic(op, value)
	}
}

var (
	secretNames      = `password|passphrase|senha|secret|token|private[_ -]?key|chave privada|api[_ -]?key|access[_ -]?key|client[_ -]?secret`
	secretAssignment = regexp.MustCompile(`(?i)(` + secretNames + `)(\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;\]}]+)`)
	secretFlag       = regexp.MustCompile(`(?i)((?:--?)(?:` + secretNames + `)(?:\s+|=))("[^"]*"|'[^']*'|[^\s,;\]}]+)`)
	secretValue      = regexp.MustCompile(`(?i)\b(?:` + secretNames + `)[_-][^\s,;\]}]+`)
	authorization    = regexp.MustCompile(`(?i)(authorization\s*:\s*(?:basic|bearer)\s+)[^\s,;]+`)
	urlUserInfo      = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://[^:/@\s]+:)[^@\s/]+@`)
	knownToken       = regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|sk-[A-Za-z0-9_-]{16,}|AKIA[0-9A-Z]{16})\b`)
	privateKeyBlock  = regexp.MustCompile(`(?is)-----BEGIN[^\r\n]*PRIVATE KEY-----.*?-----END[^\r\n]*PRIVATE KEY-----`)
)

func redact(detail string) string {
	if detail == "" {
		return ""
	}
	detail = privateKeyBlock.ReplaceAllString(detail, "[REDACTED PRIVATE KEY]")
	detail = urlUserInfo.ReplaceAllString(detail, "$1[REDACTED]@")
	detail = authorization.ReplaceAllString(detail, "$1[REDACTED]")
	detail = secretAssignment.ReplaceAllString(detail, "$1$2[REDACTED]")
	detail = secretFlag.ReplaceAllString(detail, "$1[REDACTED]")
	detail = secretValue.ReplaceAllString(detail, "[REDACTED]")
	detail = knownToken.ReplaceAllString(detail, "[REDACTED]")
	runes := []rune(strings.TrimSpace(detail))
	if len(runes) > 2048 {
		detail = string(runes[:2048]) + "…"
	} else {
		detail = string(runes)
	}
	return detail
}

func bridgeOperation() string {
	callers := make([]uintptr, 32)
	n := runtime.Callers(3, callers)
	frames := runtime.CallersFrames(callers[:n])
	for {
		frame, more := frames.Next()
		const marker = "/internal/bridge.(*"
		if index := strings.Index(frame.Function, marker); index >= 0 {
			rest := frame.Function[index+len(marker):]
			if split := strings.Index(rest, ")."); split > 0 {
				receiver, method := rest[:split], rest[split+2:]
				if dot := strings.IndexByte(method, '.'); dot >= 0 {
					method = method[:dot]
				}
				method = strings.TrimSuffix(method, "-fm")
				if strings.HasSuffix(receiver, "Service") && method != "" {
					return receiver + "." + method
				}
			}
		}
		if !more {
			return ""
		}
	}
}

// CodeOf classifica um erro: códigos do app, cancelamento, rede, sistema de arquivos e SSH.
func CodeOf(err error) Code {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	if errors.Is(err, context.Canceled) {
		return Cancelled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ConnectTimeout
	}
	if c, ok := errnoCode(err); ok {
		return c
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return HostUnreachable
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ConnectTimeout
	}
	if errors.Is(err, os.ErrPermission) {
		return PermissionDenied
	}
	if errors.Is(err, os.ErrNotExist) {
		return NotFound
	}
	// x/crypto/ssh não exporta tipos para estes casos; o texto é estável há anos.
	msg := err.Error()
	switch {
	case strings.Contains(msg, "ssh: no common algorithm"):
		return NoCommonAlgorithm
	case strings.Contains(msg, "ssh: unable to authenticate"):
		return AuthFailed
	}
	return Internal
}
