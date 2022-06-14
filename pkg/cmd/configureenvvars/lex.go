package configureenvvars

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type itemType int

const (
	itemError itemType = iota
	itemKey
	itemValue
	itemEquals
	itemSemiColon
	itemNewline
	itemEOF
)

type item struct {
	typ itemType
	val string
}

func (i item) String() string {
	switch {
	case i.typ == itemEOF:
		return "EOF"
	case i.typ == itemError:
		return i.val
	}
	return fmt.Sprintf("<%s>", i.val)
}

// a function that returns a statefn
type stateFn func(*lexer) stateFn

type lexer struct {
	name  string    // used in error reports
	input string    // string being scanned
	start int       // start position of this item
	pos   int       // current position of this item
	width int       // width of the last rune read
	items chan item // last scanned item
	state stateFn
}

func lex(name, input string) *lexer {
	l := &lexer{
		name:  name,
		input: input,
		state: lexText,
		items: make(chan item, 16),
	}
	go l.run() // concurrently begin lexing
	return l   // return lexer and channel that tokens will be sent
}

func (l *lexer) nextItem() item {
	return <-l.items
}

func (l *lexer) run() {
	for state := lexText; state != nil; {
		state = state(l)
	}
	close(l.items) // no more tokens will be delivered
}

func (l *lexer) emit(t itemType) {
	l.items <- item{t, l.input[l.start:l.pos]}
	l.start = l.pos
}

const eof = -1

// next returns the next rune in the input.
func (l *lexer) next() rune {
	if int(l.pos) >= len(l.input) {
		l.width = 0
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.width = w
	l.pos += l.width
	return r
}

// peek returns but does not consume the next rune in the input.
func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune. Can only be called once per call of next.
func (l *lexer) backup() {
	l.pos -= l.width
}

func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- item{
		itemError,
		fmt.Sprintf(format, args...),
	}
	return nil
}

const equalPrefix = "="

func lexText(l *lexer) stateFn {
	for {
		if strings.HasPrefix(l.input[l.pos:], equalPrefix) {
			return lexKey // next state
		}
		if l.next() == eof {
			break
		}
	}
	l.emit(itemEOF)
	return nil
}

const exportPrefix = "export "

func lexKey(l *lexer) stateFn {
	if strings.HasPrefix(l.input[l.start:l.pos], exportPrefix) {
		l.start = l.start + len(exportPrefix)
	}
	l.emit(itemKey)
	return lexEquals
}

func lexEquals(l *lexer) stateFn {
	l.next()
	l.emit(itemEquals)
	return lexValue
}

func lexSemiColon(l *lexer) stateFn {
	l.next()
	l.emit(itemSemiColon)
	return lexText
}

func lexNewline(l *lexer) stateFn {
	l.next()
	l.emit(itemNewline)
	return lexText
}

const (
	semicolon = ";"
	newline   = "\n"
)

func lexValue(l *lexer) stateFn {
	for {
		if strings.HasPrefix(l.input[l.pos:], semicolon) {
			l.emit(itemValue)
			if l.next() == eof {
				l.emit(itemEOF)
				return nil
			}
			return lexSemiColon
		}
		if strings.HasPrefix(l.input[l.pos:], newline) {
			if l.next() == eof {
				l.emit(itemEOF)
				return nil
			}
			return lexNewline

		}
		if l.next() == eof {
			l.emit(itemValue)
			l.emit(itemEOF)
			return nil
		}
	}
}
