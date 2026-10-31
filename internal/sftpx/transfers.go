package sftpx

import (
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/sftp"

	"openterminal/internal/apperr"
)

type TransferDirection string

const (
	TransferUpload   TransferDirection = "upload"
	TransferDownload TransferDirection = "download"
)

type Transfer struct {
	ID         string            `json:"id"`
	SessionID  string            `json:"sessionId"`
	Direction  TransferDirection `json:"direction"`
	LocalPath  string            `json:"localPath"`
	RemotePath string            `json:"remotePath"`
	BytesTotal int64             `json:"bytesTotal"`
	BytesDone  int64             `json:"bytesDone"`
	State      TransferState     `json:"state"`
	Reason     *string           `json:"reason"`
}

type ConflictEvent struct {
	TransferID         string    `json:"transferId"`
	Name               string    `json:"name"`
	ExistingSize       int64     `json:"existingSize"`
	ExistingModifiedAt time.Time `json:"existingModifiedAt"`
}

type TransferEvents struct {
	Transfer func(Transfer)
	Conflict func(ConflictEvent)
}

type TransferManagerOptions struct {
	Concurrency      int
	ProgressInterval time.Duration
}

type transferJob struct {
	transfer Transfer
	batch    string
	endpoint *Endpoint
	target   string
	partial  string
	replace  bool
	cancel   bool
	client   *sftp.Client
	answer   chan ConflictChoice
	lastEmit time.Time
}

type TransferManager struct {
	events        TransferEvents
	concurrency   int
	progressEvery time.Duration

	mu      sync.Mutex
	jobs    map[string]*transferJob
	queue   []*transferJob
	running int
}

func NewTransferManager(events TransferEvents, options TransferManagerOptions) *TransferManager {
	concurrency := options.Concurrency
	if concurrency <= 0 {
		concurrency = 3
	}
	progressEvery := options.ProgressInterval
	if progressEvery <= 0 {
		progressEvery = 100 * time.Millisecond
	}
	return &TransferManager{
		events:        events,
		concurrency:   concurrency,
		progressEvery: progressEvery,
		jobs:          make(map[string]*transferJob),
	}
}

func (m *TransferManager) Upload(endpoint *Endpoint, localPaths []string, remoteDir string) ([]string, error) {
	if endpoint == nil || len(localPaths) == 0 || remoteDir == "" {
		return nil, apperr.New(apperr.InvalidInput)
	}
	type localFile struct {
		path string
		size int64
	}
	files := make([]localFile, 0, len(localPaths))
	for _, localPath := range localPaths {
		info, err := os.Stat(localPath)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, apperr.New(apperr.InvalidInput)
		}
		files = append(files, localFile{path: localPath, size: info.Size()})
	}
	batch := uuid.NewString()
	ids := make([]string, 0, len(files))
	for _, file := range files {
		remotePath := path.Join(remoteDir, filepath.Base(file.path))
		ids = append(ids, m.add(endpoint, batch, TransferUpload, file.path, remotePath, file.size))
	}
	return ids, nil
}

func (m *TransferManager) Download(endpoint *Endpoint, remotePaths []string, localDir string) ([]string, error) {
	if endpoint == nil || len(remotePaths) == 0 || localDir == "" {
		return nil, apperr.New(apperr.InvalidInput)
	}
	type remoteFile struct {
		path string
		size int64
	}
	files := make([]remoteFile, 0, len(remotePaths))
	for _, remotePath := range remotePaths {
		info, err := endpoint.stat(remotePath)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, apperr.New(apperr.InvalidInput)
		}
		files = append(files, remoteFile{path: remotePath, size: info.Size()})
	}
	batch := uuid.NewString()
	ids := make([]string, 0, len(files))
	for _, file := range files {
		localPath := filepath.Join(localDir, path.Base(file.path))
		ids = append(ids, m.add(endpoint, batch, TransferDownload, localPath, file.path, file.size))
	}
	return ids, nil
}

func (m *TransferManager) add(
	endpoint *Endpoint,
	batch string,
	direction TransferDirection,
	localPath, remotePath string,
	size int64,
) string {
	id := uuid.NewString()
	target := remotePath
	partial := path.Join(path.Dir(target), PartialName(path.Base(target), id))
	if direction == TransferDownload {
		target = localPath
		partial = filepath.Join(filepath.Dir(target), PartialName(filepath.Base(target), id))
	}
	job := &transferJob{
		transfer: Transfer{
			ID:         id,
			SessionID:  endpoint.SessionID,
			Direction:  direction,
			LocalPath:  localPath,
			RemotePath: remotePath,
			BytesTotal: size,
			State:      TransferQueued,
		},
		batch: batch, endpoint: endpoint, target: target, partial: partial,
	}
	m.mu.Lock()
	m.jobs[id] = job
	transfer := job.transfer
	m.mu.Unlock()
	m.emitTransfer(transfer)
	go m.prepare(job)
	return id
}

func (m *TransferManager) prepare(job *transferJob) {
	existing, err := m.existing(job)
	if err != nil {
		m.fail(job, err)
		return
	}
	if existing != nil {
		answer := make(chan ConflictChoice, 1)
		m.mu.Lock()
		if isFinal(job.transfer.State) {
			m.mu.Unlock()
			return
		}
		job.answer = answer
		m.mu.Unlock()
		m.emitConflict(ConflictEvent{
			TransferID: job.transfer.ID, Name: filepath.Base(job.target), ExistingSize: existing.Size(),
			ExistingModifiedAt: existing.ModTime().UTC(),
		})
		choice := <-answer
		m.mu.Lock()
		job.answer = nil
		m.mu.Unlock()
		m.applyChoice(job, choice)
	}
	m.mu.Lock()
	if isFinal(job.transfer.State) {
		m.mu.Unlock()
		return
	}
	m.queue = append(m.queue, job)
	m.mu.Unlock()
	m.pump()
}

func (m *TransferManager) existing(job *transferJob) (os.FileInfo, error) {
	if job.transfer.Direction == TransferUpload {
		return job.endpoint.statIfExists(job.target)
	}
	info, err := os.Stat(job.target)
	if err == nil {
		return info, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return nil, err
}

func (m *TransferManager) applyChoice(job *transferJob, choice ConflictChoice) {
	var changed []Transfer
	var wake []chan ConflictChoice
	m.mu.Lock()
	if choice == ConflictReplace {
		job.replace = true
	}
	refs := make([]TransferRef, 0)
	for _, candidate := range m.jobs {
		if candidate.batch == job.batch {
			refs = append(refs, TransferRef{ID: candidate.transfer.ID, State: candidate.transfer.State})
		}
	}
	for id, state := range PlanConflictChoice(refs, job.transfer.ID, choice) {
		candidate := m.jobs[id]
		if candidate == nil || !CanTransition(candidate.transfer.State, state) {
			continue
		}
		candidate.transfer.State = state
		candidate.transfer.Reason = nil
		changed = append(changed, candidate.transfer)
		if candidate != job && candidate.answer != nil {
			wake = append(wake, candidate.answer)
		}
	}
	m.mu.Unlock()
	for _, answer := range wake {
		select {
		case answer <- ConflictSkip:
		default:
		}
	}
	for _, transfer := range changed {
		m.emitTransfer(transfer)
	}
}

func (m *TransferManager) ResolveConflict(id string, choice ConflictChoice) error {
	if choice != ConflictReplace && choice != ConflictSkip && choice != ConflictCancelAll {
		return apperr.New(apperr.InvalidInput)
	}
	m.mu.Lock()
	job := m.jobs[id]
	if job == nil {
		m.mu.Unlock()
		return apperr.New(apperr.NotFound)
	}
	answer := job.answer
	m.mu.Unlock()
	if answer == nil {
		return apperr.New(apperr.InvalidInput)
	}
	select {
	case answer <- choice:
		return nil
	default:
		return apperr.New(apperr.InvalidInput)
	}
}

func (m *TransferManager) Cancel(id string) error {
	var changed *Transfer
	var client *sftp.Client
	var answer chan ConflictChoice
	m.mu.Lock()
	job := m.jobs[id]
	if job == nil {
		m.mu.Unlock()
		return apperr.New(apperr.NotFound)
	}
	if isFinal(job.transfer.State) {
		m.mu.Unlock()
		return nil
	}
	job.cancel = true
	client = job.client
	answer = job.answer
	if job.transfer.State == TransferQueued {
		job.transfer.State = TransferCancelled
		copy := job.transfer
		changed = &copy
	}
	m.mu.Unlock()
	if answer != nil {
		select {
		case answer <- ConflictSkip:
		default:
		}
	}
	if client != nil {
		_ = client.Close()
	}
	if changed != nil {
		m.emitTransfer(*changed)
	}
	return nil
}

func (m *TransferManager) CancelSession(sessionID string) {
	m.mu.Lock()
	ids := make([]string, 0)
	for id, job := range m.jobs {
		if job.transfer.SessionID == sessionID && !isFinal(job.transfer.State) {
			ids = append(ids, id)
		}
	}
	m.mu.Unlock()
	for _, id := range ids {
		_ = m.Cancel(id)
	}
}

func (m *TransferManager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, job := range m.jobs {
		if !isFinal(job.transfer.State) {
			count++
		}
	}
	return count
}

func (m *TransferManager) pump() {
	for {
		m.mu.Lock()
		if m.running >= m.concurrency || len(m.queue) == 0 {
			m.mu.Unlock()
			return
		}
		job := m.queue[0]
		m.queue = m.queue[1:]
		if isFinal(job.transfer.State) {
			m.mu.Unlock()
			continue
		}
		job.transfer.State = TransferRunning
		m.running++
		transfer := job.transfer
		m.mu.Unlock()
		m.emitTransfer(transfer)
		go func() {
			m.run(job)
			m.mu.Lock()
			m.running--
			m.mu.Unlock()
			m.pump()
		}()
	}
}

func (m *TransferManager) run(job *transferJob) {
	client, err := job.endpoint.transferClient()
	if err != nil {
		m.fail(job, err)
		return
	}
	m.mu.Lock()
	job.client = client
	cancelled := job.cancel
	m.mu.Unlock()
	if cancelled {
		_ = client.Close()
		m.cancelled(job)
		return
	}
	defer func() {
		m.mu.Lock()
		job.client = nil
		m.mu.Unlock()
		_ = client.Close()
	}()

	if job.transfer.Direction == TransferUpload {
		err = m.upload(job, client)
	} else {
		err = m.download(job, client)
	}
	if err != nil {
		m.removePartial(job)
		m.mu.Lock()
		cancelled = job.cancel
		m.mu.Unlock()
		if cancelled {
			m.cancelled(job)
		} else {
			m.fail(job, err)
		}
		return
	}
	m.mu.Lock()
	cancelled = job.cancel
	m.mu.Unlock()
	if cancelled {
		m.removePartial(job)
		m.cancelled(job)
		return
	}
	m.complete(job)
}

func (m *TransferManager) upload(job *transferJob, client *sftp.Client) error {
	source, err := os.Open(job.transfer.LocalPath)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := client.OpenFile(job.partial, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(target, &progressReader{
		reader: source, size: job.transfer.BytesTotal, update: func(n int64) { m.progress(job, n) },
	})
	closeErr := target.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if m.isCancelled(job) {
		return apperr.New(apperr.Cancelled)
	}
	if job.replace {
		if _, err := client.Stat(job.target); err == nil {
			if err := client.Remove(job.target); err != nil {
				return err
			}
		} else if !isNotExist(err) {
			return err
		}
	}
	return client.Rename(job.partial, job.target)
}

func (m *TransferManager) download(job *transferJob, client *sftp.Client) error {
	source, err := client.Open(job.transfer.RemotePath)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.OpenFile(job.partial, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(&progressWriter{writer: target, update: func(n int64) { m.progress(job, n) }}, source)
	closeErr := target.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if m.isCancelled(job) {
		return apperr.New(apperr.Cancelled)
	}
	if job.replace {
		if err := os.Remove(job.target); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return os.Rename(job.partial, job.target)
}

func (m *TransferManager) isCancelled(job *transferJob) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return job.cancel
}

func (m *TransferManager) progress(job *transferJob, n int64) {
	if n <= 0 {
		return
	}
	now := time.Now()
	var emitted *Transfer
	m.mu.Lock()
	job.transfer.BytesDone += n
	if job.transfer.BytesDone > job.transfer.BytesTotal {
		job.transfer.BytesDone = job.transfer.BytesTotal
	}
	if job.lastEmit.IsZero() || now.Sub(job.lastEmit) >= m.progressEvery {
		job.lastEmit = now
		copy := job.transfer
		emitted = &copy
	}
	m.mu.Unlock()
	if emitted != nil {
		m.emitTransfer(*emitted)
	}
}

func (m *TransferManager) complete(job *transferJob) {
	m.mu.Lock()
	if !CanTransition(job.transfer.State, TransferCompleted) {
		m.mu.Unlock()
		return
	}
	job.transfer.BytesDone = job.transfer.BytesTotal
	job.transfer.State = TransferCompleted
	job.transfer.Reason = nil
	transfer := job.transfer
	m.mu.Unlock()
	m.emitTransfer(transfer)
}

func (m *TransferManager) cancelled(job *transferJob) {
	m.mu.Lock()
	if !CanTransition(job.transfer.State, TransferCancelled) {
		m.mu.Unlock()
		return
	}
	job.transfer.State = TransferCancelled
	job.transfer.Reason = nil
	transfer := job.transfer
	m.mu.Unlock()
	m.emitTransfer(transfer)
}

func (m *TransferManager) fail(job *transferJob, err error) {
	reason := transferReason(err)
	m.mu.Lock()
	if !CanTransition(job.transfer.State, TransferFailed) {
		m.mu.Unlock()
		return
	}
	job.transfer.State = TransferFailed
	job.transfer.Reason = &reason
	transfer := job.transfer
	m.mu.Unlock()
	m.emitTransfer(transfer)
}

func transferReason(err error) string {
	code := apperr.CodeOf(err)
	if code == apperr.Internal {
		code = apperr.SftpError
	}
	return apperr.Messages[code]
}

func (m *TransferManager) removePartial(job *transferJob) {
	if job.transfer.Direction == TransferUpload {
		_ = job.endpoint.browse.Remove(job.partial)
		return
	}
	_ = os.Remove(job.partial)
}

func (m *TransferManager) emitTransfer(transfer Transfer) {
	if m.events.Transfer != nil {
		m.events.Transfer(transfer)
	}
}

func (m *TransferManager) emitConflict(conflict ConflictEvent) {
	if m.events.Conflict != nil {
		m.events.Conflict(conflict)
	}
}

func isFinal(state TransferState) bool {
	return state == TransferCompleted || state == TransferCancelled || state == TransferFailed || state == TransferSkipped
}

type progressReader struct {
	reader io.Reader
	size   int64
	update func(int64)
}

// Size permite que pkg/sftp determine o número de requisições paralelas em File.ReadFrom.
// Sem este método, o wrapper de progresso força o upload para o caminho sequencial.
func (r *progressReader) Size() int64 { return r.size }

func (r *progressReader) Read(buffer []byte) (int, error) {
	n, err := r.reader.Read(buffer)
	r.update(int64(n))
	return n, err
}

type progressWriter struct {
	writer io.Writer
	update func(int64)
}

func (w *progressWriter) Write(buffer []byte) (int, error) {
	n, err := w.writer.Write(buffer)
	w.update(int64(n))
	return n, err
}
