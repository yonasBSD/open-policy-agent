package test

// This file collects some helpers for generating data used in
// benchmarks,
// - topdown/topdown_bench_test.go

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"
)

// PartialObjectBenchmarkCrossModule returns a module with n "bench_test_" prefixed rules
// that each refer to another "cond_bench_" prefixed rule
func PartialObjectBenchmarkCrossModule(n int) []string {
	fooMod := `package test.foo
    import data.test.bar
	import data.test.baz

	output[key] := value if {
		value := bar[key]
		startswith("bench_test_", key)
	}`
	barMod := "package test.bar\n"
	barMod += `
	cond_bench_0 if {
		contains(lower(input.test_input_0), lower("input_01"))
	}
	cond_bench_1 if {
		contains(lower(input.test_input_1), lower("input"))
	}
	cond_bench_2 if {
		contains(lower(input.test_input_2), lower("input_10"))
	}
    bench_test_out_result := load_tests(test_collector)

    load_tests(i) := out if {
		out := i
	}
    `

	bazMod := "package test.baz\nimport data.test.bar\n"
	ruleBuilder := ""

	for idx := 1; idx <= n; idx++ {
		barMod += fmt.Sprintf(`
		bench_test_%[1]d := result if {
            input.bench_test_collector_mambo_number_%[3]d
			result := input.bench_test_collector_mambo_number_%[3]d
        } else := result if {
			is_null(bench_test_out_result.mambo_number_%[3]d.error)
			result := bench_test_out_result.mambo_number_%[3]d.result
		}

        test_collector["mambo_number_%[3]d"] := result if {
			cond_bench_%[2]d
			not %[3]d == 2
			not %[3]d == 3
			not input.bench_test_collector_mambo_number_%[3]d
			result := { "result": %[3]d, "error": null }
		}
		`, idx, idx%3, idx%5)
		ruleBuilder += fmt.Sprintf("    bar.bench_test_%[1]d == %[1]d\n", idx)
		if idx%10 == 0 {
			bazMod += fmt.Sprintf(`rule_%d if {
				%s
			}`, idx, ruleBuilder)
			fooMod += fmt.Sprintf(`
			final_decision = "allow" if {
				baz.rule_%d
			}
			`, idx)
			ruleBuilder = ""
		}
	}

	return []string{fooMod, barMod, bazMod}
}

// ArrayIterationBenchmarkModule returns a module that iterates an array
// with `n` elements
func ArrayIterationBenchmarkModule(n int) string {
	return fmt.Sprintf(`package test

	fixture = [ x | x := numbers.range(1, %d)[_] ]

	main if { fixture[i] }`, n)
}

// SetIterationBenchmarkModule returns a module that iterates a set
// with `n` elements
func SetIterationBenchmarkModule(n int) string {
	return fmt.Sprintf(`package test

	fixture = { x | x := numbers.range(1, %d)[_] }

	main if { fixture[i] }`, n)
}

// ObjectIterationBenchmarkModule returns a module that iterates an object
// with `n` key/val pairs
func ObjectIterationBenchmarkModule(n int) string {
	return fmt.Sprintf(`package test

	fixture = { x: x | x := numbers.range(1, %d)[_] }

	main if { fixture[i] }`, n)
}

// GenerateLargeJSONBenchmarkData returns a map of 100 keys and 100.000 key/value
// pairs.
func GenerateLargeJSONBenchmarkData() map[string]any {
	return GenerateJSONBenchmarkData(100, 100*1000)
}

// GenerateJSONBenchmarkData returns a map of `k` keys and `v` key/value pairs.
func GenerateJSONBenchmarkData(k, v int) map[string]any {

	// create array of null values that can be iterated over
	keys := make([]any, k)
	for i := range keys {
		keys[i] = nil
	}

	// create large JSON object value (100,000 entries is about 2MB on disk)
	values := map[string]any{}
	for i := range v {
		values[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}

	return map[string]any{
		"keys":   keys,
		"values": values,
	}
}

// GenerateConcurrencyBenchmarkData returns a module and data; the module
// checks some input parameters against that data in a simple API authz
// scheme.
func GenerateConcurrencyBenchmarkData() (string, map[string]any) {
	obj := []byte(`
		{
			"objs": [
				{
					"attr1": "get",
					"path": "/foo/bar",
					"user": "bob"
				},
				{
					"attr1": "set",
					"path": "/foo/bar/baz",
					"user": "alice"
				},
				{
					"attr1": "get",
					"path": "/foo",
					"groups": [
						"admin",
						"eng"
					]
				},
				{
					"path": "/foo/bar",
					"user": "alice"
				}
			]
		}
		`)

	var data map[string]any
	if err := json.Unmarshal(obj, &data); err != nil {
		panic(err)
	}
	mod := `package test

	import data.objs

	p if {
		objs[i].attr1 = "get"
		objs[i].groups[j] = "eng"
	}

	p if {
		objs[i].user = "alice"
	}
	`

	return mod, data
}

// GenerateVirtualDocsBenchmarkData generates a module and input; the
// numTotalRules and numHitRules create as many rules in the module to
// match/miss the returned input.
func GenerateVirtualDocsBenchmarkData(numTotalRules, numHitRules int) (string, map[string]any) {

	hitRule := `
	allow if {
		input.method = "POST"
		input.path = ["accounts", account_id]
		input.user_id = account_id
	}
	`

	missRule := `
	allow if {
		input.method = "GET"
		input.path = ["salaries", account_id]
		input.user_id = account_id
	}
	`

	testModuleTmpl := `package a.b.c

	{{range .MissRules }}
		{{ . }}
	{{end}}

	{{range .HitRules }}
		{{ . }}
	{{end}}
	`

	tmpl, err := template.New("Test").Parse(testModuleTmpl)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer

	var missRules []string

	if numTotalRules > numHitRules {
		missRules = make([]string, numTotalRules-numHitRules)
		for i := range missRules {
			missRules[i] = missRule
		}
	}

	hitRules := make([]string, numHitRules)
	for i := range hitRules {
		hitRules[i] = hitRule
	}

	params := struct {
		MissRules []string
		HitRules  []string
	}{
		MissRules: missRules,
		HitRules:  hitRules,
	}

	err = tmpl.Execute(&buf, params)
	if err != nil {
		panic(err)
	}

	input := map[string]any{
		"path":    []any{"accounts", "alice"},
		"method":  "POST",
		"user_id": "alice",
	}

	return buf.String(), input
}
