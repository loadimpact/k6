/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */
package compiler

import (
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
)

func TestTransform(t *testing.T) {
	c := New(testutils.NewLogger(t))
	t.Run("blank", func(t *testing.T) {
		src, _, err := c.Transform("", "test.js")
		assert.NoError(t, err)
		assert.Equal(t, `"use strict";`, src)
		// assert.Equal(t, 3, srcmap.Version)
		// assert.Equal(t, "test.js", srcmap.File)
		// assert.Equal(t, "", srcmap.Mappings)
	})
	t.Run("double-arrow", func(t *testing.T) {
		src, _, err := c.Transform("()=> true", "test.js")
		assert.NoError(t, err)
		assert.Equal(t, `"use strict";(function () {return true;});`, src)
		// assert.Equal(t, 3, srcmap.Version)
		// assert.Equal(t, "test.js", srcmap.File)
		// assert.Equal(t, "aAAA,qBAAK,IAAL", srcmap.Mappings)
	})
	t.Run("longer", func(t *testing.T) {
		src, _, err := c.Transform(strings.Join([]string{
			`function add(a, b) {`,
			`    return a + b;`,
			`};`,
			``,
			`let res = add(1, 2);`,
		}, "\n"), "test.js")
		assert.NoError(t, err)
		assert.Equal(t, strings.Join([]string{
			`"use strict";function add(a, b) {`,
			`    return a + b;`,
			`};`,
			``,
			`let res = add(1, 2);`,
		}, "\n"), src)
		// assert.Equal(t, 3, srcmap.Version)
		// assert.Equal(t, "test.js", srcmap.File)
		// assert.Equal(t, "aAAA,SAASA,GAAT,CAAaC,CAAb,EAAgBC,CAAhB,EAAmB;AACf,WAAOD,IAAIC,CAAX;AACH;;AAED,IAAIC,MAAMH,IAAI,CAAJ,EAAO,CAAP,CAAV", srcmap.Mappings)
	})

	t.Run("double-arrow with sourceMap", func(t *testing.T) {
		c.COpts.SourceMapEnabled = true
		src, _, err := c.Transform("()=> true", "test.js")
		assert.NoError(t, err)
		assert.Equal(t, `"use strict";(function () {return true;});
//# sourceMappingURL=data:application/json;charset=utf-8;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbInRlc3QuanMiXSwibmFtZXMiOltdLCJtYXBwaW5ncyI6ImFBQUEscUJBQUssSUFBTCIsImZpbGUiOiJ0ZXN0LmpzIiwic291cmNlc0NvbnRlbnQiOlsiKCk9PiB0cnVlIl19`, src)
		// assert.Equal(t, 3, srcmap.Version)
		// assert.Equal(t, "test.js", srcmap.File)
		// assert.Equal(t, "aAAA,qBAAK,IAAL", srcmap.Mappings)
	})
}

func TestCompile(t *testing.T) {
	c := New(testutils.NewLogger(t))
	t.Run("ES5", func(t *testing.T) {
		src := `1+(function() { return 2; })()`
		pgm, code, err := c.Compile(src, "script.js", true, c.COpts)
		require.NoError(t, err)
		assert.Equal(t, src, code)
		v, err := goja.New().RunProgram(pgm)
		if assert.NoError(t, err) {
			assert.Equal(t, int64(3), v.Export())
		}

		t.Run("Wrap", func(t *testing.T) {
			src := `exports.d=1+(function() { return 2; })()`
			pgm, code, err := c.Compile(src, "script.js", false, c.COpts)
			require.NoError(t, err)
			assert.Equal(t, "(function(module, exports){\nexports.d=1+(function() { return 2; })()\n})\n", code)
			rt := goja.New()
			v, err := rt.RunProgram(pgm)
			if assert.NoError(t, err) {
				fn, ok := goja.AssertFunction(v)
				if assert.True(t, ok, "not a function") {
					exp := make(map[string]goja.Value)
					_, err := fn(goja.Undefined(), goja.Undefined(), rt.ToValue(exp))
					if assert.NoError(t, err) {
						assert.Equal(t, int64(3), exp["d"].Export())
					}
				}
			}
		})

		t.Run("Invalid", func(t *testing.T) {
			src := `1+(function() { return 2; )()`
			c.COpts.CompatibilityMode = lib.CompatibilityModeExtended
			_, _, err := c.Compile(src, "script.js", false, c.COpts)
			assert.IsType(t, &goja.Exception{}, err)
			assert.Contains(t, err.Error(), `SyntaxError: script.js: Unexpected token (1:26)
> 1 | 1+(function() { return 2; )()`)
		})
	})
	t.Run("ES6", func(t *testing.T) {
		c.COpts.CompatibilityMode = lib.CompatibilityModeExtended
		pgm, code, err := c.Compile(`1+(()=>2)()`, "script.js", true, c.COpts)
		require.NoError(t, err)
		assert.Equal(t, `"use strict";1 + function () {return 2;}();`, code)
		v, err := goja.New().RunProgram(pgm)
		if assert.NoError(t, err) {
			assert.Equal(t, int64(3), v.Export())
		}

		t.Run("Wrap", func(t *testing.T) {
			pgm, code, err := c.Compile(`exports.fn(1+(()=>2)())`, "script.js", false, c.COpts)
			require.NoError(t, err)
			assert.Equal(t, "(function(module, exports){\n\"use strict\";exports.fn(1 + function () {return 2;}());\n})\n", code)
			rt := goja.New()
			v, err := rt.RunProgram(pgm)
			if assert.NoError(t, err) {
				fn, ok := goja.AssertFunction(v)
				if assert.True(t, ok, "not a function") {
					exp := make(map[string]goja.Value)
					var out interface{}
					exp["fn"] = rt.ToValue(func(v goja.Value) {
						out = v.Export()
					})
					_, err := fn(goja.Undefined(), goja.Undefined(), rt.ToValue(exp))
					assert.NoError(t, err)
					assert.Equal(t, int64(3), out)
				}
			}
		})

		t.Run("Invalid", func(t *testing.T) {
			_, _, err := c.Compile(`1+(=>2)()`, "script.js", true, c.COpts)
			assert.IsType(t, &goja.Exception{}, err)
			assert.Contains(t, err.Error(), `SyntaxError: script.js: Unexpected token (1:3)
> 1 | 1+(=>2)()`)
		})

		t.Run("Invalid for goja but not babel", func(t *testing.T) {
			t.Skip("Find something else that breaks this as this was fixed in goja :(")
			ch := make(chan struct{})
			go func() {
				defer close(ch)
				// This is a string with U+2029 Paragraph separator in it
				// the important part is that goja won't parse it but babel will transform it but still
				// goja won't be able to parse the result it is actually "\<U+2029>"
				_, _, err := c.Compile(string([]byte{0x22, 0x5c, 0xe2, 0x80, 0xa9, 0x22}), "script.js", true, c.COpts)
				assert.IsType(t, parser.ErrorList{}, err)
				assert.Contains(t, err.Error(), ` Unexpected token ILLEGAL`)
			}()

			select {
			case <-ch:
				// everything is fine
			case <-time.After(time.Second):
				// it took too long
				t.Fatal("takes too long")
			}
		})
	})
}
