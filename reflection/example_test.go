package reflection_test

import (
	"fmt"
	"time"

	"github.com/Psiphon-Inc/psiphon-go-config/reflection"
	"github.com/Psiphon-Inc/psiphon-go-config/toml"
)

func ExampleGetStructFields_withStruct() {
	const tagName = "conf"

	type S struct {
		F     string `toml:"eff" conf:"optional"`
		Inner struct {
			InnerF int
		}
		Time time.Time // implements encoding.UnmarshalText

		unexported string
	}

	var s S
	fmt.Printf("%#v\n", reflection.GetStructFields(s, tagName, toml.Codec))

	// Output:
	/*
		[] reflection.StructField {
		  reflection.StructField {
		    AliasedKey: reflection.AliasedKey {
		      reflection.AliasedKeyElem {
		        "F",
		        "eff"
		      }
		    },
		    Type: "string",
		    Kind: "string",
		    Optional: true,
		    ExpectedType: ""
		  }, reflection.StructField {
		    AliasedKey: reflection.AliasedKey {
		      reflection.AliasedKeyElem {
		        "Inner"
		      }
		    },
		    Type: "struct { InnerF int }",
		    Kind: "struct",
		    Optional: false,
		    ExpectedType: ""
		  }, reflection.StructField {
		    AliasedKey: reflection.AliasedKey {
		      reflection.AliasedKeyElem {
		        "Inner"
		      }, reflection.AliasedKeyElem {
		        "InnerF"
		      }
		    },
		    Type: "int",
		    Kind: "int",
		    Optional: false,
		    ExpectedType: ""
		  }, reflection.StructField {
		    AliasedKey: reflection.AliasedKey {
		      reflection.AliasedKeyElem {
		        "Time"
		      }
		    },
		    Type: "time.Time",
		    Kind: "struct",
		    Optional: false,
		    ExpectedType: "string"
		  }
		}
	*/
}

func ExampleGetStructFields_withMap() {
	const tagName = "conf"

	m := map[string]interface{}{
		"a": "aaa",
		"b": 123,
		"c": time.Now(),
		"d": map[string]interface{}{
			"d1": "d1d1",
		},
		"e": []bool{true, false},
	}

	fmt.Printf("%#v\n", reflection.GetStructFields(m, tagName, toml.Codec))

	// Output:
	/*
			[] reflection.StructField {
		  reflection.StructField {
		    AliasedKey: reflection.AliasedKey {
		      reflection.AliasedKeyElem {
		        "e"
		      }
		    },
		    Type: "[]bool",
		    Kind: "slice",
		    Optional: false,
		    ExpectedType: ""
		  }, reflection.StructField {
		    AliasedKey: reflection.AliasedKey {
		      reflection.AliasedKeyElem {
		        "a"
		      }
		    },
		    Type: "string",
		    Kind: "string",
		    Optional: false,
		    ExpectedType: ""
		  }, reflection.StructField {
		    AliasedKey: reflection.AliasedKey {
		      reflection.AliasedKeyElem {
		        "b"
		      }
		    },
		    Type: "int",
		    Kind: "int",
		    Optional: false,
		    ExpectedType: ""
		  }, reflection.StructField {
		    AliasedKey: reflection.AliasedKey {
		      reflection.AliasedKeyElem {
		        "c"
		      }
		    },
		    Type: "time.Time",
		    Kind: "struct",
		    Optional: false,
		    ExpectedType: "string"
		  }, reflection.StructField {
		    AliasedKey: reflection.AliasedKey {
		      reflection.AliasedKeyElem {
		        "d"
		      }
		    },
		    Type: "map[string]interface {}",
		    Kind: "map",
		    Optional: false,
		    ExpectedType: ""
		  }, reflection.StructField {
		    AliasedKey: reflection.AliasedKey {
		      reflection.AliasedKeyElem {
		        "d"
		      }, reflection.AliasedKeyElem {
		        "d1"
		      }
		    },
		    Type: "string",
		    Kind: "string",
		    Optional: false,
		    ExpectedType: ""
		  }
		}
	*/
}
