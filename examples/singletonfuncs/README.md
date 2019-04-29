# `configloader` Example: Config access via singleton and package functions

This example is a variant of the [recommended usage example](https://github.com/Psiphon-Inc/configloader-go/tree/master/examples/recommended) -- instead of returning a config object which has methods that implement interfaces, it puts that object into a package singleton and provides functions to access it. This makes testing harder and is _not_ recommended.
