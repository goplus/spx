/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package gdextensionparser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

type preprocessorVars map[string]struct{}

type preprocessorFile struct {
	Directives []directive `parser:" @@* "`
}

type directive struct {
	Ifndef  *ifndefDirective  `parser:" EOL* ( @@          "`
	Ifdef   *ifdefDirective   `parser:"   | @@        "`
	Define  *defineDirective  `parser:"   | @@        "`
	Include *includeDirective `parser:"   | @@        "`
	Source  string            `parser:"   | @Source ) "`
}

type ifndefDirective struct {
	Name       string      `parser:" @Ifndef EOL      "`
	Directives []directive `parser:" @@* '#endif' EOL "`
}

type ifdefDirective struct {
	Name       string      `parser:" @Ifdef EOL       "`
	Directives []directive `parser:" @@* '#endif' EOL "`
}

type defineDirective struct {
	Name string `parser:" @Define EOL" `
}

type includeDirective struct {
	Name string `parser:" @Include EOL" `
}

func parsePreprocessor(s string) (*preprocessorFile, error) {
	var preprocessorHeaderFileLexer = lexer.MustStateful(lexer.Rules{
		"Root": {
			{Name: `Ifdef`, Pattern: `#ifdef[ \t]+[a-zA-Z_][a-zA-Z0-9_]*`, Action: lexer.Push("Root")},
			{Name: `Ifndef`, Pattern: `#ifndef[ \t]+[a-zA-Z_][a-zA-Z0-9_]*`, Action: lexer.Push("Root")},
			{Name: `Define`, Pattern: `#define[ \t]+[a-zA-Z_][a-zA-Z0-9_]*`},
			{Name: `Include`, Pattern: `#include[ \t]+<[A-Za-z0-9_]+\.h>`},
			{Name: `Endif`, Pattern: `#endif`, Action: lexer.Pop()},
			{Name: `Whitespace`, Pattern: `[ \t]+`},
			{Name: `Comment`, Pattern: `(\/\/[^\n]*)|(\/\*(.|[\r\n])*?\*\/)`},
			{Name: `EOL`, Pattern: `[\n\r]+`},
			{Name: `Source`, Pattern: `[\n\r]*[^#]+[\n\r]*`},
		},
	})

	parser, err := participle.Build[preprocessorFile](
		participle.Lexer(preprocessorHeaderFileLexer),
		participle.Elide("Whitespace", "Comment"),
		directiveIdent("Ifdef", "Ifndef", "Define"),
		directiveFilename("Include"),
	)

	if err != nil {
		return nil, err
	}

	ast, err := parser.ParseString("", s)

	if err != nil {
		return nil, err
	}

	return ast, nil
}

func (f preprocessorFile) eval(isCpp bool) string {
	vars := preprocessorVars{}

	if isCpp {
		vars["__cplusplus"] = struct{}{}
	}

	return evalDirectives(f.Directives, vars)
}

func (d directive) eval(vars preprocessorVars) string {
	if d.Ifndef != nil {
		return d.Ifndef.eval(vars)
	} else if d.Ifdef != nil {
		return d.Ifdef.eval(vars)
	} else if d.Define != nil {
		return d.Define.eval(vars)
	} else if d.Include != nil {
		return d.Include.eval(vars)
	} else {
		return d.Source
	}
}

func (d ifndefDirective) eval(vars preprocessorVars) string {
	if len(d.Name) == 0 {
		panic("#ifndef missing variable")
	}

	if _, ok := vars[d.Name]; !ok {
		return evalDirectives(d.Directives, vars)
	}
	return ""
}

func (d ifdefDirective) eval(vars preprocessorVars) string {
	if len(d.Name) == 0 {
		panic("#ifdef missing variable")
	}

	if _, ok := vars[d.Name]; ok {
		return evalDirectives(d.Directives, vars)
	}
	return ""
}

func (d defineDirective) eval(vars preprocessorVars) string {
	if len(d.Name) == 0 {
		panic("#define missing variable")
	}

	vars[d.Name] = struct{}{}

	return ""
}

func (d includeDirective) eval(vars preprocessorVars) string {
	return ""
}

func evalDirectives(directives []directive, vars preprocessorVars) string {
	sb := strings.Builder{}

	for _, d := range directives {
		sb.WriteString(d.eval(vars))
		sb.WriteString("\n")
	}

	return sb.String()
}

func directiveIdent(types ...string) participle.Option {
	re := regexp.MustCompile("(#[A-Za-z_]+)[ \t]+([a-zA-Z_][a-zA-Z0-9_]*)")
	return participle.Map(func(token lexer.Token) (lexer.Token, error) {
		matches := re.FindAllStringSubmatch(token.Value, 1)

		if len(matches) == 0 {
			panic(fmt.Sprintf("no matches found for DirectiveIdent: %v", token))
		}

		token.Value = matches[0][2]

		return token, nil
	}, types...)
}

func directiveFilename(types ...string) participle.Option {
	re := regexp.MustCompile("(#[A-Za-z_]+)[ \t]+<([a-zA-Z0-9_][a-zA-Z0-9_.]*)>")
	return participle.Map(func(token lexer.Token) (lexer.Token, error) {
		matches := re.FindAllStringSubmatch(token.Value, 1)

		if len(matches) == 0 {
			panic(fmt.Sprintf("no matches found for DirectiveFilename: %v", token))
		}

		token.Value = matches[0][2]

		return token, nil
	}, types...)
}
