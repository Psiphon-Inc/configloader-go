# psiphon-go-config

TODO: psiconfig: Put structFields into decoder so it doesn't have to be passed around everywhere (and make more funcs methods of decoder).
TODO: introduce NewDecoder(codec, readers) -- everything else fields on that. Then decoder.Load(&result.)

TODO: Does IsDefined return error if the whole sub-struct is absent? (Or only for pointer-to-struct?)

DOC: Defaults can be done by pre-populating struct (but not best way)

// NOTE: BurntSushi has UnmarshalText override-ability, so we get that for free. It should be noted in the doc.

ignore with `json:"-"` or `toml:"-"`

`psiconfig:"optional,type"`

```
func(v interface{}) ([]byte, error) {
  sb := &strings.Builder{}
  enc := toml.NewEncoder(sb)
  err := enc.Encode(v)
  if err != nil {
    return nil, err
  }
  return []byte(sb.String()), nil
}
```

Different field aliases on toml and json tags is undefined behaviour.

## Future work

* Type checking inside slices
