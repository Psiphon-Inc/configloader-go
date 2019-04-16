package psiconfig

import (
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
)

/*
func TestMetadata_IsDefined(t *testing.T) {
	type test struct {
		name    string
		toml    string
		strct   interface{}
		argKey  []string
		want    bool
		wantErr bool
	}
	var tests []test

	//------------------------------------
	tst := test{}
	tst.name = "simple match: struct path"
	tst.toml = `
	a = "aa"
	`
	tst.strct = &struct {
		A string
	}{}
	tst.argKey = []string{"A"}
	tst.want = true
	tst.wantErr = false
	tests = append(tests, tst)
	//------------------------------------
	tst = test{}
	tst.name = "simple match: toml path"
	tst.toml = `
	a = "aa"
	`
	tst.strct = &struct {
		A string
	}{}
	tst.argKey = []string{"a"}
	tst.want = true
	tst.wantErr = false
	tests = append(tests, tst)
	//------------------------------------
	tst = test{}
	tst.name = "simple match: alias struct"
	tst.toml = `
	apple = "aa"
	`
	tst.strct = &struct {
		A string `toml:"apple"`
	}{}
	tst.argKey = []string{"A"}
	tst.want = true
	tst.wantErr = false
	tests = append(tests, tst)
	//------------------------------------
	tst = test{}
	tst.name = "simple match: alias toml"
	tst.toml = `
	apple = "aa"
	`
	tst.strct = &struct {
		A string `toml:"apple"`
	}{}
	tst.argKey = []string{"apple"}
	tst.want = true
	tst.wantErr = false
	tests = append(tests, tst)
	//------------------------------------
	tst = test{}
	tst.name = "simple non-match"
	tst.toml = `
	x = "aa"
	`
	tst.strct = &struct {
		A string
	}{}
	tst.argKey = []string{"A"}
	tst.want = false
	tst.wantErr = false
	tests = append(tests, tst)
	//------------------------------------
	tst = test{}
	tst.name = "complex"
	tst.toml = `
	[sect1]
	a = "a1"
	[sect2]
	a = "a2"
	[sect2.1]
	a = "a2.1"
	b = "b2.1"
	`
	tst.strct = &struct {
		Sect1 struct {
			A string
		}
		Sect2 struct {
			A       string
			Sect2_1 struct {
				A string
				B string
			} `toml:"1"`
		}
	}{}
	tst.argKey = []string{"Sect2", "Sect2_1", "B"}
	tst.want = true
	tst.wantErr = false
	tests = append(tests, tst)
	//------------------------------------

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tomlMD, err := toml.Decode(tt.toml, tt.strct)
			if err != nil {
				t.Fatalf("toml.Decode failed: %v", err)
			}

			md := Metadata{
				tomlMD:           &tomlMD,
				configStructKeys: structKeys(tt.strct),
			}

			got, err := md.IsDefined(tt.argKey...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Metadata.IsDefined() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Metadata.IsDefined() = %v, want %v", got, tt.want)
			}
		})
	}
}
*/

/*
func Test_setMapByKey(t *testing.T) {
	type args struct {
		m map[string]interface{}
		k Key
		v interface{}
	}
	type test struct {
		name    string
		args    args
		wantErr bool
	}
	tests := make([]test, 0)

	checker := func(args args) bool {
		m := args.m
		for i := range args.k {
			if i == len(args.k)-1 {
				// Last key element
				return m[args.k[i]] == args.v
			}
			// If this panics, things are horribly wrong
			m = m[args.k[i]].(map[string]interface{})
		}
		return false
	}

	tst := test{}
	tst.name = "simple"
	tst.args.m = map[string]interface{}{}
	tst.args.k = Key{"a", "b", "c"}
	tst.args.v = "val"
	tst.wantErr = false
	tests = append(tests, tst)
	//-----------------------------------------------------------------------
	tst = test{}
	tst.name = "overwrite"
	tst.args.m = map[string]interface{}{
		"a": "initial",
	}
	tst.args.k = Key{"a"}
	tst.args.v = "val"
	tst.wantErr = false
	tests = append(tests, tst)
	//-----------------------------------------------------------------------
	tst = test{}
	tst.name = "nil intermediate"
	tst.args.m = map[string]interface{}{
		"a": nil,
	}
	tst.args.k = Key{"a", "b"}
	tst.args.v = "val"
	tst.wantErr = false
	tests = append(tests, tst)
	//-----------------------------------------------------------------------
	tst = test{}
	tst.name = "error"
	tst.args.m = map[string]interface{}{
		"a": "initial",
	}
	tst.args.k = Key{"a", "b"}
	tst.args.v = "val"
	tst.wantErr = true
	tests = append(tests, tst)
	//-----------------------------------------------------------------------

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := setMapByKey(tt.args.m, tt.args.k, tt.args.v)
			if err != nil != tt.wantErr {
				t.Fatalf("setMapByKey() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil {
				return
			}

			if !checker(tt.args) {
				t.Fatalf("value not set properly; v=%#v; m=%#v", tt.args.v, tt.args.m)
			}
		})
	}
}
*/

/*
func Test_setMapLeafFromMap(t *testing.T) {
	type args struct {
		fromMap map[string]interface{}
		toMap   map[string]interface{}
		k       Key
	}
	type test struct {
		name      string
		args      args
		want      bool
		wantToMap map[string]interface{}
	}
	tests := make([]test, 0)

	tst := test{}
	tst.name = "simple"
	tst.args.fromMap = map[string]interface{}{
		"a": "aa",
	}
	tst.args.toMap = map[string]interface{}{}
	tst.args.k = Key{"a"}
	tst.want = true
	tst.wantToMap = map[string]interface{}{
		"a": "aa",
	}
	tests = append(tests, tst)
	//--------------------------------------------------------------------
	tst = test{}
	tst.name = "overwrite"
	tst.args.fromMap = map[string]interface{}{
		"a": "aa",
		"b": "bb",
	}
	tst.args.toMap = map[string]interface{}{
		"a": "initial-a",
		"b": "initial-b",
	}
	tst.args.k = Key{"a"}
	tst.want = true
	tst.wantToMap = map[string]interface{}{
		"a": "aa",
		"b": "initial-b",
	}
	tests = append(tests, tst)
	//--------------------------------------------------------------------
	tst = test{}
	tst.name = "empty key (invalid call)"
	tst.args.fromMap = map[string]interface{}{
		"a": "aa",
	}
	tst.args.toMap = map[string]interface{}{
		"a": "initial",
	}
	tst.args.k = Key{}
	tst.want = false
	tst.wantToMap = map[string]interface{}{
		"a": "initial",
	}
	tests = append(tests, tst)
	//--------------------------------------------------------------------
	tst = test{}
	tst.name = "key is non-leaf"
	tst.args.fromMap = map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": map[string]interface{}{
					"d": "dd",
				},
			},
		},
	}
	tst.args.toMap = map[string]interface{}{}
	tst.args.k = Key{"a", "b"}
	tst.want = false
	tst.wantToMap = map[string]interface{}{}
	tests = append(tests, tst)
	//--------------------------------------------------------------------
	tst = test{}
	tst.name = "key too long"
	tst.args.fromMap = map[string]interface{}{
		"a": "aa",
	}
	tst.args.toMap = map[string]interface{}{}
	tst.args.k = Key{"a", "b"}
	tst.want = false
	tst.wantToMap = map[string]interface{}{}
	tests = append(tests, tst)
	//--------------------------------------------------------------------
	tst = test{}
	tst.name = "nil subtree"
	tst.args.fromMap = map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": map[string]interface{}{
					"d": "dd",
				},
			},
		},
	}
	tst.args.toMap = map[string]interface{}{
		"a": nil,
	}
	tst.args.k = Key{"a", "b", "c", "d"}
	tst.want = true
	tst.wantToMap = tst.args.fromMap
	tests = append(tests, tst)
	//--------------------------------------------------------------------
	tst = test{}
	tst.name = "nil subtree with rollback"
	tst.args.fromMap = map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": map[string]interface{}{
					"d": "dd",
				},
			},
		},
	}
	tst.args.toMap = map[string]interface{}{
		"a": nil,
	}
	tst.args.k = Key{"a", "b", "c"}
	tst.want = false
	tst.wantToMap = map[string]interface{}{
		"a": nil,
	}
	tests = append(tests, tst)
	//--------------------------------------------------------------------

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := setMapLeafFromMap(tt.args.fromMap, tt.args.toMap, tt.args.k); got != tt.want {
				t.Errorf("setMapLeafFromMap() = %v, want %v", got, tt.want)
			}

			if !reflect.DeepEqual(tt.args.toMap, tt.wantToMap) {
				t.Fatalf("toMap not equal to wantToMap; toMap=%#v; wantToMap=%#v", tt.args.toMap, tt.wantToMap)
			}
		})
	}
}
*/

func TestLoad_TOML(t *testing.T) {
	codec := Codec{
		Marshal: func(v interface{}) ([]byte, error) {
			sb := &strings.Builder{}
			enc := toml.NewEncoder(sb)
			err := enc.Encode(v)
			if err != nil {
				return nil, err
			}
			return []byte(sb.String()), nil
		},
		Unmarshal: toml.Unmarshal,
	}

	type simpleStruct struct {
		A1 string
		B1 int
	}
	type tagStruct struct {
		B2 int    `toml:"bee_two,omitempty" psiconfig:"optional,string"`
		C2 string `json:"cee_two" psiconfig:",bool"`
		D2 string `toml:"-"`
	}
	type advancedTypesStruct struct {
		A3 *string
		B3 []int
		C3 simpleStruct `toml:"cee_three"`
		D3 []simpleStruct
		E3 time.Time
	}
	type comboStruct struct {
		Simple simpleStruct
		Tag    tagStruct
		Adv    advancedTypesStruct
	}

	type args struct {
		readers      []io.Reader
		readerNames  []string
		envOverrides []EnvOverride
	}
	type test struct {
		name              string
		args              args
		env               map[string]string
		wantConfig        interface{}
		wantContributions Contributions
		wantIsDefineds    []Key
		wantNotIsDefineds []Key
		wantErrIsDefineds []Key
		wantErr           bool
	}
	tests := make([]test, 0)
	var tst test

	makeReaders := func(ss []string) []io.Reader {
		res := make([]io.Reader, len(ss))
		for i := range ss {
			res[i] = strings.NewReader(ss[i])
		}
		return res
	}

	//----------------------------------------------------------------------
	tst = test{}
	tst.name = "simple"
	tst.args.readers = makeReaders([]string{
		`
		B1 = 123
		a1 = "aa"
		`,
	})
	tst.args.readerNames = nil
	tst.args.envOverrides = nil
	tst.wantConfig = simpleStruct{
		A1: "aa",
		B1: 123,
	}
	tst.wantErr = false
	tst.wantContributions = Contributions{
		"a1": "0",
		"B1": "0",
	}
	tst.wantIsDefineds = []Key{
		{"a1"},
		{"A1"},
		{"b1"},
		{"B1"},
	}
	tst.wantNotIsDefineds = []Key{}
	tst.wantErrIsDefineds = []Key{
		{"x"},
		{"X"},
	}
	tests = append(tests, tst)
	/*
		//----------------------------------------------------------------------
		tst = test{}
		tst.name = "multi-reader"
		tst.args.readers = makeReaders([]string{
			`
			a = "aa"
			b = 22
			[sect1]
			a1 = "aa11"
			`,
			`
			a = "aaa"
			`,
			`
			[sect1]
			a1 = "aaa111"
			`,
		})
		tst.args.readerNames = []string{"first", "second", "third"}
		tst.args.envOverrides = nil
		tst.wantConfig = configStruct{
			A: "aaa",
			B: 22,
			Sect1: subStruct1{
				A1: "aaa111",
			},
		}
		tst.wantErr = false
		tst.wantContributions = Contributions{
			"a":        "second",
			"b":        "first",
			"sect1.a1": "third",
		}
		tst.wantIsDefineds = []Key{
			{"a"},
			{"A"},
			{"b"},
			{"B"},
			{"sect1", "a1"},
			{"Sect1", "A1"},
			{"Sect1", "a1"},
			{"sect1", "A1"},
		}
		tst.wantNotIsDefineds = nil
		tst.wantErrIsDefineds = nil
		tests = append(tests, tst)
		//----------------------------------------------------------------------
		tst = test{}
		tst.name = "env override"
		tst.args.readers = makeReaders([]string{
			`
			a = "aa"
			b = 22
			[sect1]
			a1 = "a1a1"
			`,
		})
		tst.args.readerNames = []string{"first"}
		tst.args.envOverrides = []EnvOverride{
			{
				EnvVar: "ENVB",
				Key:    Key{"b"},
				Conv: func(v string) interface{} {
					i, _ := strconv.Atoi(v)
					return i
				},
			},
			{
				EnvVar: "ENVA1",
				Key:    Key{"sect1", "a1"},
				Conv:   nil,
			},
		}
		tst.env = map[string]string{
			"ENVB":  "123",
			"ENVA1": "fromenv",
		}
		tst.wantConfig = configStruct{
			A: "aa",
			B: 123,
			Sect1: subStruct1{
				A1: "fromenv",
			},
		}
		tst.wantErr = false
		tst.wantContributions = Contributions{
			"a":        "first",
			"b":        "$ENVB",
			"sect1.a1": "$ENVA1",
		}
		tst.wantIsDefineds = []Key{
			{"a"},
			{"A"},
			{"b"},
			{"B"},
			{"sect1", "a1"},
			{"Sect1", "A1"},
		}
		tst.wantNotIsDefineds = nil
		tst.wantErrIsDefineds = nil
		tests = append(tests, tst)
		//----------------------------------------------------------------------
		tst = test{}
		tst.name = "error: vestigial key"
		tst.args.readers = makeReaders([]string{
			`
			a = "aa"
			b = 22
			c = "nope"
			`,
		})
		tst.args.readerNames = []string{"first"}
		tst.wantErr = true
		tests = append(tests, tst)
		//----------------------------------------------------------------------
		tst = test{}
		tst.name = "complex"
		tst.args.readers = makeReaders([]string{
			`
			a = "aa"
			date = "2011-01-01T01:01:01.01Z"
			[sect1]
			a1 = "aa11"
			B1 = 122
			[SectTwo]
			a2 = "aa22"
			bee_two = 234
			cee_two = "from first"
			[Sect3]
			A3 = "aa33"
			b3 = [3, 2, 1]
			[[Sect3.c3]]
			subc = "ccc333"
			[[Sect3.c3]]
			subc = "cccc3333"
			`,
			`
			[SectTwo]
			a2 = "a2 override"
			bee_two = 567
			[Sect3]
			[[Sect3.c3]]
			subc = "c3 1 override"
			[[Sect3.c3]]
			subc = "c3 2 override"
			[[Sect3.c3]]
			subc = "c3 3 override"
			`,
		})
		tst.args.readerNames = []string{"first", "second"}
		tst.args.envOverrides = []EnvOverride{
			{
				EnvVar: "S2_A2",
				Key:    Key{"SectTwo", "a2"},
				Conv:   nil,
			},
			{
				EnvVar: "CEE2",
				Key:    Key{"SectTwo", "cee_two"},
				Conv:   nil,
			},
			{
				EnvVar: "WONT_FIND",
				Key:    Key{"a"},
				Conv:   nil,
			},
			{
				EnvVar: "ENVB",
				Key:    Key{"b"},
				Conv: func(v string) interface{} {
					i, _ := strconv.Atoi(v)
					return i
				},
			},
		}
		tst.env = map[string]string{
			"ENVB":  "22",
			"S2_A2": "SectTwo.A2 from env",
			"CEE2":  "SectTwo.cee_two from env",
		}
		s3_a3 := "aa33"
		tst.wantConfig = configStruct{
			A:    "aa",
			B:    22,
			Date: time.Date(2011, time.January, 1, 1, 1, 1, 1000, time.UTC),
			Sect1: subStruct1{
				A1: "aa11",
				B1: 122,
			},
			Sect2: subStruct2{
				A2: "SectTwo.A2 from env",
				B2: 567,
				C2: "SectTwo.cee_two from env",
			},
			Sect3: subStruct3{
				A3: &s3_a3,
				B3: []int{3, 2, 1},
				C3: []subStruct3_1{
					{SubC: "c3 1 override"},
					{SubC: "c3 2 override"},
					{SubC: "c3 3 override"},
				},
			},
		}
		tst.wantErr = false
		tst.wantContributions = Contributions{
			"a":               "first",
			"b":               "$ENVB",
			"date":            "first",
			"sect1.a1":        "first",
			"sect1.B1":        "first",
			"SectTwo.a2":      "$S2_A2",
			"SectTwo.bee_two": "second",
			"SectTwo.cee_two": "$CEE2",
			"Sect3.A3":        "first",
			"Sect3.b3":        "first",
			"Sect3.c3":        "second",
		}
		tst.wantIsDefineds = []Key{
			{"a"},
			{"b"},
			{"sect1", "a1"},
			{"sect1", "B1"},
			{"SectTwo", "a2"},
			{"SectTwo", "B2"},
			{"SectTwo", "cee_two"},
			{"sect3", "a3"},
			{"Sect3", "b3"},
			{"Sect3", "C3"},
		}
		tst.wantNotIsDefineds = nil
		tst.wantErrIsDefineds = nil
		tests = append(tests, tst)
	*/

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			for envKey, envVal := range tt.env {
				os.Setenv(envKey, envVal)
			}

			// Create an instance of the result based on the type of wantConfig
			resultPtr := reflect.New(reflect.TypeOf(tt.wantConfig)).Interface()

			gotMD, err := Load(codec, tt.args.readers, tt.args.readerNames, tt.args.envOverrides, resultPtr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v; wantErr: %v", err, tt.wantErr)
			}

			if err != nil {
				return
			}

			if !reflect.DeepEqual(reflect.ValueOf(resultPtr).Elem().Interface(), tt.wantConfig) {
				t.Fatalf("result did not match;\ngot  %#v\nwant %#v", resultPtr, tt.wantConfig)
			}

			if !reflect.DeepEqual(gotMD.Contributions, tt.wantContributions) {
				t.Fatalf("Contributions mismatch;\ngot  %#v\nwant %#v", gotMD.Contributions, tt.wantContributions)
			}

			for _, k := range tt.wantIsDefineds {
				def, err := gotMD.IsDefined(k...)
				if err != nil {
					t.Fatalf("IsDefined should not get error; key: %#v; metadata: %#v", k, gotMD)
				}
				if !def {
					t.Fatalf("key should be defined: %#v; metadata: %#v", k, gotMD)
				}
			}

			for _, k := range tt.wantNotIsDefineds {
				def, err := gotMD.IsDefined(k...)
				if err != nil {
					t.Fatalf("IsDefined should not get error; key: %#v; metadata: %#v", k, gotMD)
				}
				if def {
					t.Fatalf("key should be undefined: %#v; metadata: %#v", k, gotMD)
				}
			}

			for _, k := range tt.wantErrIsDefineds {
				_, err := gotMD.IsDefined(k...)
				if err == nil {
					t.Fatalf("IsDefined should get error; key: %#v; metadata: %#v", k, gotMD)
				}
			}
		})
	}
}

func Test_aliasedKeysMatch(t *testing.T) {
	type args struct {
		ak1 aliasedKey
		ak2 aliasedKey
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "simple",
			args: args{
				ak1: aliasedKey{
					{"a"},
				},
				ak2: aliasedKey{
					{"a"},
				},
			},
			want: true,
		},
		{
			name: "case insensitive",
			args: args{
				ak1: aliasedKey{
					{"a"},
				},
				ak2: aliasedKey{
					{"A"},
				},
			},
			want: true,
		},
		{
			name: "aliases",
			args: args{
				ak1: aliasedKey{
					{"a", "apple"},
				},
				ak2: aliasedKey{
					{"apple"},
				},
			},
			want: true,
		},
		{
			name: "complex",
			args: args{
				ak1: aliasedKey{
					{"a", "apple"},
					{"b", "bee"},
					{"C", "carrot"},
				},
				ak2: aliasedKey{
					{"apple"},
					{"B"},
					{"carrot", "chocolate"},
				},
			},
			want: true,
		},
		{
			name: "no match",
			args: args{
				ak1: aliasedKey{
					{"a", "apple"},
					{"b", "bee"},
					{"C", "carrot"},
				},
				ak2: aliasedKey{
					{"apple"},
					{"B"},
					{"nomatch"},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := aliasedKeysMatch(tt.args.ak1, tt.args.ak2); got != tt.want {
				t.Errorf("aliasedKeysMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_findStructField(t *testing.T) {
	type args struct {
		fields    []structField
		targetKey aliasedKey
	}
	tests := []struct {
		name      string
		args      args
		wantMatch bool
	}{
		{
			name: "simple",
			args: args{
				targetKey: aliasedKey{
					{"a"},
				},
				fields: []structField{
					{
						aliasedKey: aliasedKey{
							{"a"},
						},
					},
				},
			},
			wantMatch: true,
		},
		{
			name: "aliases",
			args: args{
				targetKey: aliasedKey{
					{"b"},
					{"bee_two"},
				},
				fields: []structField{
					{
						aliasedKey: aliasedKey{
							{"a"},
						},
					},
					{
						aliasedKey: aliasedKey{
							{"b", "bee"},
						},
					},
					{
						aliasedKey: aliasedKey{
							{"b", "bee"},
							{"b2", "bee_two"},
						},
					},
					{
						aliasedKey: aliasedKey{
							{"c", "cee"},
							{"c2", "cee_two"},
						},
					},
				},
			},
			wantMatch: true,
		},
		{
			name: "no match",
			args: args{
				targetKey: aliasedKey{
					{"b"},
					{"nomatch"},
				},
				fields: []structField{
					{
						aliasedKey: aliasedKey{
							{"a"},
						},
					},
					{
						aliasedKey: aliasedKey{
							{"b", "bee"},
						},
					},
					{
						aliasedKey: aliasedKey{
							{"b", "bee"},
							{"b2", "bee_two"},
						},
					},
					{
						aliasedKey: aliasedKey{
							{"c", "cee"},
							{"c2", "cee_two"},
						},
					},
				},
			},
			wantMatch: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStructField, ok := findStructField(tt.args.fields, tt.args.targetKey)
			if ok != tt.wantMatch {
				t.Fatalf("findStructField() ok = %v, want %v", ok, tt.wantMatch)
			}

			if !ok {
				return
			}

			if !aliasedKeysMatch(gotStructField.aliasedKey, tt.args.targetKey) {
				t.Fatalf("gotStructField doesn't actually match target: got %+v; want %+v", gotStructField.aliasedKey, tt.args.targetKey)
			}
		})
	}
}
