package cloudsync

import (
	"bytes"
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"openterminal/internal/model"
	"openterminal/internal/vault"
)

// ErrNoFile: ainda não há cofre na nuvem (primeiro aparelho a sincronizar).
var ErrNoFile = errors.New("nenhum cofre na nuvem")

// Remote guarda um único arquivo: o cofre cifrado.
type Remote interface {
	Download(ctx context.Context) ([]byte, error)
	Upload(ctx context.Context, data []byte) error
}

// Store é a parte do vault.Store que o sync usa.
type Store interface {
	Read() (*model.Content, error)
	Replace(fn func(c *model.Content) error) error
	Preference(key string, out any) (bool, error)
	SetPreference(key string, value any) error
}

const stateKey = "cloudsync.state"

// state fica nas preferências do aparelho; nada aqui é secreto.
type state struct {
	Device    string `json:"device"`
	Salt      []byte `json:"salt,omitempty"`
	Time      uint32 `json:"time,omitempty"`
	MemoryKiB uint32 `json:"memoryKiB,omitempty"`
	// BaseGen é a geração da nuvem que este aparelho já incorporou.
	BaseGen  int64  `json:"baseGen"`
	Dirty    bool   `json:"dirty"`
	LastSync string `json:"lastSync,omitempty"`
}

// Status vai para a interface.
type Status struct {
	Connected bool   `json:"connected"`
	State     string `json:"state"` // off | locked | idle | syncing | error
	LastSync  string `json:"lastSync"`
	Error     string `json:"error"`
	// Pulls sobe a cada vez que o conteúdo da nuvem foi gravado aqui: a interface recarrega.
	Pulls uint64 `json:"pulls"`
}

type Engine struct {
	store    Store
	kdf      func(salt []byte) vault.KDFParams
	now      func() time.Time
	debounce time.Duration
	onStatus func(Status)
	// KeyReady recebe cada chave derivada (para o desbloqueio automático guardar cifrada).
	KeyReady func(key []byte, p vault.KDFParams)

	mu     sync.Mutex
	remote Remote
	key    []byte
	st     state
	status Status
	seq    uint64 // conta mudanças locais; o envio só limpa Dirty se nada mudou no meio
	timer  *time.Timer
	cancel context.CancelFunc

	run sync.Mutex // uma sincronização por vez
}

func NewEngine(store Store, kdf func(salt []byte) vault.KDFParams, onStatus func(Status)) *Engine {
	e := &Engine{store: store, kdf: kdf, now: time.Now, debounce: 4 * time.Second, onStatus: onStatus}
	e.status = Status{State: "off"}
	return e
}

// SetRemote liga (ou desliga, com nil) a nuvem.
func (e *Engine) SetRemote(r Remote) {
	e.mu.Lock()
	e.remote = r
	e.status.Connected = r != nil
	if r == nil {
		e.status.State, e.status.Error = "off", ""
	} else if e.key == nil {
		e.status.State = "locked"
	}
	e.mu.Unlock()
	e.emit()
}

func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.status
}

func (e *Engine) emit() {
	if e.onStatus != nil {
		e.onStatus(e.Status())
	}
}

func (e *Engine) setStatus(state, errMsg string) {
	if errMsg != "" {
		// Vai para o stderr (logcat no Android). Mensagens de rede e do Drive, sem segredos.
		log.Printf("sync: %s", errMsg)
	}
	e.mu.Lock()
	e.status.State, e.status.Error = state, errMsg
	if state == "idle" {
		e.status.LastSync = e.st.LastSync
	}
	e.mu.Unlock()
	e.emit()
}

func (e *Engine) loadState() error {
	var st state
	if _, err := e.store.Preference(stateKey, &st); err != nil {
		return err
	}
	if st.Device == "" {
		st.Device = randomID()
	}
	e.mu.Lock()
	e.st = st
	e.status.LastSync = st.LastSync
	e.mu.Unlock()
	return nil
}

func (e *Engine) saveState() error {
	e.mu.Lock()
	st := e.st
	e.mu.Unlock()
	return e.store.SetPreference(stateKey, st)
}

// Unlocked recebe a senha mestra logo depois de destravar: deriva a chave e sincroniza.
// A senha não fica guardada; só a chave, e só até Locked.
func (e *Engine) Unlocked(password string) {
	if err := e.loadState(); err != nil {
		e.setStatus("error", err.Error())
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := e.prepareKey(ctx, password, ""); err != nil {
			e.setStatus("error", err.Error())
			return
		}
		e.startTicker()
		e.syncLogged(ctx)
	}()
}

// PasswordChanged reconcilia com a senha antiga e passa a cifrar a nuvem com a nova.
func (e *Engine) PasswordChanged(old, next string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := e.prepareKey(ctx, old, ""); err == nil {
			e.syncLogged(ctx)
		}
		if err := e.prepareKey(ctx, next, "rotate"); err != nil {
			e.setStatus("error", err.Error())
			return
		}
		e.mu.Lock()
		e.st.Dirty = true
		e.mu.Unlock()
		e.syncLogged(ctx)
	}()
}

// Connect liga a nuvem com o cofre já aberto: precisa da senha para abrir o cofre de lá.
func (e *Engine) Connect(ctx context.Context, r Remote, password string) error {
	if err := e.loadState(); err != nil {
		return err
	}
	e.SetRemote(r)
	if err := e.prepareKey(ctx, password, ""); err != nil {
		e.setStatus("error", err.Error())
		return err
	}
	e.startTicker()
	return e.SyncNow(ctx)
}

// prepareKey deriva a chave. Se a nuvem usa outro salt (outro aparelho criou o arquivo, ou a
// senha mudou lá), adota o da nuvem quando a senha abre o arquivo. mode "rotate" gera salt novo.
func (e *Engine) prepareKey(ctx context.Context, password, mode string) error {
	e.mu.Lock()
	st, remote := e.st, e.remote
	e.mu.Unlock()
	var params vault.KDFParams
	if mode == "rotate" || len(st.Salt) == 0 {
		params = e.kdf(vault.RandomSalt())
	} else {
		params = vault.KDFParams{Time: st.Time, MemoryKiB: st.MemoryKiB, Salt: st.Salt}
	}
	if remote != nil && mode != "rotate" {
		data, err := remote.Download(ctx)
		switch {
		case errors.Is(err, ErrNoFile):
		case err != nil:
			return err
		default:
			h, err := ReadHeader(data)
			if err != nil {
				return err
			}
			if !bytes.Equal(h.KDF.Salt, params.Salt) {
				key := vault.DeriveKEK([]byte(password), h.Params())
				if _, _, err := Open(data, key); err != nil {
					vault.Zero(key)
					return err
				}
				e.setKey(key, h.Params())
				return e.saveState()
			}
		}
	}
	e.setKey(vault.DeriveKEK([]byte(password), params), params)
	return e.saveState()
}

func (e *Engine) setKey(key []byte, p vault.KDFParams) {
	e.mu.Lock()
	if e.key != nil {
		vault.Zero(e.key)
	}
	e.key = key
	e.st.Salt, e.st.Time, e.st.MemoryKiB = p.Salt, p.Time, p.MemoryKiB
	ready := e.KeyReady
	copied := append([]byte(nil), key...)
	e.mu.Unlock()
	if ready != nil {
		ready(copied, p)
	}
	vault.Zero(copied)
}

// UnlockedWithKey retoma o sync no desbloqueio automático, com a chave guardada (sem senha).
// Se a nuvem tiver mudado de senha, a abertura falha e o erro aparece até destravar com a senha.
func (e *Engine) UnlockedWithKey(key []byte, p vault.KDFParams) {
	if err := e.loadState(); err != nil {
		e.setStatus("error", err.Error())
		return
	}
	e.setKey(append([]byte(nil), key...), p)
	_ = e.saveState()
	e.startTicker()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		e.syncLogged(ctx)
	}()
}

// Locked descarta a chave e para os envios.
func (e *Engine) Locked() {
	e.mu.Lock()
	if e.key != nil {
		vault.Zero(e.key)
		e.key = nil
	}
	if e.timer != nil {
		e.timer.Stop()
	}
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	connected := e.remote != nil
	e.mu.Unlock()
	if connected {
		e.setStatus("locked", "")
	}
}

// Changed marca mudança local e agenda o envio.
func (e *Engine) Changed() {
	e.mu.Lock()
	e.seq++
	e.st.Dirty = true
	ready := e.remote != nil && e.key != nil
	if ready {
		if e.timer != nil {
			e.timer.Stop()
		}
		e.timer = time.AfterFunc(e.debounce, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			e.syncLogged(ctx)
		})
	}
	e.mu.Unlock()
	_ = e.saveState()
}

// startTicker busca novidades da nuvem de tempos em tempos enquanto o cofre está aberto.
func (e *Engine) startTicker() {
	e.mu.Lock()
	if e.cancel != nil {
		e.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.mu.Unlock()
	go func() {
		t := time.NewTicker(3 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c, done := context.WithTimeout(ctx, 2*time.Minute)
				e.syncLogged(c)
				done()
			}
		}
	}()
}

func (e *Engine) syncLogged(ctx context.Context) {
	_ = e.SyncNow(ctx)
}

// SyncNow puxa a nuvem, junta se preciso e envia o que estiver pendente.
func (e *Engine) SyncNow(ctx context.Context) error {
	e.run.Lock()
	defer e.run.Unlock()
	e.mu.Lock()
	remote, key := e.remote, append([]byte(nil), e.key...)
	e.mu.Unlock()
	defer vault.Zero(key)
	if remote == nil {
		return nil
	}
	if len(key) == 0 {
		e.setStatus("locked", "")
		return nil
	}
	e.setStatus("syncing", "")
	var err error
	for range 3 {
		if err = e.syncOnce(ctx, remote, key); !errors.Is(err, errCloudMoved) && !errors.Is(err, errChangedMeanwhile) {
			break
		}
	}
	if err != nil {
		e.setStatus("error", err.Error())
		return err
	}
	e.setStatus("idle", "")
	return nil
}

var (
	errChangedMeanwhile = errors.New("o cofre mudou durante a sincronização")
	errCloudMoved       = errors.New("outro aparelho enviou durante a sincronização")
)

func (e *Engine) syncOnce(ctx context.Context, remote Remote, key []byte) error {
	e.mu.Lock()
	seq, st := e.seq, e.st
	e.mu.Unlock()

	data, err := remote.Download(ctx)
	if err != nil && !errors.Is(err, ErrNoFile) {
		return err
	}
	if err == nil {
		h, cloud, err := Open(data, key)
		if err != nil {
			return err
		}
		if h.Gen > st.BaseGen {
			local, err := e.store.Read()
			if err != nil {
				return err
			}
			pull := (!st.Dirty && st.BaseGen > 0) || isEmpty(local)
			next := cloud
			if !pull {
				next = Merge(SnapshotOf(local), cloud)
				st.Dirty = true
			}
			err = e.store.Replace(func(c *model.Content) error {
				e.mu.Lock()
				changed := e.seq != seq
				e.mu.Unlock()
				if changed {
					return errChangedMeanwhile
				}
				next.Apply(c)
				return nil
			})
			if err != nil {
				return err
			}
			st.BaseGen = h.Gen
			e.mu.Lock()
			e.status.Pulls++
			e.mu.Unlock()
		}
	} else {
		// Primeiro envio deste cofre: a nuvem passa a ter o conteúdo local.
		st.Dirty = true
	}

	if st.Dirty {
		local, err := e.store.Read()
		if err != nil {
			return err
		}
		h := Header{Gen: st.BaseGen + 1, Device: st.Device, UpdatedAt: e.now().UTC().Format(time.RFC3339)}
		h.KDF.Time, h.KDF.MemoryKiB, h.KDF.Salt = st.Time, st.MemoryKiB, st.Salt
		sealed, err := Seal(SnapshotOf(local), key, h)
		if err != nil {
			return err
		}
		// O Drive não tem gravação condicional: relê a geração logo antes de enviar para não
		// apagar o que outro aparelho acabou de mandar. Sobra uma janela de milissegundos.
		if cur, err := remote.Download(ctx); err == nil {
			if ch, err := ReadHeader(cur); err == nil && ch.Gen > st.BaseGen {
				return errCloudMoved
			}
		}
		if err := remote.Upload(ctx, sealed); err != nil {
			return err
		}
		st.BaseGen = h.Gen
	}

	e.mu.Lock()
	st.Dirty = e.seq != seq
	st.LastSync = e.now().UTC().Format(time.RFC3339)
	// Salt e KDF podem ter mudado em paralelo (troca de senha): preserva os atuais.
	st.Salt, st.Time, st.MemoryKiB = e.st.Salt, e.st.Time, e.st.MemoryKiB
	e.st = st
	e.mu.Unlock()
	return e.saveState()
}

func isEmpty(c *model.Content) bool {
	return len(c.Hosts) == 0 && len(c.Groups) == 0 && len(c.Credentials) == 0
}

func randomID() string {
	const hex = "0123456789abcdef"
	b := vault.RandomSalt()
	out := make([]byte, 0, 2*len(b))
	for _, v := range b {
		out = append(out, hex[v>>4], hex[v&15])
	}
	return string(out)
}
