# psiphon-go-config

TODO: A type mismatch between struct and TOML results in a zero value in the struct. TOML.MD will say the field _is_ defined. Can we detect this mismatch?

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