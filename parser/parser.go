package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/buildkite/conditional/ast"
	"github.com/buildkite/conditional/lexer"
	"github.com/buildkite/conditional/token"
	"github.com/dlclark/regexp2"
)

const regexpMatchTimeout = time.Second

const (
	_ int = iota
	LOWEST
	TERNARY
	OR     // ||
	AND    // &&
	EQUALS // ==
	PREFIX // !X
	CALL   // myfunction(true)
	DOT    // foo.bar
)

var precedences = map[token.TokenType]int{
	token.EQ:        EQUALS,
	token.NOT_EQ:    EQUALS,
	token.RE_EQ:     EQUALS,
	token.RE_NOT_EQ: EQUALS,
	token.CONTAINS:  EQUALS,
	token.INCLUDES:  EQUALS,
	token.AND:       AND,
	token.OR:        OR,
	token.QUESTION:  TERNARY,
	token.LPAREN:    CALL,
	token.DOT:       DOT,
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

type Parser struct {
	l      *lexer.Lexer
	errors []string

	curToken  token.Token
	peekToken token.Token

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)
	p.registerPrefix(token.IDENT, p.parseIdentifier)
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.STRING, p.parseStringLiteral)
	p.registerPrefix(token.REGEXP, p.parseRegexp)
	p.registerPrefix(token.SHELL, p.parseShellExpansion)
	p.registerPrefix(token.BANG, p.parsePrefixExpression)
	p.registerPrefix(token.TRUE, p.parseBoolean)
	p.registerPrefix(token.FALSE, p.parseBoolean)
	p.registerPrefix(token.NULL, p.parseNull)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(token.LBRACKET, p.parseArrayLiteral)

	p.infixParseFns = make(map[token.TokenType]infixParseFn)
	p.registerInfix(token.EQ, p.parseInfixExpression)
	p.registerInfix(token.NOT_EQ, p.parseInfixExpression)
	p.registerInfix(token.RE_EQ, p.parseInfixExpression)
	p.registerInfix(token.RE_NOT_EQ, p.parseInfixExpression)
	p.registerInfix(token.INCLUDES, p.parseInfixExpression)
	p.registerInfix(token.AND, p.parseInfixExpression)
	p.registerInfix(token.OR, p.parseInfixExpression)
	p.registerInfix(token.QUESTION, p.parseConditionalExpression)
	p.registerInfix(token.LPAREN, p.parseCallExpression)

	// Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	} else {
		p.peekError(t)
		return false
	}
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) peekError(t token.TokenType) {
	msg := fmt.Sprintf("expected next token to be %s, got %s instead",
		t, p.peekToken.Type)
	p.errors = append(p.errors, msg)
}

func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	msg := fmt.Sprintf("no prefix parse function for %s found", t)
	p.errors = append(p.errors, msg)
}

func (p *Parser) Parse() ast.Expression {
	// defer untrace(trace("Parse"))

	if p.curToken.Type == token.EOF {
		p.errors = append(p.errors, "empty expression")
		return nil
	}

	exp := p.parseExpression(LOWEST)
	if exp == nil {
		return nil
	}

	if !p.peekTokenIs(token.EOF) {
		msg := fmt.Sprintf("unexpected token after expression: %s (%q)",
			p.peekToken.Type, p.peekToken.Literal)
		p.errors = append(p.errors, msg)
	}

	return exp
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	// defer untrace(trace("parseExpression", p.curToken))

	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()

	for !p.peekTokenIs(token.EOF) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}

		p.nextToken()

		leftExp = infix(leftExp)
	}

	return leftExp
}

func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}

	return LOWEST
}

func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}

	return LOWEST
}

func (p *Parser) parseIdentifier() ast.Expression {
	// defer untrace(trace("parseIdentifier", p.curToken))
	if invalidDottedIdentifier(p.curToken.Literal) {
		msg := fmt.Sprintf("invalid dotted identifier: %s", p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func invalidDottedIdentifier(value string) bool {
	return strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") || strings.Contains(value, "..")
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	// defer untrace(trace("parseIntegerLiteral"))
	lit := &ast.IntegerLiteral{Token: p.curToken}

	value, err := strconv.ParseInt(p.curToken.Literal, 0, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as integer", p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	lit.Value = value

	return lit
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseShellExpansion() ast.Expression {
	return &ast.ShellExpansion{Token: p.curToken, Raw: p.curToken.Literal}
}

func (p *Parser) parseRegexp() ast.Expression {
	ar := &ast.Regexp{Token: p.curToken, Flags: p.curToken.Flags}

	options, err := regexpOptions(p.curToken.Flags)
	if err != nil {
		p.errors = append(p.errors, err.Error())
		return nil
	}
	if err := validateRegexp(p.curToken.Literal); err != nil {
		p.errors = append(p.errors, err.Error())
		return nil
	}

	r, err := regexp2.Compile(p.curToken.Literal, options)
	if err != nil {
		msg := fmt.Sprintf("could not parse regexp: %v", err)
		p.errors = append(p.errors, msg)
		return nil
	}
	// regexp2 is intentionally used for Buildkite server-side syntax parity.
	// It can backtrack, so keep matching bounded.
	r.MatchTimeout = regexpMatchTimeout

	ar.Regexp = r
	return ar
}

func regexpOptions(flags string) (regexp2.RegexOptions, error) {
	options := regexp2.RegexOptions(regexp2.RE2)
	for _, flag := range flags {
		switch flag {
		case 'i':
			options |= regexp2.IgnoreCase
		default:
			return regexp2.None, fmt.Errorf("unsupported regexp flag: %c", flag)
		}
	}

	return options, nil
}

func validateRegexp(pattern string) error {
	escaped := false
	inClass := false
	classFirst := false

	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		if escaped {
			escaped = false
			if inClass {
				classFirst = false
			}
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if inClass {
			if ch == '[' && i+1 < len(pattern) && isPOSIXClassMarker(pattern[i+1]) {
				if end := regexpClassSetEnd(pattern, i); end != -1 {
					i = end
					classFirst = false
				}
				continue
			}
			if ch == ']' {
				if classFirst {
					classFirst = false
					continue
				}
				inClass = false
				continue
			}
			classFirst = false
			continue
		}
		if ch == '[' {
			inClass = true
			classFirst = true
			continue
		}

		if ch == '(' && i+1 < len(pattern) && pattern[i+1] == '?' {
			if hasRegexpPrefix(pattern, i, "(?#") {
				end := regexpCommentEnd(pattern, i+3)
				if end == -1 {
					return nil
				}
				i = end
				continue
			}
			switch {
			case hasRegexpPrefix(pattern, i, "(?<="):
				return unsupportedRegexpFeature("lookbehind")
			case hasRegexpPrefix(pattern, i, "(?<!"):
				return unsupportedRegexpFeature("nlookbehind")
			case hasRegexpPrefix(pattern, i, "(?>"):
				return unsupportedRegexpFeature("atomic")
			case hasRegexpPrefix(pattern, i, "(?<"):
				return unsupportedRegexpFeature("named_ab")
			case hasRegexpPrefix(pattern, i, "(?P<"):
				return unsupportedRegexpFeature("named_ab")
			case hasRegexpPrefix(pattern, i, "(?'"):
				return unsupportedRegexpFeature("named_sq")
			case hasRegexpPrefix(pattern, i, "(?("):
				return unsupportedRegexpFeature("condition_open")
			}
		}

		if i+1 < len(pattern) && pattern[i+1] == '+' {
			switch ch {
			case '?':
				return unsupportedRegexpFeature("zero_or_one_possessive")
			case '*':
				return unsupportedRegexpFeature("zero_or_more_possessive")
			case '+':
				return unsupportedRegexpFeature("one_or_more_possessive")
			}
		}
	}

	return nil
}

func isPOSIXClassMarker(ch byte) bool {
	return ch == ':' || ch == '.' || ch == '='
}

func regexpClassSetEnd(pattern string, start int) int {
	if start+1 >= len(pattern) {
		return -1
	}
	marker := pattern[start+1]
	for i := start + 2; i+1 < len(pattern); i++ {
		if pattern[i] == marker && pattern[i+1] == ']' {
			return i + 1
		}
	}
	return -1
}

func regexpCommentEnd(pattern string, start int) int {
	for i := start; i < len(pattern); i++ {
		if pattern[i] == ')' {
			return i
		}
	}
	return -1
}

func hasRegexpPrefix(pattern string, offset int, prefix string) bool {
	return len(pattern)-offset >= len(prefix) && pattern[offset:offset+len(prefix)] == prefix
}

func unsupportedRegexpFeature(feature string) error {
	return fmt.Errorf("unsupported regexp feature: %s", feature)
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	// defer untrace(trace("parsePrefixExpression"))

	expression := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}

	p.nextToken()

	expression.Right = p.parseExpression(PREFIX)

	return expression
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	// defer untrace(trace("parseInfixExpression"))

	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()
	expression.Right = p.parseExpression(precedence)

	return expression
}

func (p *Parser) parseConditionalExpression(condition ast.Expression) ast.Expression {
	expression := &ast.ConditionalExpression{
		Token:     p.curToken,
		Condition: condition,
	}

	p.nextToken()
	expression.Consequence = p.parseExpression(LOWEST)

	if !p.expectPeek(token.COLON) {
		return nil
	}

	p.nextToken()
	expression.Alternative = p.parseExpression(TERNARY - 1)

	return expression
}

func (p *Parser) parseBoolean() ast.Expression {
	// defer untrace(trace("parseBoolean"))

	return &ast.Boolean{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
}

func (p *Parser) parseNull() ast.Expression {
	return &ast.Null{Token: p.curToken}
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	// defer untrace(trace("parseGroupedExpression"))

	p.nextToken()

	exp := p.parseExpression(LOWEST)

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return exp
}

func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	defer untrace(trace("parseCallExpression"))

	name, ok := functionName(function)
	if !ok {
		msg := fmt.Sprintf("function call must be an identifier, got %v", p.curToken.Type)
		p.errors = append(p.errors, msg)
		return nil
	}

	exp := &ast.CallExpression{Token: p.curToken, Function: name}
	exp.Arguments = p.parseExpressionList(token.RPAREN)
	return exp
}

func functionName(function ast.Expression) (string, bool) {
	switch function := function.(type) {
	case *ast.Identifier:
		return function.Value, true
	case *ast.InfixExpression:
		if function.Operator != token.DOT {
			return "", false
		}
		left, ok := functionName(function.Left)
		if !ok {
			return "", false
		}
		right, ok := function.Right.(*ast.Identifier)
		if !ok {
			return "", false
		}
		return left + "." + right.Value, true
	default:
		return "", false
	}
}

func (p *Parser) parseExpressionList(end token.TokenType) []ast.Expression {
	list := []ast.Expression{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()
	list = append(list, p.parseExpression(LOWEST))

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		list = append(list, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}

func (p *Parser) parseArrayLiteral() ast.Expression {
	array := &ast.ArrayLiteral{Token: p.curToken}

	array.Elements = p.parseExpressionList(token.RBRACKET)

	return array
}

func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}
