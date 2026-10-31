// Package forwarding manages local TCP forwarding over shared SSH connections.
package forwarding

import (
	"io"
	"net"
	"strconv"
	"sync"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
)

type StatusKind string

const (
	StatusInactive StatusKind = "inactive"
	StatusStarting StatusKind = "starting"
	StatusActive   StatusKind = "active"
	StatusError    StatusKind = "error"
)

type Status struct {
	Status StatusKind   `json:"status"`
	Reason *apperr.Code `json:"reason"`
}

type StatusEvent struct {
	RuleID string `json:"ruleId"`
	Status
}

type ActiveForward struct {
	RuleID      string       `json:"ruleId"`
	HostLabel   string       `json:"hostLabel"`
	BindAddress string       `json:"bindAddress"`
	BindPort    int          `json:"bindPort"`
	DestHost    string       `json:"destHost"`
	DestPort    int          `json:"destPort"`
	Status      StatusKind   `json:"status"`
	Reason      *apperr.Code `json:"reason"`
}

type Client interface {
	Dial(network, address string) (net.Conn, error)
	Wait() error
	Close() error
}

type Connector func(hostID string) (Client, error)
type Emitter func(StatusEvent)

type hostLink struct {
	client Client
	rules  map[string]struct{}
}

type runtime struct {
	rule      model.ForwardRule
	hostLabel string
	status    Status
	listener  net.Listener
	stop      chan struct{}
	once      sync.Once
	sockets   map[net.Conn]struct{}
}

type Manager struct {
	connect Connector
	emit    Emitter

	mu       sync.Mutex
	hosts    map[string]*hostLink
	runtimes map[string]*runtime
}

func NewManager(connect Connector, emit Emitter) *Manager {
	return &Manager{connect: connect, emit: emit, hosts: map[string]*hostLink{}, runtimes: map[string]*runtime{}}
}

func (m *Manager) Status(ruleID string) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rt := m.runtimes[ruleID]; rt != nil {
		return rt.status
	}
	return Status{Status: StatusInactive}
}

func (m *Manager) ListActive() []ActiveForward {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ActiveForward, 0, len(m.runtimes))
	for _, rt := range m.runtimes {
		if rt.status.Status == StatusInactive {
			continue
		}
		out = append(out, ActiveForward{RuleID: rt.rule.ID, HostLabel: rt.hostLabel,
			BindAddress: rt.rule.BindAddress, BindPort: rt.rule.BindPort, DestHost: rt.rule.DestHost,
			DestPort: rt.rule.DestPort, Status: rt.status.Status, Reason: rt.status.Reason})
	}
	return out
}

func (m *Manager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, rt := range m.runtimes {
		if rt.status.Status == StatusActive {
			n++
		}
	}
	return n
}

func (m *Manager) Activate(rule model.ForwardRule, hostLabel string) error {
	m.mu.Lock()
	if current := m.runtimes[rule.ID]; current != nil &&
		(current.status.Status == StatusStarting || current.status.Status == StatusActive) {
		m.mu.Unlock()
		return nil
	}
	rt := &runtime{rule: rule, hostLabel: hostLabel, status: Status{Status: StatusStarting},
		stop: make(chan struct{}), sockets: map[net.Conn]struct{}{}}
	m.runtimes[rule.ID] = rt
	m.mu.Unlock()
	m.emitStatus(rt)

	link, err := m.link(rule.HostID, rule.ID)
	if err != nil {
		m.fail(rt, err)
		return err
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(rule.BindAddress, strconv.Itoa(rule.BindPort)))
	if err != nil {
		m.releaseLink(rule.HostID, rule.ID)
		m.fail(rt, err)
		return err
	}
	m.mu.Lock()
	if m.runtimes[rule.ID] != rt {
		m.mu.Unlock()
		_ = listener.Close()
		m.releaseLink(rule.HostID, rule.ID)
		return apperr.New(apperr.Cancelled)
	}
	rt.listener = listener
	rt.status = Status{Status: StatusActive}
	m.mu.Unlock()
	m.emitStatus(rt)
	go m.accept(rt, link.client)
	return nil
}

func (m *Manager) Deactivate(ruleID string) {
	m.mu.Lock()
	rt := m.runtimes[ruleID]
	if rt != nil {
		delete(m.runtimes, ruleID)
	}
	m.mu.Unlock()
	if rt == nil {
		return
	}
	m.closeRuntime(rt)
	m.releaseLink(rt.rule.HostID, ruleID)
	rt.status = Status{Status: StatusInactive}
	m.emitStatus(rt)
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.runtimes))
	for id := range m.runtimes {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.Deactivate(id)
	}
}

func (m *Manager) link(hostID, ruleID string) (*hostLink, error) {
	m.mu.Lock()
	if link := m.hosts[hostID]; link != nil {
		link.rules[ruleID] = struct{}{}
		m.mu.Unlock()
		return link, nil
	}
	m.mu.Unlock()
	client, err := m.connect(hostID)
	if err != nil {
		return nil, err
	}
	link := &hostLink{client: client, rules: map[string]struct{}{ruleID: {}}}
	m.mu.Lock()
	if existing := m.hosts[hostID]; existing != nil {
		existing.rules[ruleID] = struct{}{}
		m.mu.Unlock()
		_ = client.Close()
		return existing, nil
	}
	m.hosts[hostID] = link
	m.mu.Unlock()
	go func() {
		_ = client.Wait()
		m.dropHost(hostID, link)
	}()
	return link, nil
}

func (m *Manager) accept(rt *runtime, client Client) {
	for {
		local, err := rt.listener.Accept()
		if err != nil {
			select {
			case <-rt.stop:
				return
			default:
				m.Deactivate(rt.rule.ID)
				return
			}
		}
		m.mu.Lock()
		rt.sockets[local] = struct{}{}
		m.mu.Unlock()
		go m.forward(rt, client, local)
	}
}

func (m *Manager) forward(rt *runtime, client Client, local net.Conn) {
	defer func() {
		_ = local.Close()
		m.mu.Lock()
		delete(rt.sockets, local)
		m.mu.Unlock()
	}()
	remote, err := client.Dial("tcp", net.JoinHostPort(rt.rule.DestHost, strconv.Itoa(rt.rule.DestPort)))
	if err != nil {
		return
	}
	defer remote.Close()
	done := make(chan struct{}, 1)
	go func() { _, _ = io.Copy(remote, local); _ = remote.Close(); done <- struct{}{} }()
	_, _ = io.Copy(local, remote)
	<-done
}

func (m *Manager) dropHost(hostID string, link *hostLink) {
	m.mu.Lock()
	if m.hosts[hostID] != link {
		m.mu.Unlock()
		return
	}
	delete(m.hosts, hostID)
	var runtimes []*runtime
	for ruleID := range link.rules {
		if rt := m.runtimes[ruleID]; rt != nil {
			delete(m.runtimes, ruleID)
			runtimes = append(runtimes, rt)
		}
	}
	m.mu.Unlock()
	reason := apperr.SessionClosed
	for _, rt := range runtimes {
		m.closeRuntime(rt)
		rt.status = Status{Status: StatusInactive, Reason: &reason}
		m.emitStatus(rt)
	}
}

func (m *Manager) releaseLink(hostID, ruleID string) {
	m.mu.Lock()
	link := m.hosts[hostID]
	if link == nil {
		m.mu.Unlock()
		return
	}
	delete(link.rules, ruleID)
	if len(link.rules) != 0 {
		m.mu.Unlock()
		return
	}
	delete(m.hosts, hostID)
	m.mu.Unlock()
	_ = link.client.Close()
}

func (m *Manager) closeRuntime(rt *runtime) {
	rt.once.Do(func() { close(rt.stop) })
	if rt.listener != nil {
		_ = rt.listener.Close()
	}
	m.mu.Lock()
	for socket := range rt.sockets {
		_ = socket.Close()
	}
	rt.sockets = map[net.Conn]struct{}{}
	m.mu.Unlock()
}

func (m *Manager) fail(rt *runtime, err error) {
	code := apperr.CodeOf(err)
	m.mu.Lock()
	if m.runtimes[rt.rule.ID] == rt {
		rt.status = Status{Status: StatusError, Reason: &code}
	}
	m.mu.Unlock()
	m.emitStatus(rt)
}

func (m *Manager) emitStatus(rt *runtime) {
	if m.emit != nil {
		m.emit(StatusEvent{RuleID: rt.rule.ID, Status: rt.status})
	}
}
