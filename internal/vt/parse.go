package vt

import (
	"fmt"
	"strconv"
)

// The parser follows the DEC/xterm state machine (vt100.net/emu/dec_ansi_parser), with the
// states a terminal on a pty needs.
type state uint8

const (
	ground state = iota
	escape
	escInter // ESC and intermediates: charset designation, DECALN
	csiState
	osc
	oscEsc // ESC inside an OSC string: the start of ST
	str    // DCS, SOS, PM, APC: read and ignored until ST
	strEsc
)

type parser struct {
	state   state
	inter   []byte // intermediate bytes
	private byte   // a CSI's private marker: ? > = <
	params  []int  // -1 is a parameter left out
	sub     []bool // whether params[i] followed a ':' (a sub-parameter)
	cur     int
	have    bool
	colon   bool
	str     []byte
	replies *[][]byte
}

// byte takes one byte of output below 0x80.
func (s *Screen) byte(c byte) {
	p := &s.p
	switch c {
	case 0x18, 0x1a: // CAN, SUB: abandon a sequence
		p.state = ground
		return
	case 0x1b:
		switch p.state {
		case osc:
			p.state = oscEsc
		case str:
			p.state = strEsc
		default:
			p.state = escape
			p.inter = p.inter[:0]
		}
		return
	}
	switch p.state {
	case ground:
		if c < 0x20 || c == 0x7f {
			s.control(c)
			return
		}
		s.print(rune(c))
	case escape:
		switch {
		case c < 0x20:
			s.control(c)
		case c >= 0x20 && c <= 0x2f:
			p.inter = append(p.inter, c)
			p.state = escInter
		case c == '[':
			p.state = csiState
			p.private, p.params, p.sub, p.cur, p.have, p.colon = 0, p.params[:0], p.sub[:0], 0, false, false
			p.inter = p.inter[:0]
		case c == ']':
			p.state, p.str = osc, p.str[:0]
		case c == 'P' || c == 'X' || c == '^' || c == '_':
			p.state = str
		default:
			p.state = ground
			s.esc(c)
		}
	case escInter:
		switch {
		case c < 0x20:
			s.control(c)
		case c <= 0x2f:
			p.inter = append(p.inter, c)
		default:
			p.state = ground
			s.escInter(p.inter, c)
		}
	case csiState:
		switch {
		case c < 0x20:
			s.control(c) // controls inside a CSI take effect at once
		case c >= '0' && c <= '9':
			p.cur = p.cur*10 + int(c-'0')
			if p.cur > 1<<16 {
				p.cur = 1 << 16
			}
			p.have = true
		case c == ';' || c == ':':
			p.push()
			p.colon = c == ':'
		case c >= '<' && c <= '?':
			if len(p.params) == 0 && !p.have {
				p.private = c
			}
		case c >= 0x20 && c <= 0x2f:
			p.inter = append(p.inter, c)
		case c >= 0x40 && c <= 0x7e:
			p.push()
			p.state = ground
			s.csi(c)
		default:
			p.state = ground
		}
	case osc:
		if c == 0x07 {
			s.oscDone()
			p.state = ground
			return
		}
		if c >= 0x20 {
			p.str = append(p.str, c)
		}
	case oscEsc:
		if c == '\\' {
			s.oscDone()
		}
		p.state = ground
	case str:
		// ignored until ST
	case strEsc:
		p.state = ground
		if c != '\\' {
			s.byte(c)
		}
	}
}

func (p *parser) push() {
	v := -1
	if p.have {
		v = p.cur
	}
	p.params = append(p.params, v)
	p.sub = append(p.sub, p.colon)
	p.cur, p.have, p.colon = 0, false, false
}

// arg is parameter i, or def when it is left out or zero.
func (p *parser) arg(i, def int) int {
	if i < len(p.params) && p.params[i] > 0 {
		return p.params[i]
	}
	return def
}

// raw is parameter i as given, -1 when left out.
func (p *parser) raw(i int) int {
	if i < len(p.params) {
		return p.params[i]
	}
	return -1
}

func (s *Screen) control(c byte) {
	switch c {
	case 0x07: // BEL
	case 0x08: // BS
		if s.cur.x > 0 {
			s.cur.x--
		}
		s.cur.wrapNext = false
	case 0x09: // HT
		s.tab(1)
	case 0x0a, 0x0b, 0x0c: // LF, VT, FF
		s.lineFeed()
		if s.lnm {
			s.cur.x = 0
		}
		s.cur.wrapNext = false
	case 0x0d: // CR
		s.cur.x = 0
		s.cur.wrapNext = false
	case 0x0e: // SO
		s.cur.gl = 1
	case 0x0f: // SI
		s.cur.gl = 0
	}
}

func (s *Screen) tab(n int) {
	for ; n > 0 && s.cur.x < s.cols-1; n-- {
		s.cur.x++
		for s.cur.x < s.cols-1 && !s.tabs[s.cur.x] {
			s.cur.x++
		}
	}
	for ; n < 0 && s.cur.x > 0; n++ {
		s.cur.x--
		for s.cur.x > 0 && !s.tabs[s.cur.x] {
			s.cur.x--
		}
	}
	s.cur.wrapNext = false
}

func (s *Screen) esc(c byte) {
	switch c {
	case '7':
		s.saved = s.cur
	case '8':
		s.restore(s.saved)
	case 'D': // IND
		s.lineFeed()
		s.cur.wrapNext = false
	case 'E': // NEL
		s.cur.x = 0
		s.lineFeed()
		s.cur.wrapNext = false
	case 'H': // HTS
		s.tabs[s.cur.x] = true
	case 'M': // RI
		if s.cur.y == s.top {
			s.scrollDown(1)
		} else if s.cur.y > 0 {
			s.cur.y--
		}
		s.cur.wrapNext = false
	case 'c': // RIS: the history stays
		history, dropped, keep, reply := s.history, s.dropped, s.maxHistory, s.reply
		s.reset()
		s.history, s.dropped, s.maxHistory, s.reply, s.title = history, dropped, keep, reply, ""
	case '=':
		s.appKeypad = true
	case '>':
		s.appKeypad = false
	}
}

func (s *Screen) escInter(inter []byte, c byte) {
	if len(inter) != 1 {
		return
	}
	set := ascii
	if c == '0' {
		set = decGraphics
	}
	switch inter[0] {
	case '(':
		s.cur.g[0] = set
	case ')':
		s.cur.g[1] = set
	}
}

func (s *Screen) restore(c cursor) {
	s.cur = c
	s.cur.x = clamp(s.cur.x, 0, s.cols-1)
	s.cur.y = clamp(s.cur.y, 0, s.rows-1)
}

// moveTo puts the cursor at column x, row y, the row counted from the region's top in origin
// mode.
func (s *Screen) moveTo(x, y int) {
	lo, hi := 0, s.rows-1
	if s.cur.origin {
		lo, hi = s.top, s.bot
		y += s.top
	}
	s.cur.x = clamp(x, 0, s.cols-1)
	s.cur.y = clamp(y, lo, hi)
	s.cur.wrapNext = false
}

func (s *Screen) csi(final byte) {
	p := &s.p
	if len(p.inter) > 0 {
		if p.inter[0] == '!' && final == 'p' { // DECSTR
			s.insert, s.cur.origin, s.autowrap, s.hidden = false, false, true, false
			s.appCursor, s.appKeypad = false, false
			s.top, s.bot = 0, s.rows-1
			s.cur.pen, s.cur.g, s.cur.gl = Cell{}, [2]charset{}, 0
			s.saved = cursor{}
		}
		return // DECSCUSR (cursor shape) and the rest: ignored
	}
	if p.private == '?' {
		switch final {
		case 'h', 'l':
			for i := range p.params {
				s.decMode(p.raw(i), final == 'h')
			}
		}
		return
	}
	if p.private == '>' {
		if final == 'c' { // secondary device attributes
			s.send("\x1b[>0;10;1c")
		}
		return
	}
	if p.private != 0 {
		return
	}
	n := p.arg(0, 1)
	switch final {
	case '@': // ICH
		line := s.lines[s.cur.y].Cells
		n = min(n, s.cols-s.cur.x)
		copy(line[s.cur.x+n:], line[s.cur.x:])
		for i := s.cur.x; i < s.cur.x+n; i++ {
			line[i] = s.blank()
		}
	case 'A':
		s.cur.y = max(s.cur.y-n, s.upperLimit())
		s.cur.wrapNext = false
	case 'B', 'e':
		s.cur.y = min(s.cur.y+n, s.lowerLimit())
		s.cur.wrapNext = false
	case 'C', 'a':
		s.cur.x = min(s.cur.x+n, s.cols-1)
		s.cur.wrapNext = false
	case 'D':
		s.cur.x = max(s.cur.x-n, 0)
		s.cur.wrapNext = false
	case 'E':
		s.cur.y = min(s.cur.y+n, s.lowerLimit())
		s.cur.x, s.cur.wrapNext = 0, false
	case 'F':
		s.cur.y = max(s.cur.y-n, s.upperLimit())
		s.cur.x, s.cur.wrapNext = 0, false
	case 'G', '`':
		s.cur.x = clamp(n-1, 0, s.cols-1)
		s.cur.wrapNext = false
	case 'H', 'f':
		s.moveTo(p.arg(1, 1)-1, p.arg(0, 1)-1)
	case 'I':
		s.tab(n)
	case 'Z':
		s.tab(-n)
	case 'J':
		s.eraseDisplay(p.raw(0))
	case 'K':
		s.eraseLine(p.raw(0))
	case 'L', 'M': // IL, DL
		if s.cur.y < s.top || s.cur.y > s.bot {
			return
		}
		top := s.top
		s.top = s.cur.y
		if final == 'L' {
			s.scrollDown(n)
		} else {
			s.scrollUpNoHistory(n)
		}
		s.top = top
		s.cur.x, s.cur.wrapNext = 0, false
	case 'P': // DCH
		line := s.lines[s.cur.y].Cells
		n = min(n, s.cols-s.cur.x)
		copy(line[s.cur.x:], line[s.cur.x+n:])
		for i := s.cols - n; i < s.cols; i++ {
			line[i] = s.blank()
		}
	case 'S':
		s.scrollUpNoHistory(n)
	case 'T':
		if len(p.params) <= 1 {
			s.scrollDown(n)
		}
	case 'X': // ECH
		line := s.lines[s.cur.y].Cells
		for i := s.cur.x; i < min(s.cur.x+n, s.cols); i++ {
			s.clearCell(s.cur.y, i)
			line[i] = s.blank()
		}
	case 'b': // REP
		if s.last != 0 {
			for range min(n, s.cols*s.rows) {
				s.print(s.last)
			}
		}
	case 'c':
		if p.raw(0) <= 0 {
			s.send("\x1b[?62;22c") // a VT220 with ANSI colour
		}
	case 'd':
		s.moveTo(s.cur.x, n-1)
	case 'g': // TBC
		switch p.raw(0) {
		case -1, 0:
			s.tabs[s.cur.x] = false
		case 3:
			s.tabs = make([]bool, s.cols)
		}
	case 'h', 'l':
		for i := range p.params {
			switch p.raw(i) {
			case 4:
				s.insert = final == 'h'
			case 20:
				s.lnm = final == 'h'
			}
		}
	case 'm':
		s.sgr()
	case 'n':
		switch p.raw(0) {
		case 5:
			s.send("\x1b[0n")
		case 6:
			y := s.cur.y
			if s.cur.origin {
				y -= s.top
			}
			s.send(fmt.Sprintf("\x1b[%d;%dR", y+1, s.cur.x+1))
		}
	case 'r': // DECSTBM
		top, bot := p.arg(0, 1)-1, p.arg(1, s.rows)-1
		bot = min(bot, s.rows-1)
		if top < bot {
			s.top, s.bot = top, bot
			s.moveTo(0, 0)
		}
	case 's':
		s.saved = s.cur
	case 'u':
		s.restore(s.saved)
	}
}

func (s *Screen) upperLimit() int {
	if s.cur.y >= s.top {
		return s.top
	}
	return 0
}

func (s *Screen) lowerLimit() int {
	if s.cur.y <= s.bot {
		return s.bot
	}
	return s.rows - 1
}

// scrollUpNoHistory scrolls the region without keeping what leaves it: what DL and SU remove
// is gone, not scrolled.
func (s *Screen) scrollUpNoHistory(n int) {
	alt := s.altOn
	s.altOn = true
	s.scrollUp(n)
	s.altOn = alt
}

func (s *Screen) eraseDisplay(mode int) {
	switch mode {
	case -1, 0:
		s.eraseLine(0)
		for y := s.cur.y + 1; y < s.rows; y++ {
			s.lines[y] = blankLine(s.cols, s.blank())
		}
	case 1:
		s.eraseLine(1)
		for y := 0; y < s.cur.y; y++ {
			s.lines[y] = blankLine(s.cols, s.blank())
		}
	case 2:
		for y := range s.rows {
			s.lines[y] = blankLine(s.cols, s.blank())
		}
		if !s.altOn {
			s.dropMarksFrom(s.screenTop()) // the commands erased go with their marks
		}
	case 3:
		s.dropped += len(s.history)
		s.history = nil
		s.dropMarksBefore(s.dropped)
	}
}

func (s *Screen) eraseLine(mode int) {
	line := &s.lines[s.cur.y]
	from, to := s.cur.x, s.cols
	switch mode {
	case 1:
		from, to = 0, s.cur.x+1
	case 2:
		from = 0
	}
	if from > 0 {
		s.clearCell(s.cur.y, from)
	}
	if to < s.cols {
		s.clearCell(s.cur.y, to-1)
	}
	for x := from; x < to; x++ {
		line.Cells[x] = s.blank()
	}
	if to == s.cols {
		line.Wrapped = false
	}
}

func (s *Screen) decMode(mode int, on bool) {
	switch mode {
	case 1:
		s.appCursor = on
	case 6:
		s.cur.origin = on
		s.moveTo(0, 0)
	case 7:
		s.autowrap = on
	case 25:
		s.hidden = !on
	case 47, 1047:
		s.altScreen(on, false)
	case 1048:
		if on {
			s.saved = s.cur
		} else {
			s.restore(s.saved)
		}
	case 1049:
		s.altScreen(on, true)
	case 1000, 1002, 1003:
		if on {
			s.mouse = MouseMode(mode)
		} else {
			s.mouse = MouseOff
		}
	case 1006:
		s.sgrMouse = on
	case 1004:
		s.focusEvents = on
	case 2004:
		s.bracketed = on
	}
}

// altScreen switches to the alternate screen, cleared, and back; with cursor, the cursor is
// saved on the way in and restored on the way out (1049).
func (s *Screen) altScreen(on, cursor bool) {
	if on == s.altOn {
		return
	}
	if on {
		if cursor {
			s.altSaved = s.cur
		}
		s.alt = blankLines(s.cols, s.rows, Cell{Width: 1})
		s.lines, s.altOn = s.alt, true
	} else {
		s.lines, s.altOn = s.main, false
		if cursor {
			s.restore(s.altSaved)
		}
	}
	s.top, s.bot = 0, s.rows-1
}

func (s *Screen) sgr() {
	p := &s.p
	pen := &s.cur.pen
	if len(p.params) == 0 {
		p.params = append(p.params, 0)
		p.sub = append(p.sub, false)
	}
	for i := 0; i < len(p.params); i++ {
		v := p.params[i]
		if v < 0 {
			v = 0
		}
		switch {
		case v == 0:
			pen.FG, pen.BG, pen.A = Default, Default, 0
		case v == 1:
			pen.A |= Bold
		case v == 2:
			pen.A |= Faint
		case v == 3:
			pen.A |= Italic
		case v == 4:
			if i+1 < len(p.params) && p.sub[i+1] { // 4:0 is no underline, 4:n a style
				i++
				if p.params[i] == 0 {
					pen.A &^= Underline
					continue
				}
			}
			pen.A |= Underline
		case v == 5 || v == 6:
			pen.A |= Blink
		case v == 7:
			pen.A |= Inverse
		case v == 8:
			pen.A |= Hidden
		case v == 9:
			pen.A |= Strike
		case v == 21 || v == 24:
			pen.A &^= Underline
		case v == 22:
			pen.A &^= Bold | Faint
		case v == 23:
			pen.A &^= Italic
		case v == 25:
			pen.A &^= Blink
		case v == 27:
			pen.A &^= Inverse
		case v == 28:
			pen.A &^= Hidden
		case v == 29:
			pen.A &^= Strike
		case v >= 30 && v <= 37:
			pen.FG = Indexed(uint8(v - 30))
		case v == 38, v == 48, v == 58:
			c, used := extendedColor(p.params[i+1:], p.sub[i+1:])
			i += used
			switch v {
			case 38:
				pen.FG = c
			case 48:
				pen.BG = c
			}
		case v == 39:
			pen.FG = Default
		case v >= 40 && v <= 47:
			pen.BG = Indexed(uint8(v - 40))
		case v == 49:
			pen.BG = Default
		case v >= 90 && v <= 97:
			pen.FG = Indexed(uint8(v - 90 + 8))
		case v >= 100 && v <= 107:
			pen.BG = Indexed(uint8(v - 100 + 8))
		}
	}
}

// extendedColor reads the colour after 38, 48 or 58: 5;n, 2;r;g;b, or their colon forms
// (2::r:g:b and 2:cs:r:g:b). It returns the colour and how many parameters it used.
func extendedColor(ps []int, sub []bool) (Color, int) {
	if len(ps) == 0 {
		return Default, 0
	}
	byteOf := func(v int) uint8 { return uint8(clamp(v, 0, 255)) }
	switch ps[0] {
	case 5:
		if len(ps) < 2 {
			return Default, len(ps)
		}
		return Indexed(byteOf(ps[1])), 2
	case 2:
		if len(sub) > 0 && sub[0] { // the colon form
			n := countSub(sub[1:])
			switch {
			case n >= 4 && len(ps) >= 5: // 2:cs:r:g:b, the colour space ignored
				return RGB(byteOf(ps[2]), byteOf(ps[3]), byteOf(ps[4])), 1 + n
			case n == 3:
				return RGB(byteOf(ps[1]), byteOf(ps[2]), byteOf(ps[3])), 4
			}
			return Default, 1 + n
		}
		if len(ps) < 4 {
			return Default, len(ps)
		}
		return RGB(byteOf(ps[1]), byteOf(ps[2]), byteOf(ps[3])), 4
	}
	return Default, 1
}

func countSub(sub []bool) int {
	n := 0
	for _, s := range sub {
		if !s {
			break
		}
		n++
	}
	return n
}

func (s *Screen) oscDone() {
	data := string(s.p.str)
	num, rest, ok := cut(data, ';')
	if !ok {
		return
	}
	switch n, _ := strconv.Atoi(num); n {
	case 0, 2:
		if len(rest) > 256 {
			rest = rest[:256]
		}
		s.title = rest
	case 7:
		s.osc7(rest)
	case 133:
		s.osc133(rest)
	}
}

func cut(s string, sep byte) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}
