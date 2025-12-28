package textrep

import (
    "fmt"
    "strconv"
    "strings"
    "unicode"
    "unicode/utf8"
)

type tokKind int

const (
    tokEOF tokKind = iota
    tokIdent
    tokInt
    tokFloat
    tokString
    tokLet
    tokStruct
    tokEnum
    tokTS
    tokArray
    tokMap
    tokNull
    tokTrue
    tokFalse
    tokNone
    // symbols
    tokEq     // =
    tokColon  // :
    tokSemi   // ;
    tokComma  // ,
    tokLBrace // {
    tokRBrace // }
    tokLBrack // [
    tokRBrack // ]
    tokLParen // (
    tokRParen // )
    tokLt     // <
    tokGt     // >
)

type token struct {
    kind    tokKind
    lit     string
    intBase int    // 10 or 16 for tokInt
}

type lexer struct {
    src []byte
    off int
    cur token
}

func newLexer(src []byte) *lexer { return &lexer{src: src} }

func (lx *lexer) next() {
    lx.skipSpaceAndComments()
    if lx.off >= len(lx.src) {
        lx.cur = token{kind: tokEOF}
        return
    }
    b := lx.src[lx.off]
    // identifiers/keywords
    if isIdentStart(b) {
        start := lx.off
        lx.off++
        for lx.off < len(lx.src) && isIdentPart(lx.src[lx.off]) {
            lx.off++
        }
        s := string(lx.src[start:lx.off])
        switch s {
        case "let":
            lx.cur = token{kind: tokLet, lit: s}
        case "struct":
            lx.cur = token{kind: tokStruct, lit: s}
        case "enum":
            lx.cur = token{kind: tokEnum, lit: s}
        case "ts":
            lx.cur = token{kind: tokTS, lit: s}
        case "array":
            lx.cur = token{kind: tokArray, lit: s}
        case "map":
            lx.cur = token{kind: tokMap, lit: s}
        case "null":
            lx.cur = token{kind: tokNull, lit: s}
        case "true":
            lx.cur = token{kind: tokTrue, lit: s}
        case "false":
            lx.cur = token{kind: tokFalse, lit: s}
        case "none":
            lx.cur = token{kind: tokNone, lit: s}
        default:
            lx.cur = token{kind: tokIdent, lit: s}
        }
        return
    }
    // numbers
    if isDigit(b) || (b == '-' && lx.peekIsDigit()) {
        start := lx.off
        lx.off++
        // hex prefix
        if lx.off < len(lx.src) && (lx.src[start] == '0' && (lx.src[lx.off] == 'x' || lx.src[lx.off] == 'X')) {
            lx.off++
            for lx.off < len(lx.src) && isHexDigit(lx.src[lx.off]) {
                lx.off++
            }
            // optional underscore separators
            for lx.off < len(lx.src) && (isHexDigit(lx.src[lx.off]) || lx.src[lx.off] == '_') {
                lx.off++
            }
            lit := string(lx.src[start:lx.off])
            lx.cur = token{kind: tokInt, lit: lit, intBase: 16}
            return
        }
        // float or dec int
        isFloat := false
        for lx.off < len(lx.src) && (isDigit(lx.src[lx.off]) || lx.src[lx.off] == '_') {
            lx.off++
        }
        if lx.off < len(lx.src) && lx.src[lx.off] == '.' {
            isFloat = true
            lx.off++
            for lx.off < len(lx.src) && (isDigit(lx.src[lx.off]) || lx.src[lx.off] == '_') {
                lx.off++
            }
            // exponent part
            if lx.off < len(lx.src) && (lx.src[lx.off] == 'e' || lx.src[lx.off] == 'E') {
                lx.off++
                if lx.off < len(lx.src) && (lx.src[lx.off] == '+' || lx.src[lx.off] == '-') {
                    lx.off++
                }
                for lx.off < len(lx.src) && isDigit(lx.src[lx.off]) {
                    lx.off++
                }
            }
        }
        lit := string(lx.src[start:lx.off])
        if isFloat {
            lx.cur = token{kind: tokFloat, lit: lit}
        } else {
            lx.cur = token{kind: tokInt, lit: lit, intBase: 10}
        }
        return
    }
    // strings
    if b == '"' {
        s, n, err := scanString(lx.src[lx.off:])
        if err != nil {
            lx.cur = token{kind: tokEOF, lit: fmt.Sprintf("string error: %v", err)}
            lx.off = len(lx.src)
            return
        }
        lx.cur = token{kind: tokString, lit: s}
        lx.off += n
        return
    }
    // single-char tokens
    switch b {
    case '=':
        lx.off++
        lx.cur = token{kind: tokEq, lit: "="}
    case ':':
        lx.off++
        lx.cur = token{kind: tokColon, lit: ":"}
    case ';':
        lx.off++
        lx.cur = token{kind: tokSemi, lit: ";"}
    case ',':
        lx.off++
        lx.cur = token{kind: tokComma, lit: ","}
    case '{':
        lx.off++
        lx.cur = token{kind: tokLBrace, lit: "{"}
    case '}':
        lx.off++
        lx.cur = token{kind: tokRBrace, lit: "}"}
    case '[':
        lx.off++
        lx.cur = token{kind: tokLBrack, lit: "["}
    case ']':
        lx.off++
        lx.cur = token{kind: tokRBrack, lit: "]"}
    case '(':
        lx.off++
        lx.cur = token{kind: tokLParen, lit: "("}
    case ')':
        lx.off++
        lx.cur = token{kind: tokRParen, lit: ")"}
    case '<':
        lx.off++
        lx.cur = token{kind: tokLt, lit: "<"}
    case '>':
        lx.off++
        lx.cur = token{kind: tokGt, lit: ">"}
    default:
        // unknown rune
        lx.off++
        lx.cur = token{kind: tokEOF, lit: fmt.Sprintf("unexpected char %q", b)}
    }
}

func (lx *lexer) skipSpaceAndComments() {
    for lx.off < len(lx.src) {
        b := lx.src[lx.off]
        // whitespace
        if b == ' ' || b == '\t' || b == '\n' || b == '\r' { lx.off++; continue }
        // line comments: # or //
        if b == '#' || (b == '/' && lx.off+1 < len(lx.src) && lx.src[lx.off+1] == '/') {
            // consume to end of line
            for lx.off < len(lx.src) && lx.src[lx.off] != '\n' { lx.off++ }
            continue
        }
        // block comments: /* ... */
        if b == '/' && lx.off+1 < len(lx.src) && lx.src[lx.off+1] == '*' {
            lx.off += 2
            for lx.off+1 < len(lx.src) && !(lx.src[lx.off] == '*' && lx.src[lx.off+1] == '/') { lx.off++ }
            if lx.off+1 < len(lx.src) { lx.off += 2 }
            continue
        }
        break
    }
}

func isIdentStart(b byte) bool { return b == '_' || b == '$' || unicode.IsLetter(rune(b)) }
func isIdentPart(b byte) bool  { return isIdentStart(b) || unicode.IsDigit(rune(b)) }
func isDigit(b byte) bool      { return '0' <= b && b <= '9' }
func isHexDigit(b byte) bool   { return ('0' <= b && b <= '9') || ('a' <= b && b <= 'f') || ('A' <= b && b <= 'F') }
func (lx *lexer) peekIsDigit() bool {
    if lx.off+1 >= len(lx.src) { return false }
    return isDigit(lx.src[lx.off+1])
}

func scanString(src []byte) (string, int, error) {
    // src begins with '"'
    i := 1
    // find ending quote while allowing escapes
    for i < len(src) {
        c := src[i]
        if c == '"' { i++; s := string(src[:i]); unq, err := strconv.Unquote(s); return unq, i, err }
        if c == '\\' {
            i++
            if i >= len(src) { return "", 0, fmt.Errorf("unterminated escape") }
        } else {
            if c < utf8.RuneSelf {
                i++
            } else {
                _, size := utf8.DecodeRune(src[i:])
                if size <= 0 { return "", 0, fmt.Errorf("invalid utf-8") }
                i += size
            }
        }
    }
    return "", 0, fmt.Errorf("unterminated string")
}

func stripUnderscores(s string) string { return strings.ReplaceAll(s, "_", "") }

