package token

type TokenType string

const (
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"

	// Identifiers + literals
	IDENT  = "IDENT"  // add, foobar, x, y, ...
	INT    = "INT"    // 1343456
	STRING = "STRING" // "foobar"
	REGEX  = "REGEX"  // /^v1.0/

	PLUS     = "+"
	MINUS    = "-"
	BANG     = "!"
	ASTERISK = "*"

	// Comparison Operators
	LT     = "<"
	GT     = ">"
	EQ     = "=="
	NOT_EQ = "!="
	RE_EQ  = "=~"

	// Boolean Operators
	AND = "&&"
	OR  = "||"

	// Delimiters
	COMMA     = ","
	SEMICOLON = ";"

	LPAREN = "("
	RPAREN = ")"

	// Keywords
	TRUE  = "TRUE"
	FALSE = "FALSE"
)

type Token struct {
	Type    TokenType
	Literal string
}

var keywords = map[string]TokenType{
	"true":  TRUE,
	"false": FALSE,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
