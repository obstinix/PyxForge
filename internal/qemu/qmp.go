package qemu

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// Event is an asynchronous QMP event, such as STOP, RESUME, RESET or SHUTDOWN.
type Event struct {
	Name string          `json:"event"`
	Data json.RawMessage `json:"data"`
}

// QMP is a client for QEMU's machine protocol. Commands run one at a time; events arrive on
// Events until the connection closes.
type QMP struct {
	conn    net.Conn
	mu      sync.Mutex // one command in flight
	replies chan reply
	events  chan Event
	closed  chan struct{}
	once    sync.Once
	err     error // why the connection ended
}

type reply struct {
	Return json.RawMessage `json:"return"`
	Error  *struct {
		Class string `json:"class"`
		Desc  string `json:"desc"`
	} `json:"error"`
}

// DialQMP connects to a QMP server, retrying until ctx ends (QEMU opens its socket a moment
// after it starts), reads the greeting and leaves capabilities negotiation mode.
func DialQMP(ctx context.Context, addr string) (*QMP, error) {
	var conn net.Conn
	var err error
	for {
		var d net.Dialer
		conn, err = d.DialContext(ctx, "tcp", addr)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("connect to QMP at %s: %w", addr, err)
		case <-time.After(50 * time.Millisecond):
		}
	}
	q := &QMP{conn: conn, replies: make(chan reply, 1), events: make(chan Event, 64), closed: make(chan struct{})}
	r := bufio.NewReaderSize(conn, 64*1024)
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	greeting, err := r.ReadBytes('\n')
	if err != nil || !json.Valid(greeting) {
		conn.Close()
		return nil, fmt.Errorf("read the QMP greeting: %v", err)
	}
	_ = conn.SetReadDeadline(time.Time{})
	go q.read(r)
	if err := q.Execute(ctx, "qmp_capabilities", nil, nil); err != nil {
		q.Close()
		return nil, err
	}
	return q, nil
}

// read is the only sender on events, so it closes them when the connection ends.
func (q *QMP) read(r *bufio.Reader) {
	defer close(q.events)
	defer q.shutdown(errors.New("QEMU closed the QMP connection"))
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			var probe struct {
				Event *string `json:"event"`
			}
			if json.Unmarshal(line, &probe) == nil && probe.Event != nil {
				var ev Event
				_ = json.Unmarshal(line, &ev)
				select {
				case q.events <- ev:
				default: // nobody is listening fast enough; events are advisory
				}
			} else {
				var rep reply
				if json.Unmarshal(line, &rep) == nil {
					select {
					case q.replies <- rep:
					case <-q.closed:
						return
					}
				}
			}
		}
		if err != nil {
			return
		}
	}
}

func (q *QMP) shutdown(err error) {
	q.once.Do(func() {
		q.err = err
		close(q.closed)
		q.conn.Close()
	})
}

// Events delivers QMP events; the channel closes with the connection.
func (q *QMP) Events() <-chan Event { return q.events }

// Done is closed when the connection ends.
func (q *QMP) Done() <-chan struct{} { return q.closed }

// Execute runs a QMP command with args (nil for none) and decodes its return value into out
// (nil to ignore it).
func (q *QMP) Execute(ctx context.Context, cmd string, args, out any) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	msg := map[string]any{"execute": cmd}
	if args != nil {
		msg["arguments"] = args
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if _, err := q.conn.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("QMP %s: %w", cmd, err)
	}
	select {
	case rep := <-q.replies:
		if rep.Error != nil {
			return fmt.Errorf("QMP %s: %s", cmd, rep.Error.Desc)
		}
		if out != nil && len(rep.Return) > 0 {
			return json.Unmarshal(rep.Return, out)
		}
		return nil
	case <-q.closed:
		return fmt.Errorf("QMP %s: %v", cmd, q.err)
	case <-ctx.Done():
		return fmt.Errorf("QMP %s: %w", cmd, ctx.Err())
	}
}

// Status is QEMU's run state ("running", "paused", "debug", "shutdown"…).
func (q *QMP) Status(ctx context.Context) (state string, running bool, err error) {
	var st struct {
		Status  string `json:"status"`
		Running bool   `json:"running"`
	}
	err = q.Execute(ctx, "query-status", nil, &st)
	return st.Status, st.Running, err
}

// HMP runs a human monitor command ("info registers", "x /8xb 0x7c00") and returns its text.
func (q *QMP) HMP(ctx context.Context, line string) (string, error) {
	var out string
	err := q.Execute(ctx, "human-monitor-command", map[string]string{"command-line": line}, &out)
	return out, err
}

// Close ends the connection.
func (q *QMP) Close() { q.shutdown(errors.New("closed")) }
