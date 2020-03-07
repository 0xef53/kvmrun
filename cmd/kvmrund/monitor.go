package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	qmp "github.com/0xef53/go-qmp"
)

// QMPPool represents a pool of QMPConn objects
// accessed by virtual machine name
type QMPPool struct {
	mu     sync.Mutex
	mondir string
	table  map[string]*qmp.Monitor
}

func NewQMPPool(mondir string) *QMPPool {
	p := QMPPool{mondir: mondir}
	p.table = make(map[string]*qmp.Monitor)
	return &p
}

func (p *QMPPool) NewMonitor(vmname string) (*qmp.Monitor, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	m, err := qmp.NewMonitor(filepath.Join(p.mondir, vmname+".qmp0"), time.Second*256)
	if err != nil {
		return nil, err
	}

	p.table[vmname] = m

	return m, nil
}

func (p *QMPPool) CloseMonitor(vmname string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if m, found := p.table[vmname]; found {
		m.Close()
		delete(p.table, vmname)
	}
}

func (p *QMPPool) Exists(vmname string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, found := p.table[vmname]

	return found
}

func (p *QMPPool) Get(vmname string) (*qmp.Monitor, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	m, found := p.table[vmname]
	if !found {
		return nil, false
	}

	return m, true
}

func (p *QMPPool) getMonitor(vmname string) (*qmp.Monitor, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	m, found := p.table[vmname]
	if !found {
		return nil, &net.OpError{Op: "read/write", Net: "unix", Err: &os.SyscallError{"syscall", syscall.ENOENT}}
	}

	return m, nil
}

func (p *QMPPool) Run(vmname string, cmd interface{}, res interface{}) error {
	m, err := p.getMonitor(vmname)
	if err != nil {
		return err
	}

	return m.Run(cmd, res)
}

func (p *QMPPool) RunTransaction(vmname string, cmds []qmp.Command, res interface{}) error {
	m, err := p.getMonitor(vmname)
	if err != nil {
		return err
	}

	return m.RunTransaction(cmds, res, nil)
}

func (p *QMPPool) GetEvents(vmname string, ctx context.Context, t string, after uint64) ([]qmp.Event, error) {
	m, err := p.getMonitor(vmname)
	if err != nil {
		return nil, err
	}

	return m.GetEvents(ctx, t, after)
}

func (p *QMPPool) FindEvents(vmname string, t string, after uint64) ([]qmp.Event, bool, error) {
	m, err := p.getMonitor(vmname)
	if err != nil {
		return nil, false, err
	}

	events, found := m.FindEvents(t, after)

	return events, found, nil
}

func (p *QMPPool) WaitDeviceDeletedEvent(vmname string, ctx context.Context, device string, after uint64) (*qmp.Event, error) {
	m, err := p.getMonitor(vmname)
	if err != nil {
		return nil, err
	}

	return m.WaitDeviceDeletedEvent(ctx, device, after)
}

func (p *QMPPool) WaitJobStatusChangeEvent(vmname string, ctx context.Context, jobID, status string, after uint64) (*qmp.Event, error) {
	m, err := p.getMonitor(vmname)
	if err != nil {
		return nil, err
	}

	return m.WaitJobStatusChangeEvent(ctx, jobID, status, after)
}

func (p *QMPPool) FindBlockJobErrorEvent(vmname, device string, after uint64) (*qmp.Event, bool, error) {
	m, err := p.getMonitor(vmname)
	if err != nil {
		return nil, false, err
	}

	return m.FindBlockJobErrorEvent(device, after)
}

func (p *QMPPool) FindBlockJobCompletedEvent(vmname, device string, after uint64) (*qmp.Event, bool, error) {
	m, err := p.getMonitor(vmname)
	if err != nil {
		return nil, false, err
	}

	return m.FindBlockJobCompletedEvent(device, after)
}
