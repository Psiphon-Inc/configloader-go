/*
 * BSD 3-Clause License
 * Copyright (c) 2019, Psiphon Inc.
 * All rights reserved.
 */

/*
Package configloader makes loading config information easier, more flexible, and more powerful. It enables loading from multiple files, defaults, and environment overrides. TOML and JSON are supported out-of-the-box, but other formats can be easily used.

It is recommended that the examples be perused to assist usage: https://github.com/Psiphon-Inc/configloader-go/tree/master/examples

Result Structs and Maps

The value to be populated can be a struct or a map[string]interface{}. A struct is preferable, as it provides information about what fields should be expected, which are optional, and so on.

Struct Field Tags

Field name aliases can be specified using type-specific tags, like `toml:"alias"` or `json:"alias"`. Fields can be ignored using type-specific tags as well, like `toml:"-"` or `json:"-"`.

configloader also provides its own tag type, of the form `conf:"optional,specific_type"`, described below. The name "conf" is configurable with the TagName variable.

Optional and Required Fields

All fields in a result struct are by default required. A field can be marked as optional with the struct tag `conf:"optional"`. A field is also considered optional if it is present in the defaults argument.

Defaults

The best way to provide default values is via the defaults argument to Load. This defaults are implicitly considered optional fields.

Default values are only applied the field receives no value from either the config files (readers) or an environment variable.

Another way to provide defaults is to pre-populate the struct or map result. Yet another is way is by after loading, then checking metadata.IsDefined() fohttps://asl19.org/en/blog/2014-03-17-ama-psiphon.htmlr the fields in question and assigning default values to them (or returning default values from accessors).

Specify Field Type

The type to used for comparison can be specified with a struct tag, like `conf:",float32"` (before the comma is "optional", or not). It will be compared against the Type and Kind of the field. (There may not be any good use for this. If we come across one, add it here. Otherwise re-think the existence of this feature. See issue: https://github.com/Psiphon-Inc/configloader-go/issues/1)

Support for TextUnmarshaler

configloader detects fields that implement encoding/TextUnmarshaler and expects to find string values for those fields. This means that support for TextUnmarshaler is expected from the underlying unmarshaler.
*/
package configloader
