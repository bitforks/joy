package translator

import (
	"fmt"
	"go/ast"
	"strconv"
	"strings"

	"github.com/matthewmueller/joy/internal/compiler/util"

	"github.com/apex/log"
	"github.com/matthewmueller/joy/internal/compiler/def"
	"github.com/matthewmueller/joy/internal/compiler/scope"
	"github.com/matthewmueller/joy/internal/jsast"
	"github.com/pkg/errors"
)

// Rewrite fn
func (tr *Translator) Rewrite(rewrite def.Rewrite, def def.Definition, sp *scope.Scope, caller ast.Expr, args ...ast.Expr) (jsast.IExpression, error) {
	// ignore nil values
	if rewrite == nil {
		return nil, nil
	}

	rewritee := rewrite.Rewritee()
	expr := rewrite.Expression()
	vars := rewrite.Vars()

	// handle the caller
	// can be nil when it's just a regular function e.g. test()
	c, e := tr.callerToString(def, sp, caller)
	if e != nil {
		return nil, e
	}
	expr = strings.Replace(expr, "$_", c, -1)

	// get the top function params
	argmap := map[string]int{}
	last := -1
	for i, param := range rewritee.Params() {
		log.Debugf("fn=%s param=%s", rewritee.ID(), param)
		argmap[param] = i
		last = i
	}

	// we may still have $1 even if we have no vars
	// in that case, we'll use the parameters
	if len(vars) == 0 {
		for i, arg := range args {
			// handle any other argument
			v, e := tr.expression(def, sp, arg)
			if e != nil {
				return nil, e
			}
			value, ok := v.(fmt.Stringer)
			if !ok {
				return nil, errors.New("translator/rewrite: expected argument to be a stringer")
			}
			expr = strings.Replace(expr, "$"+strconv.Itoa(i+1), value.String(), -1)
		}
		return jsast.CreateRaw(expr), nil
	}

	// handle the arguments
	for i, v := range vars {
		key, err := v.String()
		if err != nil {
			return nil, err
		}

		// handle arguments coming from the function
		nth, isset := argmap[key]
		if !isset {
			v, err := tr.expression(v.Definition(), nil, v.Node())
			if err != nil {
				return nil, err
			}
			value, ok := v.(fmt.Stringer)
			if !ok {
				return nil, errors.New("translator/rewrite: expected argument to be a stringer")
			}
			expr = strings.Replace(expr, "$"+strconv.Itoa(i+1), value.String(), -1)
			continue
		}

		// handle variadic arguments
		if nth == last && rewritee.IsVariadic() {
			arr := []string{}
			for _, arg := range args[nth:] {
				v, e := tr.expression(def, sp, arg)
				if e != nil {
					return nil, e
				}
				value, ok := v.(fmt.Stringer)
				if !ok {
					return nil, errors.New("translator/rewrite: expected argument to be a stringer")
				}
				arr = append(arr, value.String())
			}
			expr = strings.Replace(expr, "$"+strconv.Itoa(i+1), "["+strings.Join(arr, ", ")+"]", -1)
			continue
		}

		// handle any other argument
		v, e := tr.expression(def, sp, args[nth])
		if e != nil {
			return nil, e
		}
		value, ok := v.(fmt.Stringer)
		if !ok {
			return nil, errors.New("translator/rewrite: expected argument to be a stringer")
		}
		expr = strings.Replace(expr, "$"+strconv.Itoa(i+1), value.String(), -1)
	}

	return jsast.CreateRaw(expr), nil
}

func (tr *Translator) callerToString(d def.Definition, sp *scope.Scope, n ast.Node) (string, error) {
	xc, err := util.GetExprCaller(n)
	if err != nil {
		return "", err
	} else if xc == nil {
		return "", nil
	}

	c, e := tr.expression(d, sp, xc)
	if e != nil {
		return "", e
	} else if c == nil {
		return "", nil
	}

	s, ok := c.(fmt.Stringer)
	if !ok {
		return "", fmt.Errorf("maybeJSRewrite: expression not a fmt.Stringer")
	}

	return s.String(), nil
}
