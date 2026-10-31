package sftpx

import (
	"errors"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type EntryType string

const (
	EntryFile      EntryType = "file"
	EntryDirectory EntryType = "dir"
	EntryLink      EntryType = "link"
	EntryOther     EntryType = "other"
)

type RemoteEntry struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modifiedAt"`
	Type       EntryType `json:"type"`
}

// Endpoint owns the browsing SFTP channel and opens an independent channel for each transfer.
// The SSH connection itself belongs to the caller and is not closed by Close.
type Endpoint struct {
	SessionID string

	ssh    *ssh.Client
	browse *sftp.Client
	mu     sync.RWMutex
	closed bool
}

func NewEndpoint(sessionID string, client *ssh.Client) (*Endpoint, error) {
	if sessionID == "" || client == nil {
		return nil, errors.New("invalid SFTP endpoint")
	}
	browse, err := newClient(client)
	if err != nil {
		return nil, err
	}
	return &Endpoint{SessionID: sessionID, ssh: client, browse: browse}, nil
}

func newClient(client *ssh.Client) (*sftp.Client, error) {
	return sftp.NewClient(
		client,
		sftp.UseConcurrentReads(true),
		sftp.UseConcurrentWrites(true),
		sftp.MaxConcurrentRequestsPerFile(32),
	)
}

func (e *Endpoint) transferClient() (*sftp.Client, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.closed {
		return nil, errors.New("SFTP endpoint closed")
	}
	return newClient(e.ssh)
}

func (e *Endpoint) Close() error {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	e.closed = true
	browse := e.browse
	e.mu.Unlock()
	return browse.Close()
}

func (e *Endpoint) RealPath(remotePath string) (string, error) {
	return e.browse.RealPath(remotePath)
}

func (e *Endpoint) List(remotePath string) ([]RemoteEntry, error) {
	items, err := e.browse.ReadDir(remotePath)
	if err != nil {
		return nil, err
	}
	entries := make([]RemoteEntry, 0, len(items))
	for _, item := range items {
		if item.Name() == "." || item.Name() == ".." {
			continue
		}
		entries = append(entries, RemoteEntry{
			Name:       item.Name(),
			Path:       path.Join(remotePath, item.Name()),
			Size:       item.Size(),
			ModifiedAt: item.ModTime().UTC(),
			Type:       entryType(item.Mode()),
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		leftDir, rightDir := entries[i].Type == EntryDirectory, entries[j].Type == EntryDirectory
		if leftDir != rightDir {
			return leftDir
		}
		left, right := strings.ToLower(entries[i].Name), strings.ToLower(entries[j].Name)
		if left == right {
			return entries[i].Name < entries[j].Name
		}
		return left < right
	})
	return entries, nil
}

func entryType(mode fs.FileMode) EntryType {
	switch {
	case mode.IsDir():
		return EntryDirectory
	case mode.IsRegular():
		return EntryFile
	case mode&fs.ModeSymlink != 0:
		return EntryLink
	default:
		return EntryOther
	}
}

func (e *Endpoint) Mkdir(remotePath string) error {
	return e.browse.Mkdir(remotePath)
}

func (e *Endpoint) Rename(from, to string) error {
	return e.browse.Rename(from, to)
}

func (e *Endpoint) Remove(remotePath string, recursive bool) error {
	info, err := e.browse.Lstat(remotePath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return e.browse.Remove(remotePath)
	}
	if recursive {
		entries, err := e.List(remotePath)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := e.Remove(entry.Path, true); err != nil {
				return err
			}
		}
	}
	return e.browse.RemoveDirectory(remotePath)
}

func (e *Endpoint) stat(remotePath string) (fs.FileInfo, error) {
	return e.browse.Stat(remotePath)
}

func (e *Endpoint) statIfExists(remotePath string) (fs.FileInfo, error) {
	info, err := e.stat(remotePath)
	if err == nil {
		return info, nil
	}
	if isNotExist(err) {
		return nil, nil
	}
	return nil, err
}

func isNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || (err != nil && strings.Contains(strings.ToLower(err.Error()), "no such file"))
}
